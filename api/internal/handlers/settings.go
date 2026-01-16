package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"api/internal/models"
	"api/internal/services"
)

// GetTheme returns the current theme setting (public endpoint)
func GetTheme(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var setting models.Settings
		if err := db.Where("`key` = ?", "theme").First(&setting).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"theme": "default"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"theme": setting.Value})
	}
}

// ListSettings returns all settings (admin endpoint)
func ListSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings []models.Settings
		if err := db.Find(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"settings": settings})
	}
}

// UpdateSetting updates a setting by key (admin endpoint)
func UpdateSetting(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		var input struct {
			Value string `json:"value" binding:"required"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate theme values
		if key == "theme" {
			validThemes := map[string]bool{
				"default":   true,
				"dark":      true,
				"corporate": true,
				"modern":    true,
			}
			if !validThemes[input.Value] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid theme value"})
				return
			}
		}

		var setting models.Settings
		result := db.Where("`key` = ?", key).First(&setting)

		if result.Error == gorm.ErrRecordNotFound {
			// Create new setting
			setting = models.Settings{
				Key:   key,
				Value: input.Value,
			}
			if err := db.Create(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create setting"})
				return
			}
		} else if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch setting"})
			return
		} else {
			// Update existing setting
			setting.Value = input.Value
			if err := db.Save(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
				return
			}
		}

		c.JSON(http.StatusOK, setting)
	}
}

// GetEmailConfig returns the email configuration (admin endpoint)
func GetEmailConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var setting models.Settings
		if err := db.Where("`key` = ?", "email_config").First(&setting).Error; err != nil {
			// Return default disabled config
			c.JSON(http.StatusOK, gin.H{
				"config": services.EmailConfig{
					Provider: "disabled",
				},
			})
			return
		}

		var config services.EmailConfig
		if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse email config"})
			return
		}

		// Mask sensitive fields
		if config.SMTPPassword != "" {
			config.SMTPPassword = "********"
		}
		if config.SendGridAPIKey != "" {
			config.SendGridAPIKey = "********"
		}
		if config.MailgunAPIKey != "" {
			config.MailgunAPIKey = "********"
		}
		if config.AWSSecretKey != "" {
			config.AWSSecretKey = "********"
		}

		c.JSON(http.StatusOK, gin.H{"config": config})
	}
}

// UpdateEmailConfig updates the email configuration (admin endpoint)
func UpdateEmailConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.EmailConfig
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate provider
		validProviders := map[string]bool{
			"disabled": true,
			"smtp":     true,
			"sendgrid": true,
			"mailgun":  true,
			"ses":      true,
		}
		if !validProviders[input.Provider] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email provider"})
			return
		}

		// If password fields are masked, preserve existing values
		var existingSetting models.Settings
		if err := db.Where("`key` = ?", "email_config").First(&existingSetting).Error; err == nil {
			var existingConfig services.EmailConfig
			if err := json.Unmarshal([]byte(existingSetting.Value), &existingConfig); err == nil {
				if input.SMTPPassword == "********" {
					input.SMTPPassword = existingConfig.SMTPPassword
				}
				if input.SendGridAPIKey == "********" {
					input.SendGridAPIKey = existingConfig.SendGridAPIKey
				}
				if input.MailgunAPIKey == "********" {
					input.MailgunAPIKey = existingConfig.MailgunAPIKey
				}
				if input.AWSSecretKey == "********" {
					input.AWSSecretKey = existingConfig.AWSSecretKey
				}
			}
		}

		// Serialize config to JSON
		configJSON, err := json.Marshal(input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize config"})
			return
		}

		var setting models.Settings
		result := db.Where("`key` = ?", "email_config").First(&setting)

		if result.Error == gorm.ErrRecordNotFound {
			setting = models.Settings{
				Key:   "email_config",
				Value: string(configJSON),
			}
			if err := db.Create(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save email config"})
				return
			}
		} else if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch setting"})
			return
		} else {
			setting.Value = string(configJSON)
			if err := db.Save(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email config"})
				return
			}
		}

		// Mask response
		if input.SMTPPassword != "" {
			input.SMTPPassword = "********"
		}
		if input.SendGridAPIKey != "" {
			input.SendGridAPIKey = "********"
		}
		if input.MailgunAPIKey != "" {
			input.MailgunAPIKey = "********"
		}
		if input.AWSSecretKey != "" {
			input.AWSSecretKey = "********"
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Email configuration saved",
			"config":  input,
		})
	}
}

// BrandingConfig represents the branding settings
type BrandingConfig struct {
	TitleEN string `json:"title_en"`
	TitleFR string `json:"title_fr"`
	Logo    string `json:"logo"` // Base64 data URL
}

// GetBranding returns the branding settings (public endpoint)
func GetBranding(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var setting models.Settings
		if err := db.Where("`key` = ?", "branding").First(&setting).Error; err != nil {
			// Return default empty branding
			c.JSON(http.StatusOK, BrandingConfig{
				TitleEN: "",
				TitleFR: "",
				Logo:    "",
			})
			return
		}

		var config BrandingConfig
		if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse branding config"})
			return
		}

		c.JSON(http.StatusOK, config)
	}
}

// UpdateBranding updates the branding settings (admin endpoint)
func UpdateBranding(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input BrandingConfig
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate logo if provided
		if input.Logo != "" {
			// Check size (max 500KB base64)
			if len(input.Logo) > 500*1024 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Logo too large (max 500KB)"})
				return
			}

			// Validate content type - must be a valid image data URL
			validPrefixes := []string{
				"data:image/png;base64,",
				"data:image/jpeg;base64,",
				"data:image/jpg;base64,",
				"data:image/gif;base64,",
				"data:image/svg+xml;base64,",
				"data:image/webp;base64,",
			}
			isValid := false
			for _, prefix := range validPrefixes {
				if len(input.Logo) > len(prefix) && input.Logo[:len(prefix)] == prefix {
					isValid = true
					break
				}
			}
			if !isValid {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Logo must be a valid image (PNG, JPEG, GIF, SVG, or WebP)"})
				return
			}
		}

		// Serialize config to JSON
		configJSON, err := json.Marshal(input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize branding config"})
			return
		}

		var setting models.Settings
		result := db.Where("`key` = ?", "branding").First(&setting)

		if result.Error == gorm.ErrRecordNotFound {
			setting = models.Settings{
				Key:   "branding",
				Value: string(configJSON),
			}
			if err := db.Create(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save branding config"})
				return
			}
		} else if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch setting"})
			return
		} else {
			setting.Value = string(configJSON)
			if err := db.Save(&setting).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update branding config"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Branding settings saved",
			"config":  input,
		})
	}
}

// TestEmailConfig tests the email configuration by sending a test email
func TestEmailConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Email string `json:"email" binding:"required,email"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Valid email address required"})
			return
		}

		// Load email config from database
		var setting models.Settings
		if err := db.Where("`key` = ?", "email_config").First(&setting).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email not configured"})
			return
		}

		var config services.EmailConfig
		if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse email config"})
			return
		}

		// Create email service
		emailService, err := services.NewEmailServiceFromConfig(&config)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Send test email using confirmation email as test
		if err := emailService.SendConfirmationEmail(input.Email, "test-token-12345"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send test email: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Test email sent successfully"})
	}
}
