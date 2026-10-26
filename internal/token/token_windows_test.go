//go:build windows

package token

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveDoesNotLeaveTheAccessTokenInPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	const access = "secret-access-token"
	if err := Save(path, Token{AccessToken: access}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(access)) {
		t.Fatal("DPAPI container contains the plaintext access token")
	}
	if bytes.Contains(raw, []byte("access_token")) {
		t.Fatal("DPAPI container contains the JSON field names")
	}
}

func TestLoadMigratesPlaintextToDPAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	const access = "legacy-access-token"
	if err := os.WriteFile(path, []byte(`{"access_token":"`+access+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != access {
		t.Fatalf("access token = %q", got.AccessToken)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(access)) {
		t.Fatal("migrated token file still contains the plaintext token")
	}
}
