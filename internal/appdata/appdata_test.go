package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirMigratesLegacyDirectory(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, legacyDirName)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "marker"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, err := ensureDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != dirName {
		t.Fatalf("directory = %q, want %q", filepath.Base(dir), dirName)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "marker")); err != nil || string(data) != "keep" {
		t.Fatalf("migrated marker = %q, %v", data, err)
	}
}
