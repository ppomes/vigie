package totp

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfcSecret is the SHA1 key of RFC 6238's test vectors, "12345678901234567890".
var rfcSecret = encoding.EncodeToString([]byte("12345678901234567890"))

func TestRFC6238Vectors(t *testing.T) {
	// RFC 6238 appendix B gives 8-digit codes; the last 6 digits are the
	// 6-digit ones.
	for _, v := range []struct {
		unix int64
		code string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
	} {
		got, err := Code(rfcSecret, Step(time.Unix(v.unix, 0)))
		if err != nil {
			t.Fatal(err)
		}
		if want := v.code[2:]; got != want {
			t.Errorf("at %d: got %s, want %s", v.unix, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	now := time.Unix(1234567890, 0)
	code, _ := Code(rfcSecret, Step(now))
	previous, _ := Code(rfcSecret, Step(now)-1)
	old, _ := Code(rfcSecret, Step(now)-2)

	step, ok := Validate(rfcSecret, code, now, 0)
	if !ok || step != Step(now) {
		t.Fatalf("current code refused: %d %v", step, ok)
	}
	if _, ok := Validate(rfcSecret, previous, now, 0); !ok {
		t.Error("code from the previous step refused, clocks drift")
	}
	if _, ok := Validate(rfcSecret, old, now, 0); ok {
		t.Error("code from two steps ago accepted")
	}
	if _, ok := Validate(rfcSecret, code, now, Step(now)); ok {
		t.Error("code accepted twice")
	}
	if _, ok := Validate(rfcSecret, code[:3]+" "+code[3:], now, 0); !ok {
		t.Error("code with a space refused")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := Validate(rfcSecret, bad, now, 0); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNewSecret(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSecret()
	if a == b || len(a) != 32 {
		t.Errorf("secrets %q and %q", a, b)
	}
	if _, err := NormalizeSecret(a); err != nil {
		t.Error(err)
	}
}

func TestNormalizeSecret(t *testing.T) {
	// Cachet's secrets are 16 base32 characters.
	cachet := base32.StdEncoding.EncodeToString([]byte("0123456789"))
	got, err := NormalizeSecret(strings.ToLower(cachet[:8]) + " " + cachet[8:])
	if err != nil || got != cachet {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "not base32!", "AAAA"} {
		if _, err := NormalizeSecret(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestURL(t *testing.T) {
	got := URL("Acme status", "marie", "ABC")
	want := "otpauth://totp/Acme%20status:marie?algorithm=SHA1&digits=6&issuer=Acme+status&period=30&secret=ABC"
	if got != want {
		t.Errorf("got %s", got)
	}
}
