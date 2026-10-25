package watchparty

import (
	"strings"
	"testing"
)

func TestNewKeyFormat(t *testing.T) {
	for _, provider := range Providers() {
		seen := make(map[string]bool)
		code, ok := providerKeyCode(provider)
		if !ok {
			t.Fatalf("providerKeyCode(%q) is missing", provider)
		}
		for i := 0; i < 512; i++ {
			key, err := NewKey(provider)
			if err != nil {
				t.Fatalf("NewKey(%q): %v", provider, err)
			}
			if len(key) != KeyLength {
				t.Fatalf("key %q has %d characters, want %d", key, len(key), KeyLength)
			}
			if !strings.HasPrefix(key, KeyPrefix) {
				t.Fatalf("key %q does not start with %q", key, KeyPrefix)
			}
			if key[keyProviderIndex] != code {
				t.Fatalf("key %q does not name %q", key, provider)
			}
			if !ValidKey(key) {
				t.Fatalf("generated key %q is not valid", key)
			}
			if got, err := ProviderFromKey(key); err != nil || got != provider {
				t.Fatalf("ProviderFromKey(%q) = %q, %v; want %q", key, got, err, provider)
			}
			for _, r := range key[keyProviderIndex+1:] {
				if r < 'A' || r > 'Z' {
					t.Fatalf("key %q contains %q", key, r)
				}
			}
			seen[key] = true
		}
		if len(seen) < 500 {
			t.Fatalf("generated %d unique keys out of 512, the suffix is too predictable", len(seen))
		}
	}
}

func TestNewKeyRejectsAnUnknownProvider(t *testing.T) {
	if _, err := NewKey(Provider("x")); err == nil {
		t.Fatal("NewKey accepted an unknown provider")
	}
}

func TestNormalizeKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "cloudflare", input: "YOCBQOPGZ", want: "YOCBQOPGZ", ok: true},
		{name: "supabase", input: "YOSBQOPGZ", want: "YOSBQOPGZ", ok: true},
		{name: "lower case", input: "yocbqopgz", want: "YOCBQOPGZ", ok: true},
		{name: "spaces", input: "  yocbqopgz  ", want: "YOCBQOPGZ", ok: true},
		{name: "wrong prefix", input: "XXCBQOPGZ", ok: false},
		{name: "too short", input: "YOCBQOPG", ok: false},
		{name: "too long", input: "YOCBQOPGZZ", ok: false},
		{name: "digits", input: "YOCBQ0PGZ", ok: false},
		{name: "cyrillic", input: "УОCBQOPGZ", ok: false},
		{name: "unknown provider", input: "YOFBQOPGZ", ok: false},
		{name: "empty", input: "", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeKey(tc.input)
			if tc.ok {
				if err != nil {
					t.Fatalf("NormalizeKey(%q) error: %v", tc.input, err)
				}
				if got != tc.want {
					t.Fatalf("NormalizeKey(%q) = %q, want %q", tc.input, got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NormalizeKey(%q) = %q, want an error", tc.input, got)
			}
		})
	}
}

func TestProviderFromKey(t *testing.T) {
	cases := map[string]Provider{
		"YOCBQOPGZ":   Cloudflare,
		" yosbqopgz ": Supabase,
	}
	for key, want := range cases {
		got, err := ProviderFromKey(key)
		if err != nil {
			t.Fatalf("ProviderFromKey(%q): %v", key, err)
		}
		if got != want {
			t.Fatalf("ProviderFromKey(%q) = %q, want %q", key, got, want)
		}
	}
	if _, err := ProviderFromKey("YOFBQOPGZ"); err == nil {
		t.Fatal("ProviderFromKey accepted a key without a known relay")
	}
}
