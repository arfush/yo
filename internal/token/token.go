// Package token reads and writes the AnimeLIB access token.
//
// AnimeLIB stores an OAuth token object in its localStorage. Yonime keeps the
// same fields in %APPDATA%\Yonime\animelib_token.txt as JSON, which lets the
// expiration be checked before the main window opens. A plain-text token is
// still accepted for compatibility with hand-written files.
package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExpiryMargin makes a token that is about to expire count as expired, so the
// user is not sent through authorization in the middle of a session.
const ExpiryMargin = 2 * time.Minute

// ErrNotFound is returned by Load when the token file does not exist.
var ErrNotFound = errors.New("token file not found")

// Token mirrors the token object stored by the AnimeLIB web application.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
}

// Load reads and parses a token file.
func Load(path string) (Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	token, err := Parse(data)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return token, nil
}

// Parse accepts either Yonime's JSON format, the token object persisted by
// AnimeLIB (optionally wrapped in a "token" field), or a plain access token.
func Parse(data []byte) (Token, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return Token{}, errors.New("файл токена пустой")
	}
	if !strings.HasPrefix(trimmed, "{") {
		return Token{AccessToken: trimmed, TokenType: "Bearer"}, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &fields); err != nil {
		return Token{}, fmt.Errorf("некорректный JSON: %w", err)
	}
	if nested, ok := fields["token"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(nested, &inner); err == nil && len(inner) > 0 {
			fields = inner
		}
	}

	token := Token{
		AccessToken:  stringField(fields, "access_token"),
		RefreshToken: stringField(fields, "refresh_token"),
		ExpiresIn:    intField(fields, "expires_in"),
		Timestamp:    intField(fields, "timestamp"),
		TokenType:    stringField(fields, "token_type"),
	}
	if token.AccessToken == "" {
		return Token{}, errors.New("в файле нет access_token")
	}
	return token, nil
}

// Save writes the token atomically, creating the parent directory if needed.
func Save(path string, token Token) error {
	if strings.TrimSpace(token.AccessToken) == "" {
		return errors.New("нельзя сохранить пустой токен")
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".animelib-token-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

// ExpiresAt returns the earliest known expiration time. It is computed from
// the site's timestamp/expires_in pair and, when present, the JWT exp claim.
func (t Token) ExpiresAt() (time.Time, bool) {
	var (
		expiry time.Time
		found  bool
	)
	if t.Timestamp > 0 && t.ExpiresIn > 0 {
		expiry = time.UnixMilli(t.Timestamp + t.ExpiresIn*1000)
		found = true
	}
	if jwtExpiry, ok := jwtExpiration(t.AccessToken); ok && (!found || jwtExpiry.Before(expiry)) {
		expiry = jwtExpiry
		found = true
	}
	return expiry, found
}

// Expired reports whether the token is missing, expired or about to expire.
// Tokens without any expiration metadata are treated as valid: only the
// AnimeLIB API can reject them, and the user should not be asked to log in on
// every launch because of an unknown format.
func (t Token) Expired(now time.Time) bool {
	if strings.TrimSpace(t.AccessToken) == "" {
		return true
	}
	expiry, ok := t.ExpiresAt()
	if !ok {
		return false
	}
	return !now.Add(ExpiryMargin).Before(expiry)
}

func jwtExpiration(raw string) (time.Time, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, int64(claims.Exp*float64(time.Second))), true
}

func stringField(fields map[string]json.RawMessage, name string) string {
	raw, ok := fields[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func intField(fields map[string]json.RawMessage, name string) int64 {
	raw, ok := fields[name]
	if !ok {
		return 0
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		if value, err := number.Int64(); err == nil {
			return value
		}
		if value, err := number.Float64(); err == nil {
			return int64(value)
		}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value := json.Number(strings.TrimSpace(text))
		if number, err := value.Int64(); err == nil {
			return number
		}
	}
	return 0
}
