package handlers

import (
	"testing"

	"api/internal/models"
	"api/internal/testutil"
)

func TestReplaceTranslationsKeepsUneditedMachineTranslations(t *testing.T) {
	db := testutil.NewDB(t)
	for _, tr := range []models.Translation{
		{EntityID: 7, EntityType: "component", Locale: "fr", Field: "description", Value: "Base interne"},
		{EntityID: 7, EntityType: "component", Locale: "en", Field: "description", Value: "Internal base", Machine: true},
		{EntityID: 7, EntityType: "component", Locale: "en", Field: "name", Value: "Billing base", Machine: true},
		{EntityID: 7, EntityType: "component", Locale: "fr", Field: "name", Value: "Base de facturation", Machine: true},
	} {
		if err := db.Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
	}
	// The admin sends every translation back, with the English description
	// corrected, the English name left as the model made it, and the French
	// name accepted as it is.
	err := replaceTranslations(db, "component", 7, []TranslationRequest{
		{Locale: "fr", Field: "description", Value: "Base interne"},
		{Locale: "en", Field: "description", Value: "Internal database"},
		{Locale: "en", Field: "name", Value: "Billing base"},
		{Locale: "fr", Field: "name", Value: "Base de facturation", Reviewed: true},
		{Locale: "fr", Field: "message", Value: "   "},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var rows []models.Translation
	db.Where("entity_type = ? AND entity_id = ?", "component", 7).Find(&rows)
	got := map[string]models.Translation{}
	for _, r := range rows {
		got[r.Field+"/"+r.Locale] = r
	}
	if len(rows) != 4 {
		t.Fatalf("rows: %+v", rows)
	}
	if d := got["description/en"]; d.Value != "Internal database" || d.Machine {
		t.Errorf("edited translation still marked as machine-made: %+v", d)
	}
	if n := got["name/en"]; !n.Machine {
		t.Errorf("unedited machine translation lost its mark: %+v", n)
	}
	if n := got["name/fr"]; n.Machine || n.Value != "Base de facturation" {
		t.Errorf("translation accepted as reviewed still marked as machine-made: %+v", n)
	}
	if d := got["description/fr"]; d.Machine {
		t.Errorf("original marked as machine-made: %+v", d)
	}
}
