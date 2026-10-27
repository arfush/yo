// Package party keeps a room of yoanime clients on the same playback
// position. A room lives on a relay the user deploys themselves; this package
// only speaks the wire protocol and the drift correction math.
package party

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// KeyLength is the room key length shown to the user.
	KeyLength = 9
	// KeyPrefix opens every room key, so a pasted value is recognisable. The
	// prefix is fixed text, not part of the random alphabet below.
	KeyPrefix = "YO"
	// keyProviderIndex is the position of the relay letter. It follows the
	// prefix: YOC… is a Cloudflare room, YOS… a Supabase room.
	keyProviderIndex = len(KeyPrefix)
	// keySuffixLength covers the random part after the prefix and the relay.
	keySuffixLength = KeyLength - len(KeyPrefix) - 1

	// addressBytes is the part of the key hash the relay learns. 128 bits is
	// far more than the room key can ever fill, so two rooms practically never
	// share a Durable Object.
	addressBytes = 16
)

// keyAlphabet is Crockford base32: the digits plus the letters that survive a
// handwriting test. I, L, O and U are missing on purpose, so a key typed from a
// screenshot or read aloud on a call cannot be confused with a similar one.
// Thirty-two symbols divide the byte range evenly, which keeps generation
// unbiased without rejection sampling.
const keyAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ErrInvalidKey reports a value that is not a room key.

// NewKey returns a random room key for the given relay, such as YOC4K7Q2M.
func NewKey(provider Provider) (string, error) {
	code, ok := providerKeyCode(provider)
	if !ok {
		return "", fmt.Errorf("неизвестный провайдер %q", provider)
	}
	buf := make([]byte, keySuffixLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for index := range buf {
		buf[index] = keyAlphabet[buf[index]%byte(len(keyAlphabet))]
	}
	return KeyPrefix + string(code) + string(buf), nil
}

// Address derives the room address the relay sees. The relay stores and routes
// rooms by this hash, so the key itself never leaves the two machines that
// already share it and cannot leak from a relay log or a Durable Object name.
func Address(key string) (string, error) {
	normalized, err := NormalizeKey(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("yoanime/party/room\x00" + normalized))
	return hex.EncodeToString(sum[:addressBytes]), nil
}

// NormalizeKey trims a pasted key and returns its canonical upper-case form.
// Case, spaces, hyphens and the usual Crockford look-alikes do not matter:
// "yoc 4k7q-2m", "YOC4K7Q2M" and "Y0C4K7Q2M" all resolve to the same room.
func NormalizeKey(value string) (string, error) {
	key := canonicalKey(value)
	if !ValidKey(key) {
		return "", ErrInvalidKey
	}
	return key, nil
}

// canonicalKey folds a typed key into Crockford form without validating it.
func canonicalKey(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '-' {
			continue
		}
		if r > 0x7f || builder.Len() >= KeyLength {
			return ""
		}
		builder.WriteByte(byte(r))
	}
	key := []byte(builder.String())
	if len(key) != KeyLength {
		return builder.String()
	}
	for index := 1; index < len(key); index++ {
		key[index] = foldKeyChar(key[index])
	}
	// The second character is the fixed "YO": a typed zero there means the
	// same letter, because the prefix is not part of the random alphabet.
	if key[0] == 'Y' && key[1] == '0' {
		key[1] = 'O'
	}
	return string(key)
}

// foldKeyChar maps the characters Crockford treats as look-alikes.
func foldKeyChar(r byte) byte {
	switch r {
	case 'O':
		return '0'
	case 'I', 'L':
		return '1'
	case 'U':
		return 'V'
	}
	return r
}

func isKeyChar(r byte) bool {
	return strings.IndexByte(keyAlphabet, r) >= 0
}

// ValidKey reports whether key is already in canonical form.
func ValidKey(key string) bool {
	if len(key) != KeyLength || !strings.HasPrefix(key, KeyPrefix) {
		return false
	}
	if !validProviderKey(key[keyProviderIndex]) {
		return false
	}
	for index := keyProviderIndex + 1; index < len(key); index++ {
		if !isKeyChar(key[index]) {
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

// SanitizeKeyInput keeps only key characters and never exceeds the key length,
// which lets the join field stay in the canonical form while the user types.
// The fixed prefix is left alone so that a typed O does not turn into a zero
// under the caret.
func SanitizeKeyInput(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(value) {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			continue
		}
		if builder.Len() >= KeyLength {
			break
		}
		if builder.Len() < len(KeyPrefix) {
			builder.WriteByte(byte(r))
			continue
		}
		builder.WriteByte(foldKeyChar(byte(r)))
	}
	return builder.String()
}
