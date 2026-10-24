// Package watchparty keeps a room of Yonime clients on the same playback
// position. A room lives on a relay the user deploys themselves; this package
// only speaks the wire protocol and the drift correction math.
package watchparty

import (
	"crypto/rand"
	"errors"
	"strings"
)

const (
	// KeyLength is the room key length shown to the user.
	KeyLength = 9
	// KeyPrefix opens every room key, so a pasted value is recognisable.
	KeyPrefix = "YO"
)

const keyAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// ErrInvalidKey reports a value that is not a room key.
var ErrInvalidKey = errors.New("некорректный ключ комнаты")

// NewKey returns a random room key such as YOFBQOPGZ.
func NewKey() (string, error) {
	buf := make([]byte, KeyLength-len(KeyPrefix))
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Rejection sampling keeps every letter equally likely; 256 is not a
	// multiple of 26, so a plain modulo would slightly favour A..F.
	for index, value := range buf {
		for value >= 26 {
			value %= 26
		}
		buf[index] = keyAlphabet[value]
	}
	return KeyPrefix + string(buf), nil
}

// NormalizeKey trims a pasted key and returns its canonical upper-case form.
// The keyboard layout and letter case do not matter to the user, so "yofbqopgz"
// and " YOFBQOPGZ " both resolve to the same room.
func NormalizeKey(value string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	if !ValidKey(key) {
		return "", ErrInvalidKey
	}
	return key, nil
}

// ValidKey reports whether key is already in canonical form.
func ValidKey(key string) bool {
	if len(key) != KeyLength || !strings.HasPrefix(key, KeyPrefix) {
		return false
	}
	for _, r := range key[len(KeyPrefix):] {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// NormalizeDisplay returns the upper-case form of a key for display. It does
// not validate, so a half-typed key still renders.
func NormalizeDisplay(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// SanitizeKeyInput keeps only Latin letters in upper case and never exceeds the
// key length, which lets the join field stay in the canonical form while the
// user types.
func SanitizeKeyInput(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(value) {
		if r < 'A' || r > 'Z' {
			continue
		}
		if builder.Len() >= KeyLength {
			break
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
