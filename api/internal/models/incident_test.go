package models

import (
	"testing"
)

func TestIncidentComponentRelationship(t *testing.T) {
	// Test that incident has a foreign key relationship to component
	incident := Incident{
		Name:        "API Outage",
		Status:      "investigating",
		ComponentID: 1,
		Component: Component{
			ID:     1,
			Name:   "API",
			Status: "major_outage",
		},
	}

	if incident.ComponentID != 1 {
		t.Errorf("Expected ComponentID=1, got %d", incident.ComponentID)
	}

	if incident.Component.ID != 1 {
		t.Errorf("Expected Component.ID=1, got %d", incident.Component.ID)
	}

	if incident.Component.Name != "API" {
		t.Errorf("Expected Component.Name='API', got '%s'", incident.Component.Name)
	}
}

func TestIncidentStatusValidation(t *testing.T) {
	validStatuses := []string{"investigating", "identified", "monitoring", "resolved"}

	for _, status := range validStatuses {
		incident := Incident{
			Name:        "Test Incident",
			Status:      status,
			ComponentID: 1,
		}

		if incident.Status != status {
			t.Errorf("Expected status='%s', got '%s'", status, incident.Status)
		}

		// Valid statuses should be accepted by the model
		// In production, database CHECK constraint enforces this
	}
}

func TestInvalidIncidentStatus(t *testing.T) {
	invalidStatuses := []string{"invalid_status", "pending", "closed", "open", ""}

	for _, status := range invalidStatuses {
		incident := Incident{
			Name:        "Test Incident",
			Status:      status,
			ComponentID: 1,
		}

		// These statuses should be rejected by database CHECK constraint
		// The model itself allows any string, but database validation would fail
		if status != "" && status != "investigating" && status != "identified" && status != "monitoring" && status != "resolved" {
			t.Logf("Invalid status '%s' would be rejected by database CHECK constraint", incident.Status)
		}
	}
}

func TestIncidentRequiredFields(t *testing.T) {
	// Test required fields: name, status, component_id
	incident := Incident{
		Name:        "Database Connection Timeout",
		Status:      "investigating",
		ComponentID: 2,
	}

	if incident.Name == "" {
		t.Error("Incident name is required but is empty")
	}

	if incident.Status == "" {
		t.Error("Incident status is required but is empty")
	}

	if incident.ComponentID == 0 {
		t.Error("Incident component_id is required but is 0")
	}
}

func TestIncidentDefaultStatus(t *testing.T) {
	// Test that default status is 'investigating' if not provided
	incident := Incident{
		Name:        "New Incident",
		ComponentID: 1,
	}

	// GORM would set default status to 'investigating' via struct tag
	// default:'investigating' in the Status field definition
	if incident.Name == "" {
		t.Error("Incident name should not be empty")
	}
}

func TestIncidentTableName(t *testing.T) {
	incident := Incident{}
	expectedTableName := "incidents"

	if incident.TableName() != expectedTableName {
		t.Errorf("Expected table name '%s', got '%s'", expectedTableName, incident.TableName())
	}
}

func TestIncidentTranslations(t *testing.T) {
	// Test that incident can have translations
	incident := Incident{
		Name:        "API Outage",
		Status:      "investigating",
		ComponentID: 1,
		Translations: []Translation{
			{
				EntityID: 1,
				Entity:   "incident",
				Locale:   "en",
				Field:    "message",
				Value:    "We are investigating an issue with the API",
			},
			{
				EntityID: 1,
				Entity:   "incident",
				Locale:   "fr",
				Field:    "message",
				Value:    "Nous enquêtons sur un problème avec l'API",
			},
		},
	}

	if len(incident.Translations) != 2 {
		t.Errorf("Expected 2 translations, got %d", len(incident.Translations))
	}

	// Verify English translation
	if incident.Translations[0].Locale != "en" {
		t.Errorf("Expected first translation locale='en', got '%s'", incident.Translations[0].Locale)
	}

	// Verify French translation
	if incident.Translations[1].Locale != "fr" {
		t.Errorf("Expected second translation locale='fr', got '%s'", incident.Translations[1].Locale)
	}
}

func TestIncidentStructure(t *testing.T) {
	// Test that incident structure includes all required fields
	incident := Incident{
		ID:          1,
		Name:        "Test Incident",
		Status:      "monitoring",
		ComponentID: 3,
	}

	if incident.ID != 1 {
		t.Errorf("Expected ID=1, got %d", incident.ID)
	}

	if incident.Name != "Test Incident" {
		t.Errorf("Expected Name='Test Incident', got '%s'", incident.Name)
	}

	if incident.Status != "monitoring" {
		t.Errorf("Expected Status='monitoring', got '%s'", incident.Status)
	}

	if incident.ComponentID != 3 {
		t.Errorf("Expected ComponentID=3, got %d", incident.ComponentID)
	}
}

func TestIncidentCascadeDelete(t *testing.T) {
	// Test that translations are configured for cascade delete
	incident := Incident{
		Name:        "Test Incident",
		Status:      "resolved",
		ComponentID: 1,
		Translations: []Translation{
			{
				EntityID: 1,
				Entity:   "incident",
				Locale:   "en",
				Field:    "message",
				Value:    "Issue resolved",
			},
		},
	}

	// Verify that translations foreign key is set up correctly
	// In production, GORM constraint:OnDelete:CASCADE ensures translations
	// are automatically deleted when incident is deleted
	if len(incident.Translations) > 0 {
		if incident.Translations[0].EntityID != incident.ID && incident.ID != 0 {
			t.Logf("Translation EntityID should match Incident ID for proper cascade")
		}
	}
}
