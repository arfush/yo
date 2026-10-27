package install

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"1.1":   "1.1",
		"v1.1":  "1.1",
		" v2.0": "2.0",
		"":      "",
	}
	for input, want := range cases {
		if got := normalizeVersion(input); got != want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", input, got, want)
		}
	}
}
