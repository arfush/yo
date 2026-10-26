package token

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParsePlainToken(t *testing.T) {
	value, err := Parse([]byte("  abc.def.ghi\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if value.AccessToken != "abc.def.ghi" {
		t.Fatalf("access token = %q", value.AccessToken)
	}
	if value.TokenType != "Bearer" {
		t.Fatalf("token type = %q", value.TokenType)
	}
}

func TestParseAnimeLIBStore(t *testing.T) {
	raw := `{"token":{"access_token":"access","refresh_token":"refresh","expires_in":3600,"timestamp":1700000000000,"token_type":"Bearer"},"auth":true}`
	value, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if value.AccessToken != "access" || value.RefreshToken != "refresh" {
		t.Fatalf("unexpected token: %+v", value)
	}
	if value.ExpiresIn != 3600 || value.Timestamp != 1700000000000 {
		t.Fatalf("unexpected metadata: %+v", value)
	}
}

func TestParseRejectsIncompleteData(t *testing.T) {
	for _, raw := range []string{"", "   ", `{}`, `{"refresh_token":"x"}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("Parse(%q) succeeded", raw)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	want := Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
		Timestamp:    1700000000000,
		TokenType:    "Bearer",
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSaveWritesTokenContainer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	if err := Save(path, Token{AccessToken: "access"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte(tokenMagic)) {
		t.Fatalf("token file has no container marker: %q", raw[:min(len(raw), 32)])
	}
}

func TestLoadMigratesPlaintextToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	plain := []byte(`{"access_token":"access","token_type":"Bearer"}`)
	if err := os.WriteFile(path, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "access" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte(tokenMagic)) {
		t.Fatalf("plaintext token was not migrated: %q", raw[:min(len(raw), 32)])
	}
	if bytes.Equal(raw, plain) {
		t.Fatal("token file was not rewritten")
	}
}

func TestLoadRejectsDamagedProtectedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(path, append([]byte(tokenMagic), []byte("garbage")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("damaged protected token was accepted")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.txt"))
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestExpiredUsesSiteTimestamp(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	valid := Token{AccessToken: "a", ExpiresIn: 3600, Timestamp: now.UnixMilli()}
	if valid.Expired(now) {
		t.Fatal("fresh token reported as expired")
	}
	expired := Token{AccessToken: "a", ExpiresIn: 3600, Timestamp: now.Add(-time.Hour).UnixMilli()}
	if !expired.Expired(now) {
		t.Fatal("old token reported as valid")
	}
	// A token that expires inside the safety margin is treated as expired.
	near := Token{AccessToken: "a", ExpiresIn: 60, Timestamp: now.UnixMilli()}
	if !near.Expired(now) {
		t.Fatal("token expiring within the margin reported as valid")
	}
}

func TestExpiredUsesJWTExp(t *testing.T) {
	now := time.Unix(1700000000, 0)
	jwt := func(exp float64) string {
		payload, err := json.Marshal(map[string]any{"exp": exp})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	}

	if (Token{AccessToken: jwt(float64(now.Add(time.Hour).Unix()))}).Expired(now) {
		t.Fatal("fresh JWT reported as expired")
	}
	if !(Token{AccessToken: jwt(float64(now.Add(-time.Minute).Unix()))}).Expired(now) {
		t.Fatal("expired JWT reported as valid")
	}
}

func TestTokenWithoutExpirationIsAccepted(t *testing.T) {
	if (Token{AccessToken: "opaque"}).Expired(time.Now()) {
		t.Fatal("opaque token should not be treated as expired")
	}
	if !(Token{}).Expired(time.Now()) {
		t.Fatal("empty token should be treated as expired")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "token.txt")
	if err := Save(path, Token{AccessToken: "a"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.IsDir() {
		t.Fatal("token path is a directory")
	}
}
