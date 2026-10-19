package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

var locales = []string{"en", "fr"}

func TestSkip(t *testing.T) {
	for _, s := range []string{"", "  ", "8.4", "https://api.acme.fr/v1", "api.acme.fr", "www.acme.fr", "VPN", "API"} {
		if !Skip(s) {
			t.Errorf("Skip(%q) = false", s)
		}
	}
	for _, s := range []string{"Billing DB", "Website", "Mystery", "Base de données", "API publique"} {
		if Skip(s) {
			t.Errorf("Skip(%q) = true", s)
		}
	}
}

func TestDetect(t *testing.T) {
	for text, want := range map[string]string{
		"Correctif déployé, nous surveillons.":        "fr",
		"Nous constatons des temps de réponse élevés.": "fr",
		"Update of a deleted incident":                 "en",
		"Billing DB down":                              "en",
	} {
		if got, ok := Detect(text, locales); !ok || got != want {
			t.Errorf("Detect(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
	// One word, or a low confidence: left to the page language.
	for _, text := range []string{"Mystery", "Boutique", "Website", "DNS change", "Site web"} {
		if got, ok := Detect(text, locales); ok {
			t.Errorf("Detect(%q) = %q, want no detection", text, got)
		}
	}
}

func TestMajority(t *testing.T) {
	french := []string{
		"Nous constatons des temps de réponse élevés.", "Une partie des paiements par carte échoue.",
		"La base principale ne répond plus.", "Correctif déployé, nous surveillons.",
		"Le prestataire de paiement confirme un incident de son côté.", "Bascule sur le serveur secondaire, service rétabli.",
		"Coupures brèves possibles.", "Abonnez-vous pour être prévenu des incidents.",
		"Cause identifiée : un index manquant après la mise à jour.", "Coupure de la base de données",
	}
	texts := append([]string{"Update of a deleted incident", "Billing DB down", "VPN"}, french...)
	if got, ok := Majority(texts, locales); !ok || got != "fr" {
		t.Fatalf("Majority = %q, %v; want fr", got, ok)
	}
	if got, ok := Majority(french[:5], locales); ok {
		t.Fatalf("Majority of 5 texts = %q, want no answer", got)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"  Fix deployed\n":   "Fix deployed",
		`"Fix deployed"`:     "Fix deployed",
		"« Correctif »":      "Correctif",
		`"quoted" and "not"`: `"quoted" and "not"`,
	} {
		if got := Clean(in, "source"); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Clean(`"Quoted"`, `"Cité"`); got != `"Quoted"` {
		t.Errorf("quotes of a quoted source were removed: %q", got)
	}
}

func TestCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c, err := OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	key := Key("m", "fr", "en", "kind", "Bonjour")
	if _, ok := c.Get(key); ok {
		t.Fatal("empty cache has an entry")
	}
	if err := c.Put(key, "Hello"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c, err = OpenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Get(key); !ok || v != "Hello" {
		t.Fatalf("reopened cache: %q, %v", v, ok)
	}
	if Key("other", "fr", "en", "kind", "Bonjour") == key || Key("m", "fr", "en", "other", "Bonjour") == key {
		t.Fatal("model and kind are not part of the key")
	}
}

func TestOllama(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"qwen2.5:7b"},{"name":"mistral:latest"}]}`))
		case "/api/chat":
			json.NewDecoder(r.Body).Decode(&got)
			if strings.Contains(r.Header.Get("Content-Type"), "json") && got["model"] == "broken" {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"model crashed"}`))
				return
			}
			w.Write([]byte(`{"message":{"role":"assistant","content":"\"Fix deployed\"\n"}}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	for _, model := range []string{"qwen2.5:7b", "mistral"} {
		if err := NewOllama(srv.URL+"/", model).Check(ctx); err != nil {
			t.Errorf("Check(%s): %v", model, err)
		}
	}
	if err := NewOllama(srv.URL, "llama3").Check(ctx); err == nil || !strings.Contains(err.Error(), "ollama pull llama3") {
		t.Errorf("missing model: %v", err)
	}
	if err := NewOllama("http://127.0.0.1:1", "m").Check(ctx); err == nil {
		t.Error("unreachable server: no error")
	}

	out, err := NewOllama(srv.URL, "qwen2.5:7b").Translate(ctx, "Correctif déployé", "fr", "en", "an update on an incident")
	if err != nil || out != "Fix deployed" {
		t.Fatalf("Translate = %q, %v", out, err)
	}
	msgs := got["messages"].([]any)
	system := msgs[0].(map[string]any)["content"].(string)
	if !strings.Contains(system, "from French into English") || !strings.Contains(system, "an update on an incident") ||
		msgs[1].(map[string]any)["content"] != "Correctif déployé" || got["stream"] != false {
		t.Fatalf("request: %v", got)
	}
	if _, err := NewOllama(srv.URL, "broken").Translate(ctx, "x", "fr", "en", "k"); err == nil || !strings.Contains(err.Error(), "model crashed") {
		t.Fatalf("server error: %v", err)
	}
}
