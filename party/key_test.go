package party

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
				if !isKeyChar(byte(r)) {
					t.Fatalf("key %q contains %q, which is outside the alphabet", key, r)
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

func TestKeyAlphabetIsCrockfordBase32(t *testing.T) {
	if len(keyAlphabet) != 32 {
		t.Fatalf("alphabet has %d symbols, want 32", len(keyAlphabet))
	}
	for _, forbidden := range []byte{'I', 'L', 'O', 'U'} {
		if strings.IndexByte(keyAlphabet, forbidden) >= 0 {
			t.Fatalf("alphabet contains the confusable letter %q", forbidden)
		}
	}
	seen := make(map[byte]bool)
	for _, r := range []byte(keyAlphabet) {
		if seen[r] {
			t.Fatalf("alphabet repeats %q", r)
		}
		seen[r] = true
	}
}

func TestNormalizeKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "cloudflare", input: "YOC4K7Q2M", want: "YOC4K7Q2M", ok: true},
		{name: "supabase", input: "YOSBQ0PGZ", want: "YOSBQ0PGZ", ok: true},
		{name: "lower case", input: "yoc4k7q2m", want: "YOC4K7Q2M", ok: true},
		{name: "spaces and hyphen", input: "  yoc4-k7q2m  ", want: "YOC4K7Q2M", ok: true},
		{name: "o instead of zero", input: "YOC4K7Q2O", want: "YOC4K7Q20", ok: true},
		{name: "i instead of one", input: "YOC4K7Q2I", want: "YOC4K7Q21", ok: true},
		{name: "l instead of one", input: "YOC4K7Q2L", want: "YOC4K7Q21", ok: true},
		{name: "u instead of v", input: "YOC4K7Q2U", want: "YOC4K7Q2V", ok: true},
		{name: "zero in the prefix", input: "Y0C4K7Q2M", want: "YOC4K7Q2M", ok: true},
		{name: "digits", input: "YOC012345", want: "YOC012345", ok: true},
		{name: "wrong prefix", input: "XXC4K7Q2M", ok: false},
		{name: "too short", input: "YOC4K7Q2", ok: false},
		{name: "too long", input: "YOC4K7Q2MM", ok: false},
		{name: "cyrillic", input: "УОC4K7Q2M", ok: false},
		{name: "unknown provider", input: "YOF4K7Q2M", ok: false},
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
		"YOC4K7Q2M":   Cloudflare,
		" yos4k7q2m ": Supabase,
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
	if _, err := ProviderFromKey("YOF4K7Q2M"); err == nil {
		t.Fatal("ProviderFromKey accepted a key without a known relay")
	}
}

func TestSanitizeKeyInput(t *testing.T) {
	cases := map[string]string{
		"yoc4k7q2m":      "YOC4K7Q2M",
		"YOC-4K7-Q2M":    "YOC4K7Q2M",
		"YOC4K7Q2MEXTRA": "YOC4K7Q2M",
		"YOC4K7Q2O":      "YOC4K7Q20",
		"привет":         "",
	}
	for input, want := range cases {
		if got := SanitizeKeyInput(input); got != want {
			t.Fatalf("SanitizeKeyInput(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAddressPinsPartyNamespace(t *testing.T) {
	address, err := Address("YOC4K7Q2M")
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	const want = "bd46a3e3b9cd4486ec6a4e75e10956b9"
	if address != want {
		t.Fatalf("address = %q, want %q; changing it breaks rooms across versions", address, want)
	}
}

func TestAddressHidesTheKey(t *testing.T) {
	key := "YOC4K7Q2M"
	address, err := Address(key)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	if len(address) != 32 {
		t.Fatalf("address %q has %d characters, want 32", address, len(address))
	}
	for _, r := range address {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			t.Fatalf("address %q contains %q outside hex", address, r)
		}
	}
	if strings.Contains(address, key) || strings.Contains(strings.ToLower(address), strings.ToLower(key)) {
		t.Fatalf("address %q leaks the key %q", address, key)
	}
	again, err := Address(strings.ToLower(key))
	if err != nil {
		t.Fatalf("Address lower case: %v", err)
	}
	if again != address {
		t.Fatalf("Address is not stable: %q != %q", again, address)
	}
	other, err := Address("YOC4K7Q2N")
	if err != nil {
		t.Fatalf("Address other: %v", err)
	}
	if other == address {
		t.Fatal("two keys produced the same room address")
	}
}
