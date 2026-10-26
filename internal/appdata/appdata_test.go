package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigratePartyConfigRenamesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyPartyName)
	if err := os.WriteFile(legacy, []byte(`{"cloudflare_url":"wss://example.test"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := migratePartyConfig(dir)
	if got != filepath.Join(dir, partyName) {
		t.Fatalf("config path = %q, want %q", got, filepath.Join(dir, partyName))
	}
	if data, err := os.ReadFile(got); err != nil || string(data) != `{"cloudflare_url":"wss://example.test"}` {
		t.Fatalf("migrated config = %q, %v", data, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy config still exists: %v", err)
	}
}

func TestMigratePartyConfigKeepsCurrentFile(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, partyName)
	if err := os.WriteFile(current, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyPartyName), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := migratePartyConfig(dir)
	if got != current {
		t.Fatalf("config path = %q, want %q", got, current)
	}
	if data, err := os.ReadFile(current); err != nil || string(data) != "current" {
		t.Fatalf("current config was overwritten: %q, %v", data, err)
	}
}

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
