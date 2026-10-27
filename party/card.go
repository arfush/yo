package party

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// cardCheck is the fixed plaintext sealed into every card. A guest that derives
// a key from the wrong secret cannot open it, which gives an instant "wrong
// secret" answer without asking the host. The plaintext is public knowledge on
// purpose: knowing it does not help anyone produce a valid tag.
const cardCheck = "yoanime/party/card"

// ErrWrongSecret reports a secret that does not open the room.
var ErrWrongSecret = errors.New("неверный секрет комнаты")

// ErrNoCard reports a guest that has not heard the host card yet.
var ErrNoCard = errors.New("комната ещё не ответила")

// Card is the cleartext room header. The relay stores and replays it, so a
// guest learns whether a secret is needed before the host answers, and can
// verify a typed secret locally. It carries no playback data: locked, the
// version counter, the KDF salt and one sealed constant.
type Card struct {
	Locked bool   `json:"locked"`
	Epoch  int    `json:"epoch"`
	Salt   string `json:"salt"`
	Check  string `json:"check"`
}

// newCard creates a fresh card and its content key. Every lock, unlock and
// secret rotation generates a new salt, so the previous key cannot be reused
// even when the secret happens to repeat.
func newCard(roomKey, secret string, locked bool, epoch int) (Card, []byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return Card{}, nil, err
	}
	key := deriveContentKey(roomKey, secret, salt)
	check, err := sealContent(key, []byte(cardCheck))
	if err != nil {
		return Card{}, nil, err
	}
	return Card{
		Locked: locked,
		Epoch:  epoch,
		Salt:   base64.StdEncoding.EncodeToString(salt),
		Check:  check,
	}, key, nil
}

// Valid reports whether the card has the parts a key derivation needs.
func (c Card) Valid() bool {
	if c.Epoch <= 0 || c.Check == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(c.Salt)
	return err == nil && len(salt) == saltSize
}

// Key derives the content key and reports whether the secret fits the card.
// An empty secret is the public-room case; the check still has to open.
func (c Card) Key(roomKey, secret string) ([]byte, bool) {
	salt, err := base64.StdEncoding.DecodeString(c.Salt)
	if err != nil || len(salt) != saltSize {
		return nil, false
	}
	key := deriveContentKey(roomKey, secret, salt)
	if _, err := openContent(key, c.Check); err != nil {
		return nil, false
	}
	return key, true
}
