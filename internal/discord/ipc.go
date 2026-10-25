package discord

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
)

// Opcodes of the local Discord IPC protocol.
const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
	opPing      = 3
	opPong      = 4
)

// maxFrameBytes bounds one JSON frame. Discord only sends small replies, so a
// larger header means the stream is out of sync.
const maxFrameBytes = 1 << 20

// handshakeTimeout bounds the wait for Discord's READY reply. Commands written
// before it are ignored by the client, so the connection is not usable until
// the handshake round trip finishes.
const handshakeTimeout = 5 * time.Second

// ipcClient is one connection to the Discord client. Writes are serialized,
// because the worker publishes activities while the watcher answers pings.
type ipcClient struct {
	conn io.ReadWriteCloser
	r    *bufio.Reader
	mu   sync.Mutex
	once sync.Once
}

func newClient(conn io.ReadWriteCloser, appID string) (*ipcClient, error) {
	c := &ipcClient{conn: conn, r: bufio.NewReader(conn)}
	if err := c.write(opHandshake, map[string]any{"v": 1, "client_id": appID}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	// Discord drops commands that arrive before it has answered the handshake,
	// so block until READY. Without this wait the first SET_ACTIVITY of a
	// session is silently lost and the profile stays empty until it closes.
	if setter, ok := conn.(interface{ SetReadDeadline(time.Time) error }); ok {
		_ = setter.SetReadDeadline(time.Now().Add(handshakeTimeout))
		defer setter.SetReadDeadline(time.Time{})
	}
	op, payload, err := c.read()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("discord ipc: handshake: %w", err)
	}
	var ready struct {
		Evt string `json:"evt"`
	}
	if op != opFrame || json.Unmarshal(payload, &ready) != nil || ready.Evt != "READY" {
		_ = conn.Close()
		return nil, errors.New("discord ipc: unexpected handshake reply")
	}
	return c, nil
}

func (c *ipcClient) setActivity(activity map[string]any) error {
	return c.command("SET_ACTIVITY", map[string]any{
		"pid":      os.Getpid(),
		"activity": activity,
	})
}

func (c *ipcClient) clear() error {
	return c.command("SET_ACTIVITY", map[string]any{
		"pid":      os.Getpid(),
		"activity": nil,
	})
}

func (c *ipcClient) command(name string, args map[string]any) error {
	return c.write(opFrame, map[string]any{
		"cmd":   name,
		"nonce": strconv.FormatInt(time.Now().UnixNano(), 36),
		"args":  args,
	})
}

func (c *ipcClient) write(op int, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	frame := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(frame[0:4], uint32(op))
	binary.LittleEndian.PutUint32(frame[4:8], uint32(len(data)))
	copy(frame[8:], data)

	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.conn.Write(frame)
	return err
}

func (c *ipcClient) read() (int, []byte, error) {
	var header [8]byte
	if _, err := io.ReadFull(c.r, header[:]); err != nil {
		return 0, nil, err
	}
	op := int(binary.LittleEndian.Uint32(header[0:4]))
	size := int(binary.LittleEndian.Uint32(header[4:8]))
	if size < 0 || size > maxFrameBytes {
		return 0, nil, fmt.Errorf("discord ipc: frame of %d bytes", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(c.r, data); err != nil {
		return 0, nil, err
	}
	return op, data, nil
}

// watch drains Discord's replies. Without it a closed client would only be
// noticed on the next write, which may never come for a paused episode.
func (c *ipcClient) watch() error {
	for {
		op, payload, err := c.read()
		if err != nil {
			return err
		}
		switch op {
		case opPing:
			var echo any
			if len(payload) > 0 {
				echo = json.RawMessage(payload)
			}
			if err := c.write(opPong, echo); err != nil {
				return err
			}
		case opClose:
			return errors.New("discord ipc: client closed the pipe")
		}
	}
}

func (c *ipcClient) close() {
	c.once.Do(func() {
		_ = c.conn.Close()
	})
}
