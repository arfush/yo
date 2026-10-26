package party

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	contentKeySize = 32
	saltSize       = 16
)

// Argon2id parameters. The cost is paid once when a room key and secret are
// combined: on joining, locking and rotating. It never runs on the anchor path,
// so a few hundred milliseconds here do not touch playback sync.
const (
	kdfTimeDefault    uint32 = 3
	kdfMemoryDefault  uint32 = 64 * 1024
	kdfThreadsDefault uint8  = 2
)

// They are variables instead of constants so tests can lower the cost; the
// defaults above are the ones a release uses.
var (
	kdfTime    = kdfTimeDefault
	kdfMemory  = kdfMemoryDefault
	kdfThreads = kdfThreadsDefault
)

// errCiphertext reports a payload that is too short or fails authentication.
var errCiphertext = errors.New("повреждённое сообщение комнаты")

// deriveContentKey turns the room key and the optional secret into the AES key
// that protects every room frame. The room key is included on purpose: a secret
// alone must not open a room, and the relay never sees either value.
func deriveContentKey(roomKey, secret string, salt []byte) []byte {
	return argon2.IDKey([]byte(roomKey+":"+secret), salt, kdfTime, kdfMemory, kdfThreads, contentKeySize)
}

// sealContent encrypts a payload and returns base64(nonce || ciphertext).
func sealContent(key, plaintext []byte) (string, error) {
	aead, err := contentAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// openContent reverses sealContent. A failure means the key is wrong or the
// relay altered the frame; both are treated the same.
func openContent(key []byte, encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errCiphertext
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errCiphertext
	}
	aead, err := contentAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize()+aead.Overhead() {
		return nil, errCiphertext
	}
	plaintext, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return nil, errCiphertext
	}
	return plaintext, nil
}

func contentAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != contentKeySize {
		return nil, fmt.Errorf("unexpected content key length %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
