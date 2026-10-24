package watchparty

import (
	"strings"
	"testing"
)

func TestNewKeyFormat(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 512; i++ {
		key, err := NewKey()
		if err != nil {
			t.Fatalf("NewKey: %v", err)
		}
		if len(key) != KeyLength {
			t.Fatalf("key %q has %d characters, want %d", key, len(key), KeyLength)
		}
		if !strings.HasPrefix(key, KeyPrefix) {
			t.Fatalf("key %q does not start with %q", key, KeyPrefix)
		}
		if !ValidKey(key) {
			t.Fatalf("generated key %q is not valid", key)
		}
		for _, r := range key[len(KeyPrefix):] {
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

func TestNormalizeKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "canonical", input: "YOFBQOPGZ", want: "YOFBQOPGZ", ok: true},
		{name: "lower case", input: "yofbqopgz", want: "YOFBQOPGZ", ok: true},
		{name: "spaces", input: "  yofbqopgz  ", want: "YOFBQOPGZ", ok: true},
		{name: "wrong prefix", input: "XXYBQOPGZ", ok: false},
		{name: "too short", input: "YOFBQOPG", ok: false},
		{name: "too long", input: "YOFBQOPGZZ", ok: false},
		{name: "digits", input: "YOFBQ0PGZ", ok: false},
		{name: "cyrillic", input: "УОFBQOPGZ", ok: false},
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
