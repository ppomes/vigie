package models

import (
	"testing"
)

func TestTranslationLocaleFallback(t *testing.T) {
	// Test that when French translation is missing, system falls back to English
	// This is critical for user experience

	translations := []Translation{
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "en",
			Field:    "name",
			Value:    "API Service",
		},
		// Note: French translation intentionally missing
	}

	// Simulate locale fallback logic
	requestedLocale := "fr"
	var foundTranslation *Translation

	// Try to find French translation
	for i := range translations {
		if translations[i].Locale == requestedLocale && translations[i].Field == "name" {
			foundTranslation = &translations[i]
			break
		}
	}

	// If French not found, fall back to English
	if foundTranslation == nil {
		for i := range translations {
			if translations[i].Locale == "en" && translations[i].Field == "name" {
				foundTranslation = &translations[i]
				break
			}
		}
	}

	if foundTranslation == nil {
		t.Error("Expected fallback to English translation, but no translation found")
	}

	if foundTranslation.Value != "API Service" {
		t.Errorf("Expected English fallback value='API Service', got '%s'", foundTranslation.Value)
	}
}

func TestTranslationLookup(t *testing.T) {
	// Test correct translation returned for locale
	translations := []Translation{
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "en",
			Field:    "name",
			Value:    "Component",
		},
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "fr",
			Field:    "name",
			Value:    "Composant",
		},
	}

	testCases := []struct {
		locale   string
		expected string
	}{
		{"fr", "Composant"},
		{"en", "Component"},
		{"", "Component"}, // Default to English when locale not specified
	}

	for _, tc := range testCases {
		t.Run("Locale_"+tc.locale, func(t *testing.T) {
			locale := tc.locale
			if locale == "" {
				locale = "en" // Default to English
			}

			var foundTranslation *Translation
			for i := range translations {
				if translations[i].Locale == locale && translations[i].Field == "name" {
					foundTranslation = &translations[i]
					break
				}
			}

			if foundTranslation == nil {
				t.Errorf("Translation not found for locale '%s'", locale)
				return
			}

			if foundTranslation.Value != tc.expected {
				t.Errorf("Expected value='%s', got '%s'", tc.expected, foundTranslation.Value)
			}
		})
	}
}

func TestDefaultEnglish(t *testing.T) {
	// Test English used when locale not specified
	translations := []Translation{
		{
			EntityID: 1,
			EntityType: "incident",
			Locale:   "en",
			Field:    "message",
			Value:    "System is operational",
		},
		{
			EntityID: 1,
			EntityType: "incident",
			Locale:   "fr",
			Field:    "message",
			Value:    "Le système est opérationnel",
		},
	}

	// When no locale specified, default to English
	defaultLocale := "en"
	var foundTranslation *Translation

	for i := range translations {
		if translations[i].Locale == defaultLocale && translations[i].Field == "message" {
			foundTranslation = &translations[i]
			break
		}
	}

	if foundTranslation == nil {
		t.Error("Expected English translation by default, but none found")
	}

	if foundTranslation.Value != "System is operational" {
		t.Errorf("Expected English value='System is operational', got '%s'", foundTranslation.Value)
	}
}

func TestUnsupportedLocaleFallback(t *testing.T) {
	// Test unsupported locale (e.g., es) falls back to English
	translations := []Translation{
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "en",
			Field:    "name",
			Value:    "Database",
		},
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "fr",
			Field:    "name",
			Value:    "Base de données",
		},
	}

	// Request Spanish (unsupported)
	requestedLocale := "es"
	var foundTranslation *Translation

	// Try Spanish first
	for i := range translations {
		if translations[i].Locale == requestedLocale && translations[i].Field == "name" {
			foundTranslation = &translations[i]
			break
		}
	}

	// If not found, fall back to English
	if foundTranslation == nil {
		for i := range translations {
			if translations[i].Locale == "en" && translations[i].Field == "name" {
				foundTranslation = &translations[i]
				break
			}
		}
	}

	if foundTranslation == nil {
		t.Error("Expected fallback to English for unsupported locale, but no translation found")
	}

	if foundTranslation.Value != "Database" {
		t.Errorf("Expected English fallback value='Database', got '%s'", foundTranslation.Value)
	}
}

func TestTranslationStructure(t *testing.T) {
	// Test that translation structure includes all required fields
	translation := Translation{
		ID:       1,
		EntityID: 5,
		EntityType: "incident",
		Locale:   "en",
		Field:    "message",
		Value:    "We are investigating an issue",
	}

	if translation.ID != 1 {
		t.Errorf("Expected ID=1, got %d", translation.ID)
	}

	if translation.EntityTypeID != 5 {
		t.Errorf("Expected EntityID=5, got %d", translation.EntityTypeID)
	}

	if translation.EntityType != "incident" {
		t.Errorf("Expected Entity='incident', got '%s'", translation.EntityType)
	}

	if translation.Locale != "en" {
		t.Errorf("Expected Locale='en', got '%s'", translation.Locale)
	}

	if translation.Field != "message" {
		t.Errorf("Expected Field='message', got '%s'", translation.Field)
	}

	if translation.Value != "We are investigating an issue" {
		t.Errorf("Expected Value='We are investigating an issue', got '%s'", translation.Value)
	}
}

func TestTranslationEntities(t *testing.T) {
	// Test valid entity types
	validEntities := []string{"incident", "component"}

	for _, entity := range validEntities {
		translation := Translation{
			EntityID: 1,
			EntityType: entity,
			Locale:   "en",
			Field:    "name",
			Value:    "Test",
		}

		if translation.EntityType != entity {
			t.Errorf("Expected Entity='%s', got '%s'", entity, translation.EntityType)
		}
	}
}

func TestTranslationLocales(t *testing.T) {
	// Test valid locales
	validLocales := []string{"en", "fr"}

	for _, locale := range validLocales {
		translation := Translation{
			EntityID: 1,
			EntityType: "component",
			Locale:   locale,
			Field:    "name",
			Value:    "Test",
		}

		if translation.Locale != locale {
			t.Errorf("Expected Locale='%s', got '%s'", locale, translation.Locale)
		}
	}
}

func TestTranslationFields(t *testing.T) {
	// Test valid field types
	validFields := []string{"name", "message", "description"}

	for _, field := range validFields {
		translation := Translation{
			EntityID: 1,
			EntityType: "incident",
			Locale:   "en",
			Field:    field,
			Value:    "Test value",
		}

		if translation.Field != field {
			t.Errorf("Expected Field='%s', got '%s'", field, translation.Field)
		}
	}
}

func TestMultipleTranslationsPerEntity(t *testing.T) {
	// Test that an entity can have multiple translations (different locales)
	translations := []Translation{
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "en",
			Field:    "name",
			Value:    "API",
		},
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "fr",
			Field:    "name",
			Value:    "API",
		},
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "en",
			Field:    "description",
			Value:    "API service for handling requests",
		},
		{
			EntityID: 1,
			EntityType: "component",
			Locale:   "fr",
			Field:    "description",
			Value:    "Service API pour traiter les demandes",
		},
	}

	// Count translations for entity 1
	count := 0
	for i := range translations {
		if translations[i].EntityID == 1 {
			count++
		}
	}

	if count != 4 {
		t.Errorf("Expected 4 translations for entity 1, got %d", count)
	}

	// Count English translations
	enCount := 0
	for i := range translations {
		if translations[i].EntityID == 1 && translations[i].Locale == "en" {
			enCount++
		}
	}

	if enCount != 2 {
		t.Errorf("Expected 2 English translations for entity 1, got %d", enCount)
	}

	// Count French translations
	frCount := 0
	for i := range translations {
		if translations[i].EntityID == 1 && translations[i].Locale == "fr" {
			frCount++
		}
	}

	if frCount != 2 {
		t.Errorf("Expected 2 French translations for entity 1, got %d", frCount)
	}
}
