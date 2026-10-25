package watchparty

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// message is the wire envelope shared by both relays.
type message struct {
	Type    string   `json:"t"`
	Role    Role     `json:"role,omitempty"`
	Name    string   `json:"name,omitempty"`
	State   *State   `json:"state,omitempty"`
	Control *Control `json:"control,omitempty"`
	Host    *bool    `json:"host,omitempty"`
	Error   string   `json:"error,omitempty"`
}

const (
	messageHello   = "hello"
	messageState   = "state"
	messageControl = "control"
	messageBye     = "bye"
	messageError   = "error"
	messageWelcome = "welcome"
)

// transport is one live connection to a relay. A session replaces it when the
// socket drops.
type transport interface {
	write(message) error
	// run reads until the connection fails or ctx is done.
	run(ctx context.Context, deliver func(message)) error
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

func (t *wsTransport) write(msg message) error {
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
