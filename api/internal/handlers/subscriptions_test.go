package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"api/internal/language"
	"api/internal/models"
	"api/internal/testutil"
)

func TestSubscribeToSomeComponents(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	api := testutil.CreateComponent(t, db, "API", "operational")
	hidden := createdID(t, call(t, r, http.MethodPost, "/components", map[string]any{
		"name": "Internal", "status": "operational", "enabled": false,
	}), "component")

	call(t, r, http.MethodPost, "/subscribe", map[string]any{"email": "Fan@Example.com", "component_ids": []uint{api.ID}})
	var s models.Subscriber
	db.Where("email = ?", "fan@example.com").First(&s)
	var subs []models.Subscription
	db.Where("subscriber_id = ?", s.ID).Find(&subs)
	if s.Global || len(subs) != 1 || subs[0].ComponentID != api.ID {
		t.Fatalf("subscriber %+v, subscriptions %+v", s, subs)
	}

	// Without a list, the subscriber follows everything.
	call(t, r, http.MethodPost, "/subscribe", map[string]any{"email": "all@example.com"})
	var all models.Subscriber
	db.Where("email = ?", "all@example.com").First(&all)
	if !all.Global {
		t.Fatal("a subscriber without components should be global")
	}

	if code := status(t, r, http.MethodPost, "/subscribe", map[string]any{
		"email": "x@example.com", "component_ids": []uint{hidden},
	}); code != http.StatusBadRequest {
		t.Fatalf("subscribing to a hidden component: status %d, want %d", code, http.StatusBadRequest)
	}
}

func TestManageSubscription(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)
	api := testutil.CreateComponent(t, db, "API", "operational")
	web := testutil.CreateComponent(t, db, "Website", "operational")
	s := models.Subscriber{Email: "fan@example.com", Confirmed: true, UnsubscribeToken: "tok"}
	db.Create(&s)

	var choice SubscriptionChoice
	get(t, r, "/subscriptions?token=tok", &choice)
	if !choice.Global || choice.Email != "fan@example.com" {
		t.Fatalf("initial choice: %+v", choice)
	}

	call(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": false, "component_ids": []uint{web.ID, api.ID, web.ID}})
	get(t, r, "/subscriptions?token=tok", &choice)
	if choice.Global || fmt.Sprint(choice.ComponentIDs) != fmt.Sprint([]uint{api.ID, web.ID}) {
		t.Fatalf("after choosing components: %+v", choice)
	}

	call(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": true})
	get(t, r, "/subscriptions?token=tok", &choice)
	if !choice.Global || len(choice.ComponentIDs) != 0 {
		t.Fatalf("after going global: %+v", choice)
	}

	if code := status(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": false}); code != http.StatusBadRequest {
		t.Fatalf("following nothing: status %d, want %d", code, http.StatusBadRequest)
	}
	if code := status(t, r, http.MethodGet, "/subscriptions?token=nope", nil); code != http.StatusNotFound {
		t.Fatalf("unknown token: status %d, want %d", code, http.StatusNotFound)
	}
}

func TestSubscriptionLanguage(t *testing.T) {
	db := testutil.NewDB(t)
	r := newRouter(db)

	// Without a choice, the subscriber gets the page's default language.
	if err := language.Save(db, language.Settings{Default: "fr", Mode: language.ModeFixed}); err != nil {
		t.Fatal(err)
	}
	call(t, r, http.MethodPost, "/subscribe", map[string]any{"email": "a@example.com"})
	call(t, r, http.MethodPost, "/subscribe", map[string]any{"email": "b@example.com", "locale": "en"})
	var a, b models.Subscriber
	db.Where("email = ?", "a@example.com").First(&a)
	db.Where("email = ?", "b@example.com").First(&b)
	if a.Locale != "fr" || b.Locale != "en" {
		t.Fatalf("locales: a %q, b %q", a.Locale, b.Locale)
	}

	// The manage page shows the language and can change it.
	if err := db.Model(&b).Update("unsubscribe_token", "tok").Error; err != nil {
		t.Fatal(err)
	}
	var choice SubscriptionChoice
	get(t, r, "/subscriptions?token=tok", &choice)
	if choice.Locale != "en" {
		t.Fatalf("choice: %+v", choice)
	}
	call(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": true, "locale": "fr"})
	get(t, r, "/subscriptions?token=tok", &choice)
	if choice.Locale != "fr" {
		t.Fatalf("after update: %+v", choice)
	}
	// Without a locale, an update keeps it.
	call(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": true})
	get(t, r, "/subscriptions?token=tok", &choice)
	if choice.Locale != "fr" {
		t.Fatalf("update without locale: %+v", choice)
	}
	if code := status(t, r, http.MethodPut, "/subscriptions?token=tok", map[string]any{"global": true, "locale": "de"}); code != http.StatusBadRequest {
		t.Fatalf("unsupported language: %d", code)
	}

	// A confirmed subscriber subscribing again is told so in a field the
	// page can translate.
	if err := db.Model(&b).Update("confirmed", true).Error; err != nil {
		t.Fatal(err)
	}
	w := call(t, r, http.MethodPost, "/subscribe", map[string]any{"email": "b@example.com"})
	if !strings.Contains(w.Body.String(), `"already_subscribed":true`) {
		t.Fatalf("subscribing again: %s", w.Body)
	}
}
