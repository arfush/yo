// Package watchparty keeps a room of Yonime clients on the same playback
// position. A room lives on a relay the user deploys themselves; this package
// only speaks the wire protocol and the drift correction math.
package watchparty

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

const (
	// KeyLength is the room key length shown to the user.
	KeyLength = 9
	// KeyPrefix opens every room key, so a pasted value is recognisable.
	KeyPrefix = "YO"
	// keyProviderIndex is the position of the relay letter. It follows the
	// prefix: YOC… is a Cloudflare room, YOS… a Supabase room.
	keyProviderIndex = len(KeyPrefix)
	// keySuffixLength covers the random part after the prefix and the relay.
	keySuffixLength = KeyLength - len(KeyPrefix) - 1
)

const keyAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// ErrInvalidKey reports a value that is not a room key.
var ErrInvalidKey = errors.New("некорректный ключ комнаты")

// NewKey returns a random room key for the given relay, such as YOCBQOPGZ.
func NewKey(provider Provider) (string, error) {
	code, ok := providerKeyCode(provider)
	if !ok {
		return "", fmt.Errorf("неизвестный провайдер %q", provider)
	}
	buf := make([]byte, keySuffixLength)
	for index := range buf {
		for {
			if _, err := rand.Read(buf[index : index+1]); err != nil {
				return "", err
			}
			// 234 is the largest multiple of 26 that fits in a byte. Values
			// above it are redrawn so every letter stays equally likely;
			// folding the whole byte range with a modulo would favour A..V.
			if buf[index] < 234 {
				buf[index] = keyAlphabet[buf[index]%26]
				break
			}
		}
	}
	return KeyPrefix + string(code) + string(buf), nil
}

// NormalizeKey trims a pasted key and returns its canonical upper-case form.
// The keyboard layout and letter case do not matter to the user, so "yocbqopgz"
// and " YOCBQOPGZ " both resolve to the same room.
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
	if !validProviderKey(key[keyProviderIndex]) {
		return false
	}
	for _, r := range key[keyProviderIndex+1:] {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ProviderFromKey returns the relay encoded in a room key. The guest must join
// the relay of the host, regardless of the provider selected in its settings.
func ProviderFromKey(value string) (Provider, error) {
	key, err := NormalizeKey(value)
	if err != nil {
		return "", err
	}
	switch key[keyProviderIndex] {
	case 'C':
		return Cloudflare, nil
	case 'S':
		return Supabase, nil
	}
	return "", ErrInvalidKey
}

// providerKeyCode is the third letter of a key for the given relay.
func providerKeyCode(provider Provider) (byte, bool) {
	switch provider {
	case Cloudflare:
		return 'C', true
	case Supabase:
		return 'S', true
	}
	return 0, false
}

func validProviderKey(code byte) bool {
	switch code {
	case 'C', 'S':
		return true
	}
	return false
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
