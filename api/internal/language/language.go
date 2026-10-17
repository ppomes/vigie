// Package language holds the public page's default language and how the
// page picks the language of a visitor.
package language

import (
	"encoding/json"
	"errors"
	"fmt"

	"api/internal/models"

	"gorm.io/gorm"
)

// SettingKey is the settings row that stores the language setting, as JSON.
const SettingKey = "language"

// Modes: how the page picks a visitor's language when they have not chosen
// one with the switcher.
const (
	// ModeFixed shows the default language.
	ModeFixed = "fixed"
	// ModeBrowser follows the browser, and the default language when the
	// browser asks for a language the page does not have.
	ModeBrowser = "browser"
)

// Supported lists the languages of the page and of notification emails.
var Supported = []string{"en", "fr"}

// Settings is the language setting.
type Settings struct {
	Default string `json:"default"`
	Mode    string `json:"mode"`
}

// Defaults keep the page's behaviour from before the setting existed.
var Defaults = Settings{Default: "en", Mode: ModeBrowser}

// IsSupported reports whether locale is a language of the page.
func IsSupported(locale string) bool {
	for _, l := range Supported {
		if l == locale {
			return true
		}
	}
	return false
}

// Validate checks the values.
func (s Settings) Validate() error {
	if !IsSupported(s.Default) {
		return fmt.Errorf("default must be one of %v", Supported)
	}
	if s.Mode != ModeFixed && s.Mode != ModeBrowser {
		return fmt.Errorf("mode must be %q or %q", ModeFixed, ModeBrowser)
	}
	return nil
}

// Load reads the setting. Unset or invalid, it returns Defaults; exists
// reports whether a row was found.
func Load(db *gorm.DB) (s Settings, exists bool, err error) {
	var setting models.Settings
	err = db.Where(&models.Settings{Key: SettingKey}).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Defaults, false, nil
	}
	if err != nil {
		return Defaults, false, err
	}
	if json.Unmarshal([]byte(setting.Value), &s) != nil || s.Validate() != nil {
		return Defaults, true, nil
	}
	return s, true, nil
}

// Save stores s after validating it.
func Save(db *gorm.DB, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	value, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var setting models.Settings
	err = db.Where(&models.Settings{Key: SettingKey}).First(&setting).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return db.Create(&models.Settings{Key: SettingKey, Value: string(value)}).Error
	case err != nil:
		return err
	}
	return db.Model(&setting).Update("value", string(value)).Error
}
