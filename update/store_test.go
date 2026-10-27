package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updates.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if store.State().Available != nil {
		t.Fatal("new state should not have a cached release")
	}

	release := Release{
		Version:   "v1.4.0",
		Size:      1234,
		URL:       "https://github.com/arfush/yo/releases/download/v1.4.0/yo_v1.4.0_amd64.exe",
		SHA256:    testHash,
		AssetName: "yo_v1.4.0_amd64.exe",
	}
	checked := time.Now().UTC().Truncate(time.Second)
	if err := store.RecordCheck(&release, checked); err != nil {
		t.Fatalf("RecordCheck: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	state := reopened.State()
	if state.Available == nil || state.Available.Version != "v1.4.0" {
		t.Fatalf("available = %+v", state.Available)
	}
	if !state.LastCheck.Equal(checked) {
		t.Fatalf("last check = %v, want %v", state.LastCheck, checked)
	}

	if err := reopened.RecordCheck(nil, checked.Add(time.Hour)); err != nil {
		t.Fatalf("clear release: %v", err)
	}
	if reopened.State().Available != nil {
		t.Fatal("available release was not cleared")
	}
}

func TestStoreDropsReleaseWithUntrustedURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updates.json")
	data := []byte(`{
  "available": {
    "version": "v1.4.0",
    "size": 1234,
    "url": "https://files.catbox.moe/test.bin",
    "sha256": "` + testHash + `"
  }
}`)

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if store.State().Available != nil {
		t.Fatal("untrusted cached release was kept")
	}
}

func TestStoreMovesCorruptStateAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updates.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if store.State().Available != nil {
		t.Fatal("corrupt state should be empty")
	}
	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Fatalf("corrupt state was not preserved: %v", err)
	}
}
