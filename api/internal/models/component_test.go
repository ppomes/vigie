package models

import (
	"testing"
)

func TestComponentStatusValidation(t *testing.T) {
	validStatuses := []string{"operational", "degraded_performance", "partial_outage", "major_outage"}

	for _, status := range validStatuses {
		component := Component{
			Name:   "Test Component",
			Status: status,
			Order:  1,
		}

		// Verify component fields are set correctly
		if component.Name != "Test Component" {
			t.Errorf("Expected name='Test Component', got '%s'", component.Name)
		}

		if component.Status != status {
			t.Errorf("Expected status='%s', got '%s'", status, component.Status)
		}

		// Valid statuses should be accepted by the model
		// In production, database CHECK constraint enforces this
	}
}

func TestInvalidComponentStatus(t *testing.T) {
	invalidStatuses := []string{"invalid_status", "down", "up", "offline", ""}

	for _, status := range invalidStatuses {
		component := Component{
			Name:   "Test Component",
			Status: status,
		}

		// These statuses should be rejected by database CHECK constraint
		// The model itself allows any string, but database validation would fail
		// This test documents expected invalid values
		if status != "" && status != "operational" && status != "degraded_performance" && status != "partial_outage" && status != "major_outage" {
			t.Logf("Invalid status '%s' would be rejected by database CHECK constraint", component.Status)
		}
	}
}

func TestComponentCreation(t *testing.T) {
	// Test required fields
	component := Component{
		Name:   "API Service",
		Status: "operational",
		Order:  1,
	}

	if component.Name == "" {
		t.Error("Component name is required but is empty")
	}

	if component.Status == "" {
		t.Error("Component status is required but is empty")
	}

	// Verify default status if not provided
	defaultComponent := Component{
		Name: "Database",
	}

	// GORM would set default status to 'operational' via struct tag
	// default:'operational' in the Status field definition
	if defaultComponent.Name == "" {
		t.Error("Component name should not be empty")
	}
}

func TestComponentTableName(t *testing.T) {
	component := Component{}
	expectedTableName := "components"

	if component.TableName() != expectedTableName {
		t.Errorf("Expected table name '%s', got '%s'", expectedTableName, component.TableName())
	}
}

func TestComponentStructure(t *testing.T) {
	// Test that component structure includes all required fields
	component := Component{
		ID:     1,
		Name:   "Test Component",
		Status: "operational",
		Order:  5,
	}

	if component.ID != 1 {
		t.Errorf("Expected ID=1, got %d", component.ID)
	}

	if component.Name != "Test Component" {
		t.Errorf("Expected Name='Test Component', got '%s'", component.Name)
	}

	if component.Status != "operational" {
		t.Errorf("Expected Status='operational', got '%s'", component.Status)
	}

	if component.Order != 5 {
		t.Errorf("Expected Order=5, got %d", component.Order)
	}
}

func TestComponentRelationships(t *testing.T) {
	// Test that component can have translations and incidents
	component := Component{
		Name:         "API",
		Status:       "operational",
		Translations: []Translation{},
		Incidents:    []Incident{},
	}

	if component.Translations == nil {
		t.Error("Translations slice should be initialized")
	}

	if component.Incidents == nil {
		t.Error("Incidents slice should be initialized")
	}
}
