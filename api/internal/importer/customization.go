package importer

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"api/internal/customization"
)

// cachetColours maps Cachet's colour settings to the CSS variables of
// Vigie's themes. Cachet uses greens for operational and fixed, blues for
// performance issues and watching, yellows for partial outages and
// identified, reds for major outages, oranges for investigating.
var cachetColours = []struct {
	setting string
	vars    []string
}{
	{"style_background_color", []string{"--color-background"}},
	{"style_background_fills", []string{"--color-card"}},
	{"style_text_color", []string{"--color-text"}},
	{"style_banner_background_color", []string{"--color-header"}},
	{"style_links", []string{"--color-primary"}},
	{"style_greens", []string{"--color-operational", "--color-resolved"}},
	{"style_blues", []string{"--color-degraded", "--color-monitoring"}},
	{"style_yellows", []string{"--color-partial-outage", "--color-identified"}},
	{"style_reds", []string{"--color-major-outage"}},
	{"style_oranges", []string{"--color-investigating"}},
}

// cachetStylesUnmapped are Cachet style settings Vigie has no equivalent for.
var cachetStylesUnmapped = []string{"style_greys", "style_metrics", "style_banner_padding", "style_fullwidth_header"}

// cssColour accepts the colour values Cachet's colour pickers write, and
// nothing that could end the CSS declaration.
var cssColour = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[a-zA-Z]+|rgba?\([0-9., %]+\))$`)

// customizationFromCachet builds Vigie's custom CSS, header and footer from
// Cachet's settings, and reports what needs a look.
func (im *importer) customizationFromCachet() error {
	for _, name := range cachetStylesUnmapped {
		if v := strings.TrimSpace(im.settings[name]); v != "" {
			im.report.skip("style setting with no equivalent in Vigie (redo it in the custom CSS if needed)", name+" = "+v)
		}
	}
	var css strings.Builder
	var colours []string
	for _, c := range cachetColours {
		v := strings.TrimSpace(im.settings[c.setting])
		if v == "" {
			continue
		}
		if !cssColour.MatchString(v) {
			im.report.skip(fmt.Sprintf("colour setting with an unexpected value (%s)", c.setting), v)
			continue
		}
		for _, name := range c.vars {
			colours = append(colours, fmt.Sprintf("  %s: %s;", name, v))
		}
	}
	if len(colours) > 0 {
		css.WriteString("/* Colours from Cachet */\n:root {\n" + strings.Join(colours, "\n") + "\n}\n")
	}
	if s := strings.TrimSpace(im.settings["stylesheet"]); s != "" {
		if css.Len() > 0 {
			css.WriteString("\n")
		}
		css.WriteString("/* Cachet's custom stylesheet */\n" + s + "\n")
		im.report.note("Cachet's custom stylesheet was copied to the custom CSS. Its selectors were written for Cachet's page: " +
			"check them under Settings, Customization in the admin.")
	}

	header := strings.TrimSpace(im.settings["header"])
	footer := strings.TrimSpace(im.settings["footer"])
	if css.Len() == 0 && header == "" && footer == "" {
		return nil
	}

	existing, err := customization.Load(im.dst)
	if err != nil {
		return err
	}
	if existing != (customization.Customization{}) {
		im.report.note("Vigie already has a custom CSS, header or footer: Cachet's were not applied.")
		return nil
	}

	c := customization.Customization{CSS: css.String(), HeaderHTML: header, FooterHTML: footer}
	if err := c.Validate(); err != nil {
		im.report.skip("custom CSS, header and footer ("+err.Error()+")", "settings")
		return nil
	}
	saved, err := customization.Save(im.dst, c)
	if err != nil {
		return err
	}
	for _, part := range []struct{ name, before, after string }{
		{"header", header, saved.HeaderHTML},
		{"footer", footer, saved.FooterHTML},
	} {
		if part.before != part.after {
			im.report.adapt("custom HTML changed by sanitizing: scripts, event handlers, forms and frames are not allowed; check it in the admin", part.name)
		}
	}
	im.report.imported("settings")
	return nil
}

// cachetBanner returns Cachet's banner image as a data URL for Vigie's
// logo, or "" when there is none or it does not fit.
func (im *importer) cachetBanner() string {
	banner := strings.TrimSpace(im.settings["app_banner"])
	if banner == "" {
		return ""
	}
	mime := strings.ToLower(strings.TrimSpace(im.settings["app_banner_type"]))
	switch mime {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/svg+xml", "image/webp":
	default:
		im.report.skip("banner image (type not supported by Vigie's logo)", mime)
		return ""
	}
	if data, err := base64.StdEncoding.DecodeString(banner); err != nil || len(data) == 0 {
		im.report.skip("banner image (not valid base64)", "app_banner")
		return ""
	}
	logo := "data:" + mime + ";base64," + banner
	if len(logo) > 500*1024 {
		im.report.skip("banner image (larger than Vigie's 500 KB logo limit)", "app_banner")
		return ""
	}
	return logo
}
