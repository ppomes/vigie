package language

import (
	"testing"

	"api/internal/models"
	"api/internal/testutil"
)

func TestLoadAndSave(t *testing.T) {
	db := testutil.NewDB(t)
	s, exists, err := Load(db)
	if err != nil || exists || s != Defaults {
		t.Fatalf("unset: %+v %v %v", s, exists, err)
	}
	if err := Save(db, Settings{Default: "de", Mode: ModeFixed}); err == nil {
		t.Error("unsupported language accepted")
	}
	if err := Save(db, Settings{Default: "fr", Mode: "random"}); err == nil {
		t.Error("unknown mode accepted")
	}
	for _, want := range []Settings{{"fr", ModeFixed}, {"en", ModeBrowser}} {
		if err := Save(db, want); err != nil {
			t.Fatal(err)
		}
		if s, exists, err := Load(db); err != nil || !exists || s != want {
			t.Fatalf("got %+v %v %v, want %+v", s, exists, err, want)
		}
	}
	// A row written by hand with a bad value falls back to the defaults.
	if err := db.Model(&models.Settings{}).Where("`key` = ?", SettingKey).Update("value", `{"default":"xx"}`).Error; err != nil {
		t.Fatal(err)
	}
	if s, _, err := Load(db); err != nil || s != Defaults {
		t.Fatalf("bad row: %+v %v", s, err)
	}
}
