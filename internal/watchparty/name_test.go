package watchparty

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewNameLooksLikeAGitHubHandle(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 128; i++ {
		name, err := NewName()
		if err != nil {
			t.Fatalf("NewName: %v", err)
		}
		parts := strings.Split(name, "-")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			t.Fatalf("name %q is not adjective-noun", name)
		}
		for _, part := range parts {
			for _, r := range part {
				if r < 'a' || r > 'z' {
					t.Fatalf("name %q is not lower-case ASCII", name)
				}
			}
		}
		if utf8.RuneCountInString(name) > NameLength {
			t.Fatalf("name %q is longer than %d runes", name, NameLength)
		}
		if _, err := NormalizeName(name); err != nil {
			t.Fatalf("NormalizeName(%q): %v", name, err)
		}
		seen[name] = true
	}
	if len(seen) < 100 {
		t.Fatalf("generated %d unique names out of 128", len(seen))
	}
}

func TestNormalizeName(t *testing.T) {
	long := strings.Repeat("щ", NameLength)
	tooLong := long + "щ"
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "trim", input: "  quiet-tanuki  ", want: "quiet-tanuki", ok: true},
		{name: "custom cyrillic", input: "Наруто", want: "Наруто", ok: true},
		{name: "spaces inside", input: "big tanuki", want: "big tanuki", ok: true},
		{name: "max length", input: long, want: long, ok: true},
		{name: "too long", input: tooLong, ok: false},
		{name: "empty", input: "", ok: false},
		{name: "spaces only", input: "   ", ok: false},
		{name: "control character", input: "quiet\ntanuki", ok: false},
		{name: "looks like a key", input: "YOC4K7Q2M", ok: false},
		{name: "lower-case key", input: "yoc4k7q2m", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeName(tc.input)
			if tc.ok {
				if err != nil {
					t.Fatalf("NormalizeName(%q): %v", tc.input, err)
				}
				if got != tc.want {
					t.Fatalf("NormalizeName(%q) = %q, want %q", tc.input, got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NormalizeName(%q) = %q, want an error", tc.input, got)
			}
		})
	}
}
