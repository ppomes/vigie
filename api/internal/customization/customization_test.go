package customization

import (
	"strings"
	"testing"

	"api/internal/testutil"
)

func TestSanitizeHTML(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		// Kept: what a header or footer is made of.
		{`<a href="https://acme.test" class="brand">Acme</a>`, `<a href="https://acme.test" class="brand">Acme</a>`},
		{`<p style="text-align: center; color: #333">Hi</p>`, `<p style="text-align: center; color: #333">Hi</p>`},
		{`<img src="data:image/png;base64,iVBORw0KGgo=" alt="logo">`, `<img src="data:image/png;base64,iVBORw0KGgo=" alt="logo">`},
		// Removed: anything that runs code or reaches outside.
		{`<script>alert(1)</script><b>x</b>`, `<b>x</b>`},
		{`<img src=x onerror="alert(1)">`, `<img src="x">`},
		{`<a href="javascript:alert(1)">x</a>`, `x`},
		{`<iframe src="https://evil.test"></iframe>`, ``},
		{`<form action="https://evil.test"><input name=p></form>`, ``},
		{`<style>body{display:none}</style>`, ``},
		{`<div style="background-image: url(https://evil.test/t.png)">x</div>`, `<div>x</div>`},
	} {
		if got := SanitizeHTML(tc.in); got != tc.want {
			t.Errorf("SanitizeHTML(%s)\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	db := testutil.NewDB(t)
	c, err := Load(db)
	if err != nil || c != (Customization{}) {
		t.Fatalf("unset: %+v, %v", c, err)
	}
	saved, err := Save(db, Customization{CSS: "body{color:red}", HeaderHTML: `<b onclick="x()">Hi</b>`, FooterHTML: "<i>bye</i>"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.HeaderHTML != "<b>Hi</b>" {
		t.Errorf("saved header %q", saved.HeaderHTML)
	}
	if _, err := Save(db, Customization{CSS: "a{}"}); err != nil {
		t.Fatal(err)
	}
	c, err = Load(db)
	if err != nil || c.CSS != "a{}" || c.HeaderHTML != "" {
		t.Fatalf("after second save: %+v, %v", c, err)
	}
}

func TestValidate(t *testing.T) {
	if err := (Customization{CSS: strings.Repeat("a", MaxCSS+1)}).Validate(); err == nil {
		t.Error("CSS over the limit accepted")
	}
	if err := (Customization{FooterHTML: strings.Repeat("a", MaxHTML+1)}).Validate(); err == nil {
		t.Error("footer over the limit accepted")
	}
	if err := (Customization{CSS: "a{}"}).Validate(); err != nil {
		t.Error(err)
	}
}
