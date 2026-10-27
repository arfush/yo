package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FileSHA256(path)
	if err != nil {
		t.Fatalf("FileSHA256: %v", err)
	}
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
}

func TestFileSHA256RejectsMissingFile(t *testing.T) {
	if _, err := FileSHA256(filepath.Join(t.TempDir(), "missing.bin")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestIsUpdate(t *testing.T) {
	const currentVersion = "v1.0"
	const currentSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newer := &Release{Version: "v1.1", SHA256: currentSHA}
	older := &Release{Version: "v0.9", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	sameBuild := &Release{Version: "v1.0", SHA256: currentSHA}
	sameVersionFix := &Release{Version: "v1.0.0", SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}

	cases := []struct {
		name    string
		release *Release
		sha     string
		want    bool
	}{
		{name: "newer version", release: newer, sha: currentSHA, want: true},
		{name: "older version", release: older, sha: currentSHA, want: false},
		{name: "same version same build", release: sameBuild, sha: currentSHA, want: false},
		{name: "same version fixed build", release: sameVersionFix, sha: currentSHA, want: true},
		{name: "same version unknown local hash", release: sameVersionFix, sha: "", want: false},
		{name: "no release", release: nil, sha: currentSHA, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUpdate(tc.release, currentVersion, tc.sha); got != tc.want {
				t.Fatalf("IsUpdate() = %v, want %v", got, tc.want)
			}
		})
	}
}
