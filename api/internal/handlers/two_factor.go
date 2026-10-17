package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"api/internal/models"
	"api/internal/services"
	"api/internal/totp"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

// TwoFactorPasswordRequest confirms a sensitive two-factor change with the
// user's password, so an open session left on a desk is not enough.
type TwoFactorPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// TwoFactorCodeRequest carries a code from the authenticator app.
type TwoFactorCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// TwoFactorSetupResponse is what the user needs to add Vigie to their app.
type TwoFactorSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
	QRCode     string `json:"qr_code"` // PNG data URL of OTPAuthURL
}

// RecoveryCodesResponse returns new recovery codes. They are shown once.
type RecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// checkSecondFactor accepts a code from the authenticator app or an unused
// recovery code. Both are consumed with a conditional update, so two
// requests sending the same code cannot both succeed. It reports whether a
// recovery code was used.
func checkSecondFactor(db *gorm.DB, user *models.User, code string) (ok, recovery bool, err error) {
	if step, valid := totp.Validate(*user.TwoFactorSecret, code, time.Now(), user.TwoFactorLastStep); valid {
		res := db.Model(&models.User{}).
			Where("id = ? AND two_factor_last_step < ?", user.ID, step).
			Update("two_factor_last_step", step)
		if res.Error != nil {
			return false, false, res.Error
		}
		if res.RowsAffected == 1 {
			user.TwoFactorLastStep = step
		}
		return res.RowsAffected == 1, false, nil
	}
	res := db.Model(&models.RecoveryCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", user.ID, models.HashRecoveryCode(code)).
		Update("used_at", time.Now())
	if res.Error != nil {
		return false, false, res.Error
	}
	return res.RowsAffected == 1, res.RowsAffected == 1, nil
}

// recoveryCodesLeft counts the user's unused recovery codes.
func recoveryCodesLeft(db *gorm.DB, userID uint) (int, error) {
	var n int64
	err := db.Model(&models.RecoveryCode{}).Where("user_id = ? AND used_at IS NULL", userID).Count(&n).Error
	return int(n), err
}

// replaceRecoveryCodes deletes the user's recovery codes and makes new ones.
func replaceRecoveryCodes(tx *gorm.DB, userID uint) ([]string, error) {
	if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
		return nil, err
	}
	codes, rows, err := models.NewRecoveryCodes(userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Create(&rows).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// clearTwoFactor turns two-factor login off for a user and deletes their
// recovery codes.
func clearTwoFactor(db *gorm.DB, userID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
			"two_factor_secret":         nil,
			"two_factor_pending_secret": nil,
			"two_factor_last_step":      0,
		}).Error
		if err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error
	})
}

// twoFactorIssuer is the name authenticator apps show next to the code: the
// status page's English title, or "Vigie".
func twoFactorIssuer(db *gorm.DB) string {
	var setting models.Settings
	if err := db.Where("`key` = ?", "branding").First(&setting).Error; err == nil {
		var config BrandingConfig
		if json.Unmarshal([]byte(setting.Value), &config) == nil && config.TitleEN != "" {
			return config.TitleEN
		}
	}
	return "Vigie"
}

// confirmPassword checks the password of a two-factor change, and logs a
// failure in the audit log.
func confirmPassword(c *gin.Context, auditService *services.AuditService, user *models.User, action string) bool {
	var req TwoFactorPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password required"})
		return false
	}
	if !user.CheckPassword(req.Password) {
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": action + "_failed",
			"reason": "invalid_password",
		}, false)
		// 403, not 401: the session is valid, and the admin logs out on 401.
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid password"})
		return false
	}
	return true
}

// SetupTwoFactor starts turning two-factor login on: it makes a secret and
// returns it with a QR code. Login is unchanged until EnableTwoFactor
// confirms a first code.
func SetupTwoFactor(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := services.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		if !confirmPassword(c, auditService, user, "two_factor_setup") {
			return
		}
		if user.TwoFactorEnabled() {
			c.JSON(http.StatusConflict, gin.H{"error": "Two-factor login is already on"})
			return
		}

		secret, err := totp.NewSecret()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create a secret"})
			return
		}
		if err := db.Model(&models.User{}).Where("id = ?", user.ID).Update("two_factor_pending_secret", secret).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save the secret"})
			return
		}

		url := totp.URL(twoFactorIssuer(db), user.Username, secret)
		png, err := qrcode.Encode(url, qrcode.Medium, 256)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make the QR code"})
			return
		}
		c.JSON(http.StatusOK, TwoFactorSetupResponse{
			Secret:     secret,
			OTPAuthURL: url,
			QRCode:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		})
	}
}

// EnableTwoFactor turns two-factor login on once the user has typed a code
// made from the secret of SetupTwoFactor, and returns recovery codes.
func EnableTwoFactor(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := services.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		var req TwoFactorCodeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Code required"})
			return
		}
		if user.TwoFactorEnabled() {
			c.JSON(http.StatusConflict, gin.H{"error": "Two-factor login is already on"})
			return
		}
		if user.TwoFactorPendingSecret == nil || *user.TwoFactorPendingSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Start the two-factor setup first"})
			return
		}
		step, valid := totp.Validate(*user.TwoFactorPendingSecret, req.Code, time.Now(), 0)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid code. Check the time on your phone, then try the next code."})
			return
		}

		var codes []string
		err := db.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&models.User{}).
				Where("id = ? AND two_factor_pending_secret = ?", user.ID, *user.TwoFactorPendingSecret).
				Updates(map[string]any{
					"two_factor_secret":         *user.TwoFactorPendingSecret,
					"two_factor_pending_secret": nil,
					"two_factor_last_step":      step,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errSetupChanged
			}
			var err error
			codes, err = replaceRecoveryCodes(tx, user.ID)
			return err
		})
		if errors.Is(err, errSetupChanged) {
			c.JSON(http.StatusConflict, gin.H{"error": "The setup was started again elsewhere. Start it again."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to turn two-factor login on"})
			return
		}

		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": "two_factor_enabled",
		}, true)
		c.JSON(http.StatusOK, RecoveryCodesResponse{RecoveryCodes: codes})
	}
}

var errSetupChanged = errors.New("two-factor setup changed")

// DisableTwoFactor turns two-factor login off for the current user.
func DisableTwoFactor(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := services.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		if !confirmPassword(c, auditService, user, "two_factor_disable") {
			return
		}
		if err := clearTwoFactor(db, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to turn two-factor login off"})
			return
		}
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": "two_factor_disabled",
		}, true)
		c.JSON(http.StatusOK, gin.H{"message": "Two-factor login is off"})
	}
}

// RegenerateRecoveryCodes replaces the current user's recovery codes.
func RegenerateRecoveryCodes(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := services.GetCurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		if !confirmPassword(c, auditService, user, "recovery_codes") {
			return
		}
		if !user.TwoFactorEnabled() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Two-factor login is off"})
			return
		}
		var codes []string
		err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			codes, err = replaceRecoveryCodes(tx, user.ID)
			return err
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make recovery codes"})
			return
		}
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": "recovery_codes_regenerated",
		}, true)
		c.JSON(http.StatusOK, RecoveryCodesResponse{RecoveryCodes: codes})
	}
}

// ResetUserTwoFactor turns two-factor login off for another user, who lost
// their phone and their recovery codes (admin only).
func ResetUserTwoFactor(db *gorm.DB, auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
			return
		}
		if err := clearTwoFactor(db, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to turn two-factor login off"})
			return
		}
		auditService.Log(c, models.ActionUpdate, models.ResourceUser, &user.ID, map[string]interface{}{
			"action": "two_factor_reset",
		}, true)
		c.JSON(http.StatusOK, gin.H{"message": "Two-factor login is off for " + user.Username})
	}
}
