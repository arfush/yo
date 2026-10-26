package party

import (
	"crypto/rand"
	"errors"
	"strings"
)

// SecretLength is the length of the optional room secret. Ten symbols from the
// same thirty-two-character alphabet give about 50 bits, which is worthless to
// guess even for somebody who already has the room key: they would have to run
// one Argon2id derivation per candidate.
const SecretLength = 10

// ErrInvalidSecret reports a value that is not a room secret.
var ErrInvalidSecret = errors.New("некорректный секрет комнаты")

// NewSecret returns a random room secret, such as 4K7Q2M9XBT.
func NewSecret() (string, error) {
	buf := make([]byte, SecretLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for index := range buf {
		buf[index] = keyAlphabet[buf[index]%byte(len(keyAlphabet))]
	}
	return string(buf), nil
}

// NormalizeSecret trims and folds a typed secret. An empty value is valid and
// means "no secret"; every other value must have the full length.
func NormalizeSecret(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	secret, ok := foldSecret(value, SecretLength)
	if !ok || !ValidSecret(secret) {
		return "", ErrInvalidSecret
	}
	return secret, nil
}

// ValidSecret reports whether secret is in canonical form.
func ValidSecret(secret string) bool {
	if len(secret) != SecretLength {
		return false
	}
	for index := 0; index < len(secret); index++ {
		if !isKeyChar(secret[index]) {
			return false
		}
	}
	return true
}

// SanitizeSecretInput keeps only secret characters and never exceeds the
// length, so the field stays in the canonical form while the user types.
func SanitizeSecretInput(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '-' {
			continue
		}
		if r > 0x7f {
			continue
		}
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			continue
		}
		if builder.Len() >= SecretLength {
			break
		}
		builder.WriteByte(foldKeyChar(byte(r)))
	}
	return builder.String()
}

// foldSecret uppercases a typed value, drops the separators people add when
// reading a code aloud and folds Crockford look-alikes. It reports false when
// the value contains something that cannot belong to a secret or is too long.
func foldSecret(value string, limit int) (string, bool) {
	var builder strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '-' {
			continue
		}
		if r > 0x7f {
			return "", false
		}
		if builder.Len() >= limit {
			return "", false
		}
		builder.WriteByte(foldKeyChar(byte(r)))
	}
	return builder.String(), true
}

// FormatSecret groups a secret for reading aloud: 4K7Q2-M9XBT.
func FormatSecret(secret string) string {
	if len(secret) != SecretLength {
		return secret
	}
	return secret[:SecretLength/2] + "-" + secret[SecretLength/2:]
}
