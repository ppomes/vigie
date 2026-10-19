package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ollama translates with a model served by Ollama (https://ollama.com).
type Ollama struct {
	URL    string
	Model  string
	Client *http.Client
}

// NewOllama returns a client for the Ollama server at url.
func NewOllama(url, model string) *Ollama {
	return &Ollama{URL: strings.TrimRight(url, "/"), Model: model, Client: &http.Client{Timeout: 10 * time.Minute}}
}

// Check makes sure the server answers and has the model.
func (o *Ollama) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("no answer from Ollama at %s: %w", o.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama at %s answered %s", o.URL, resp.Status)
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("unexpected answer from Ollama at %s: %w", o.URL, err)
	}
	for _, m := range tags.Models {
		if m.Name == o.Model || m.Name == o.Model+":latest" {
			return nil
		}
	}
	return fmt.Errorf("Ollama at %s does not have the model %s: pull it first (ollama pull %s)", o.URL, o.Model, o.Model)
}

// Prompt is the instruction given to the model. Listing typical status page
// words in it made small models insert them where they did not belong, so
// it only asks for a faithful translation.
func Prompt(from, to, kind string) string {
	return fmt.Sprintf("You translate texts from the status page of an IT service, from %s into %s. "+
		"The text is %s. Translate exactly what is written, naturally, without adding or removing information. "+
		"Keep Markdown and HTML markup, URLs, host names, product names, abbreviations such as API, DB or VPN, "+
		"and numbers unchanged. Answer with the %s translation only.",
		Names[from], Names[to], kind, Names[to])
}

// Translate asks the model for a translation of text.
func (o *Ollama) Translate(ctx context.Context, text, from, to, kind string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":      o.Model,
		"stream":     false,
		"keep_alive": "15m",
		"options":    map[string]any{"temperature": 0},
		"messages": []map[string]string{
			{"role": "system", "content": Prompt(from, to, kind)},
			{"role": "user", "content": text},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.URL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("unexpected answer from Ollama (%s)", resp.Status)
	}
	if resp.StatusCode != http.StatusOK || out.Error != "" {
		return "", fmt.Errorf("Ollama: %s %s", resp.Status, out.Error)
	}
	t := Clean(out.Message.Content, text)
	if t == "" {
		return "", fmt.Errorf("Ollama returned an empty translation")
	}
	return t, nil
}

// Clean removes what models tend to add around a translation: spaces,
// quotes the source did not have.
func Clean(translation, source string) string {
	t := strings.TrimSpace(translation)
	for _, q := range [][2]string{{`"`, `"`}, {"«", "»"}, {"“", "”"}} {
		if !strings.HasPrefix(t, q[0]) || !strings.HasSuffix(t, q[1]) || len(t) <= len(q[0])+len(q[1]) ||
			strings.HasPrefix(strings.TrimSpace(source), q[0]) {
			continue
		}
		// Only quotes around the whole text: not two quoted words.
		if inner := t[len(q[0]) : len(t)-len(q[1])]; !strings.Contains(inner, q[0]) && !strings.Contains(inner, q[1]) {
			t = strings.TrimSpace(inner)
		}
	}
	return t
}
