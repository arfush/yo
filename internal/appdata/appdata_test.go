package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirCreatesCurrentDataDirectory(t *testing.T) {
	base := t.TempDir()
	dir, err := ensureDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != dirName {
		t.Fatalf("directory = %q, want %q", filepath.Base(dir), dirName)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("data directory was not created: %v", err)
	}
}
