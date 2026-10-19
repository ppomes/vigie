// Package translate translates page content with a local language model,
// and detects the language a text is written in.
package translate

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/abadojack/whatlanggo"
)

// Translator translates one text. kind says what the text is (an incident
// title, a component name...), which helps short texts.
type Translator interface {
	Translate(ctx context.Context, text, from, to, kind string) (string, error)
}

// Names are the language names given to the model.
var Names = map[string]string{"en": "English", "fr": "French"}

var detectable = map[string]whatlanggo.Lang{"en": whatlanggo.Eng, "fr": whatlanggo.Fra}

// minConfidence is the detection confidence under which a text is taken to
// be in the page language. Short texts such as component names are often
// detected wrongly with a low confidence ("Website" comes out French at 0.69).
const minConfidence = 0.9

// Detect returns the language of text among locales, when the detection is
// confident enough to be trusted. Texts of one word are never detected:
// many ("Maintenance", "Migration", "Boutique") are words in both languages.
func Detect(text string, locales []string) (string, bool) {
	if len(words(text)) < 2 {
		return "", false
	}
	return detect(text, locales)
}

func detect(text string, locales []string) (string, bool) {
	opts := whatlanggo.Options{Whitelist: map[whatlanggo.Lang]bool{}}
	for _, l := range locales {
		if wl, ok := detectable[l]; ok {
			opts.Whitelist[wl] = true
		}
	}
	info := whatlanggo.DetectWithOptions(text, opts)
	if info.Confidence < minConfidence {
		return "", false
	}
	code := info.Lang.Iso6391()
	for _, l := range locales {
		if l == code {
			return l, true
		}
	}
	return "", false
}

// Majority returns the language most of texts are written in, when the
// texts make it clear: at least ten confident detections, and two thirds
// of them agreeing. Each text counts for its number of words, so that long
// messages weigh more than names.
func Majority(texts []string, locales []string) (string, bool) {
	votes := map[string]int{}
	total, detected := 0, 0
	for _, t := range texts {
		n := len(words(t))
		if n < 2 {
			continue
		}
		if l, ok := detect(t, locales); ok {
			votes[l] += n
			total += n
			detected++
		}
	}
	if detected < 10 {
		return "", false
	}
	for l, v := range votes {
		if 3*v >= 2*total {
			return l, true
		}
	}
	return "", false
}

var (
	urlLike  = regexp.MustCompile(`^(?i)([a-z][a-z0-9+.-]*://\S+|[a-z0-9-]+(\.[a-z0-9-]+)+(/\S*)?)$`)
	wordLike = regexp.MustCompile(`[\pL\pN][\pL\pN'’-]*`)
)

func words(text string) []string { return wordLike.FindAllString(text, -1) }

// Skip reports whether text needs no translation: no letters, a URL or a
// host name, or a single abbreviation such as API, DNS or VPN.
func Skip(text string) bool {
	t := strings.TrimSpace(text)
	if !strings.ContainsFunc(t, unicode.IsLetter) {
		return true
	}
	if urlLike.MatchString(t) {
		return true
	}
	if !strings.ContainsFunc(t, unicode.IsSpace) && len([]rune(t)) <= 6 && strings.ToUpper(t) == t {
		return true
	}
	return false
}
