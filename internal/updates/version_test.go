package updates

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"v1.0", "v1.0.0", 0},
		{"v1.2", "v1.3", -1},
		{"v2", "v1.9.9", 1},
		{"v1.10", "v1.9", 1},
		{"1.0", "v1.0", 0},
		{"broken", "v1", 0},
	}
	for _, test := range tests {
		if got := CompareVersions(test.left, test.right); got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v1.3.0", "1.0") {
		t.Fatal("v1.3.0 should be newer than 1.0")
	}
	if IsNewer("v1.0", "v1.0.0") {
		t.Fatal("equal versions must not be newer")
	}
	if IsNewer("nope", "v1.0") {
		t.Fatal("invalid versions must not be newer")
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := NormalizeVersion(" v01.3.0.0 "); got != "v1.3.0.0" {
		t.Fatalf("NormalizeVersion = %q", got)
	}
	if got := NormalizeVersion("1.2.3.4.5"); got != "" {
		t.Fatalf("NormalizeVersion accepted invalid version: %q", got)
	}
}
