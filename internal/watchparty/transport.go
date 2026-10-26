package watchparty

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// ProtocolVersion is the current watch-party wire format. Bump it whenever a
// change makes old and new clients unable to understand each other.
//
// Version 2 encrypts every room payload with a key derived from the room key
// and the optional secret, and routes rooms by a hash of the key. A 1.x client
// cannot read a 2.x frame and must leave instead of guessing.
const ProtocolVersion = "2.0"

const (
	messageHello   = "hello"
	messageState   = "state"
	messageControl = "control"
	messageCard    = "card"
	// messageClose is an intentional host exit: guests must not wait for a
	// reconnect. messageBye only reports that the socket disappeared, so it
	// starts the grace window instead of ending the room.
	messageClose   = "close"
	messageBye     = "bye"
	messageError   = "error"
	messageWelcome = "welcome"
)

// frame is the cleartext envelope shared by both relays. The relay needs the
// type to route hello, control and state, and the card so a newcomer learns
// whether a secret is required before the host answers. Everything that says
// what is being watched travels in Data, sealed with the room content key.
type frame struct {
	// Version is the protocol version of the sender. Peers with a different
	// version must not share playback: their frames may mean something else.
	Version string `json:"v,omitempty"`
	Type    string `json:"t"`
	// Role is only used by a guest hello; the relay strips its own copy.
	Role Role `json:"role,omitempty"`
	// Card is the room header, present in cleartext on purpose.
	Card *Card `json:"card,omitempty"`
	// Data is base64(nonce || AES-GCM ciphertext) of payload.
	Data string `json:"d,omitempty"`
	// Host is the relay's welcome answer: whether a host is in the room.
	Host *bool `json:"host,omitempty"`
	// Error is a relay-level refusal; it never contains room content.
	Error string `json:"error,omitempty"`
}

// payload is the sealed part of a frame. The outer type tells what to expect,
// so the same struct serves state and control frames.
type payload struct {
	// Name is the sender's nickname. The host sends it with every anchor, so a
	// waiting guest sees who it is waiting for.
	Name    string   `json:"name,omitempty"`
	State   *State   `json:"state,omitempty"`
	Control *Control `json:"control,omitempty"`
}

// transport is one live connection to a relay. A session replaces it when the
// socket drops.
type transport interface {
	write(frame) error
	// run reads until the connection fails or ctx is done.
	run(ctx context.Context, deliver func(frame)) error
	close()
}

var errNotConnected = errors.New("нет соединения с комнатой")

const (
	readLimit    = 64 << 10
	writeTimeout = 10 * time.Second
)

// wsTransport wraps a WebSocket with a write mutex, because Supabase heartbeats
// and room messages are written from different goroutines.
type wsTransport struct {
	conn *websocket.Conn

	mu sync.Mutex
}

func newWSTransport(conn *websocket.Conn) *wsTransport {
	conn.SetReadLimit(readLimit)
	return &wsTransport{conn: conn}
}

func (t *wsTransport) write(msg frame) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return t.writeRaw(data)
}

func (t *wsTransport) writeRaw(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return t.conn.Write(ctx, websocket.MessageText, data)
}

func (t *wsTransport) read(ctx context.Context) ([]byte, error) {
	_, data, err := t.conn.Read(ctx)
	return data, err
}

func (t *wsTransport) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.conn.Close(websocket.StatusNormalClosure, "")
}

// closeError converts a WebSocket failure into a message a person can read.
func closeError(err error) error {
	if err == nil {
		return errors.New("соединение с комнатой закрыто")
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	if status := websocket.CloseStatus(err); status != -1 {
		switch status {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
			return errors.New("соединение с комнатой закрыто")
		}
	}
	return errors.New("не удалось связаться с сервером комнаты")
}
