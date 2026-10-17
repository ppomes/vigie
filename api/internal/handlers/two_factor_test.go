package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"api/internal/middleware"
	"api/internal/models"
	"api/internal/services"
	"api/internal/testutil"
	"api/internal/totp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const twoFactorPassword = "Correct-horse-1!"

// newTwoFactorRouter wires login and the two-factor routes as in
// cmd/api/main.go, without the login rate limiter.
func newTwoFactorRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	t.Setenv("JWT_SECRET_KEY", testJWTSecret)
	gin.SetMode(gin.TestMode)
	audit := services.NewAuditService(db)
	// A fresh limiter per test: user IDs repeat across test databases.
	middleware.TwoFactorLimiter = middleware.NewRateLimiter(5, 15*time.Minute)

	r := gin.New()
	r.POST("/login", Login(db, audit))
	admin := r.Group("/admin", middleware.AuthRequiredWithDB(db))
	admin.GET("/me", GetCurrentUserProfile(db))
	tf := admin.Group("/me/two-factor", middleware.RequireSession())
	tf.POST("/setup", SetupTwoFactor(db, audit))
	tf.POST("/enable", EnableTwoFactor(db, audit))
	tf.POST("/disable", DisableTwoFactor(db, audit))
	tf.POST("/recovery-codes", RegenerateRecoveryCodes(db, audit))
	admin.DELETE("/users/:id/two-factor", middleware.RequireAdmin(), middleware.RequireSession(), ResetUserTwoFactor(db, audit))
	return r
}

func createUserWithPassword(t *testing.T, db *gorm.DB, name, role string) models.User {
	t.Helper()
	u := models.User{Username: name, Email: name + "@example.com", Role: role, Active: true}
	if err := u.HashPassword(twoFactorPassword); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func asUser(t *testing.T, method, path string, userID uint, body any) authRequest {
	return authRequest{method: method, path: path, body: body, header: "Authorization", value: "Bearer " + sessionToken(t, userID)}
}

func login(t *testing.T, r *gin.Engine, username, code string) (int, map[string]any) {
	t.Helper()
	body := map[string]string{"username": username, "password": twoFactorPassword}
	if code != "" {
		body["code"] = code
	}
	w := do(t, r, authRequest{method: http.MethodPost, path: "/login", body: body})
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// enableTwoFactor runs the setup for a user and returns their secret and
// recovery codes.
func enableTwoFactor(t *testing.T, r *gin.Engine, userID uint) (string, []string) {
	t.Helper()
	w := do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/setup", userID, map[string]string{"password": twoFactorPassword}))
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	var setup TwoFactorSetupResponse
	json.Unmarshal(w.Body.Bytes(), &setup)
	if !strings.HasPrefix(setup.QRCode, "data:image/png;base64,") || !strings.Contains(setup.OTPAuthURL, setup.Secret) {
		t.Fatalf("setup response: %+v", setup)
	}
	code, _ := totp.Code(setup.Secret, totp.Step(time.Now()))
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/enable", userID, map[string]string{"code": code}))
	if w.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", w.Code, w.Body)
	}
	var rc RecoveryCodesResponse
	json.Unmarshal(w.Body.Bytes(), &rc)
	if len(rc.RecoveryCodes) != models.RecoveryCodeCount {
		t.Fatalf("recovery codes: %v", rc.RecoveryCodes)
	}
	return setup.Secret, rc.RecoveryCodes
}

func TestTwoFactorSetup(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)

	w := do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/setup", u.ID, map[string]string{"password": "wrong"}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("setup with a wrong password: %d", w.Code)
	}
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/enable", u.ID, map[string]string{"code": "123456"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("enable before setup: %d", w.Code)
	}

	// Setup alone does not change login.
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/setup", u.ID, map[string]string{"password": twoFactorPassword}))
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	if code, _ := login(t, r, "marie", ""); code != http.StatusOK {
		t.Fatalf("login during setup: %d", code)
	}
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/enable", u.ID, map[string]string{"code": "000000"}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("enable with a wrong code: %d", w.Code)
	}

	enableTwoFactor(t, r, u.ID)
	w = do(t, r, asUser(t, http.MethodGet, "/admin/me", u.ID, nil))
	var me struct{ User UserResponse }
	json.Unmarshal(w.Body.Bytes(), &me)
	if !me.User.TwoFactorEnabled || me.User.RecoveryCodesLeft == nil || *me.User.RecoveryCodesLeft != models.RecoveryCodeCount {
		t.Fatalf("profile: %s", w.Body)
	}
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/setup", u.ID, map[string]string{"password": twoFactorPassword}))
	if w.Code != http.StatusConflict {
		t.Fatalf("setup when already on: %d", w.Code)
	}
}

func TestTwoFactorLogin(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)
	secret, recovery := enableTwoFactor(t, r, u.ID)

	code, out := login(t, r, "marie", "")
	if code != http.StatusUnauthorized || out["two_factor_required"] != true || out["token"] != nil {
		t.Fatalf("login without a code: %d %v", code, out)
	}
	if code, out := login(t, r, "marie", "000000"); code != http.StatusUnauthorized || out["two_factor_required"] != true {
		t.Fatalf("login with a wrong code: %d %v", code, out)
	}

	// Turning it on used the current code. The next one is accepted too,
	// for clocks a little ahead, so the test need not wait 30 seconds.
	nextStep := totp.Step(time.Now()) + 1
	next, _ := totp.Code(secret, nextStep)
	if code, out := login(t, r, "marie", next); code != http.StatusOK || out["token"] == nil {
		t.Fatalf("login with the next code: %d %v", code, out)
	}
	if code, _ := login(t, r, "marie", next); code != http.StatusUnauthorized {
		t.Fatalf("same code accepted twice: %d", code)
	}

	code, out = login(t, r, "marie", strings.ToUpper(recovery[0]))
	if code != http.StatusOK || out["recovery_codes_left"] != float64(models.RecoveryCodeCount-1) {
		t.Fatalf("login with a recovery code: %d %v", code, out)
	}
	if code, _ := login(t, r, "marie", recovery[0]); code != http.StatusUnauthorized {
		t.Fatalf("recovery code accepted twice: %d", code)
	}

	// Saving last_login must not write back an older step.
	var stored models.User
	db.First(&stored, u.ID)
	if stored.TwoFactorLastStep != nextStep || stored.LastLogin == nil {
		t.Fatalf("stored user: step %d, last login %v", stored.TwoFactorLastStep, stored.LastLogin)
	}
}

func TestTwoFactorWrongCodesLockTheAccount(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)
	secret, _ := enableTwoFactor(t, r, u.ID)

	for i := 0; i < 5; i++ {
		if code, _ := login(t, r, "marie", "000000"); code != http.StatusUnauthorized {
			t.Fatalf("wrong code %d: %d", i, code)
		}
	}
	next, _ := totp.Code(secret, totp.Step(time.Now())+1)
	if code, _ := login(t, r, "marie", next); code != http.StatusTooManyRequests {
		t.Fatalf("right code after 5 wrong ones: %d", code)
	}
}

func TestTwoFactorRecoveryCodesAndDisable(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)
	_, old := enableTwoFactor(t, r, u.ID)

	w := do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/recovery-codes", u.ID, map[string]string{"password": twoFactorPassword}))
	if w.Code != http.StatusOK {
		t.Fatalf("new recovery codes: %d %s", w.Code, w.Body)
	}
	var rc RecoveryCodesResponse
	json.Unmarshal(w.Body.Bytes(), &rc)
	if code, _ := login(t, r, "marie", old[0]); code != http.StatusUnauthorized {
		t.Fatalf("old recovery code still accepted: %d", code)
	}
	if code, _ := login(t, r, "marie", rc.RecoveryCodes[0]); code != http.StatusOK {
		t.Fatalf("new recovery code refused: %d", code)
	}

	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/disable", u.ID, map[string]string{"password": "wrong"}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("disable with a wrong password: %d", w.Code)
	}
	w = do(t, r, asUser(t, http.MethodPost, "/admin/me/two-factor/disable", u.ID, map[string]string{"password": twoFactorPassword}))
	if w.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	if code, _ := login(t, r, "marie", ""); code != http.StatusOK {
		t.Fatalf("login after disable: %d", code)
	}
	var n int64
	db.Model(&models.RecoveryCode{}).Where("user_id = ?", u.ID).Count(&n)
	if n != 0 {
		t.Errorf("%d recovery codes left after disable", n)
	}
}

func TestAdminResetsTwoFactor(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	admin := createUserWithPassword(t, db, "pierre", models.RoleAdmin)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)
	enableTwoFactor(t, r, u.ID)

	w := do(t, r, asUser(t, http.MethodDelete, "/admin/users/1/two-factor", u.ID, nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer reset: %d", w.Code)
	}
	w = do(t, r, asUser(t, http.MethodDelete, fmt.Sprintf("/admin/users/%d/two-factor", u.ID), admin.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("admin reset: %d %s", w.Code, w.Body)
	}
	if code, _ := login(t, r, "marie", ""); code != http.StatusOK {
		t.Fatalf("login after reset: %d", code)
	}
}

func TestTwoFactorNeedsSession(t *testing.T) {
	db := testutil.NewDB(t)
	r := newTwoFactorRouter(t, db)
	u := createUserWithPassword(t, db, "marie", models.RoleViewer)
	raw := "vk_" + strings.Repeat("a", 40)
	key := models.NewAPIKey("monitoring", raw, u.ID)
	if err := db.Create(&key).Error; err != nil {
		t.Fatal(err)
	}
	w := do(t, r, authRequest{method: http.MethodPost, path: "/admin/me/two-factor/setup",
		body: map[string]string{"password": twoFactorPassword}, header: middleware.CachetTokenHeader, value: raw})
	if w.Code != http.StatusForbidden {
		t.Fatalf("setup with an API key: %d", w.Code)
	}
}
