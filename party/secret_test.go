package party

import (
	"strings"
	"testing"
)

func TestNewSecretFormat(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 256; i++ {
		secret, err := NewSecret()
		if err != nil {
			t.Fatalf("NewSecret: %v", err)
		}
		if len(secret) != SecretLength {
			t.Fatalf("secret %q has %d characters, want %d", secret, len(secret), SecretLength)
		}
		if !ValidSecret(secret) {
			t.Fatalf("generated secret %q is not valid", secret)
		}
		for _, r := range []byte(secret) {
			if !isKeyChar(r) {
				t.Fatalf("secret %q contains %q outside the alphabet", secret, r)
			}
		}
		seen[secret] = true
	}
	if len(seen) < 250 {
		t.Fatalf("generated %d unique secrets out of 256", len(seen))
	}
}

func TestNormalizeSecret(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "empty", input: "", want: "", ok: true},
		{name: "spaces", input: "   ", want: "", ok: true},
		{name: "plain", input: "4K7Q2M9XBT", want: "4K7Q2M9XBT", ok: true},
		{name: "lower case", input: "4k7q2m9xbt", want: "4K7Q2M9XBT", ok: true},
		{name: "grouped", input: " 4K7Q2-M9XBT ", want: "4K7Q2M9XBT", ok: true},
		{name: "o instead of zero", input: "4K7Q2M9XBO", want: "4K7Q2M9XB0", ok: true},
		{name: "i instead of one", input: "4K7Q2M9XBI", want: "4K7Q2M9XB1", ok: true},
		{name: "u instead of v", input: "4K7Q2M9XBU", want: "4K7Q2M9XBV", ok: true},
		{name: "too short", input: "4K7Q2M9XB", ok: false},
		{name: "too long", input: "4K7Q2M9XBTX", ok: false},
		{name: "cyrillic", input: "4К7Q2M9XBT", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeSecret(tc.input)
			if tc.ok {
				if err != nil {
					t.Fatalf("NormalizeSecret(%q): %v", tc.input, err)
				}
				if got != tc.want {
					t.Fatalf("NormalizeSecret(%q) = %q, want %q", tc.input, got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NormalizeSecret(%q) = %q, want an error", tc.input, got)
			}
		})
	}
}

func TestSanitizeSecretInput(t *testing.T) {
	cases := map[string]string{
		"4k7q2m9xbt":      "4K7Q2M9XBT",
		"4K7Q2-M9XBT":     "4K7Q2M9XBT",
		"4K7Q2M9XBTEXTRA": "4K7Q2M9XBT",
		"4K7Q2M9XBO":      "4K7Q2M9XB0",
		"4K7Q2!M9XBT":     "4K7Q2M9XBT",
		"привет":          "",
	}
	for input, want := range cases {
		if got := SanitizeSecretInput(input); got != want {
			t.Fatalf("SanitizeSecretInput(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatSecret(t *testing.T) {
	if got := FormatSecret("4K7Q2M9XBT"); got != "4K7Q2-M9XBT" {
		t.Fatalf("FormatSecret = %q, want 4K7Q2-M9XBT", got)
	}
	if got := FormatSecret("short"); got != "short" {
		t.Fatalf("FormatSecret(short) = %q", got)
	}
	if got := FormatSecret(""); got != "" {
		t.Fatalf("FormatSecret(empty) = %q", got)
	}
}

// TestSecretIsNotAKey keeps the two codes distinct, because both are pasted in
// the same chat message and the UI has two fields.
func TestSecretIsNotAKey(t *testing.T) {
	if !strings.HasPrefix(KeyPrefix, "YO") {
		t.Fatal("the key prefix changed; update the documentation with it")
	}
	if _, err := NormalizeKey("4K7Q2M9XBT"); err == nil {
		t.Fatal("a secret parses as a room key")
	}
}
