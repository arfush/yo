package watchparty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Supabase Realtime speaks the Phoenix channels protocol. One room maps to one
// public broadcast channel, so no table, policy or edge function is needed: the
// anon key is enough to join and exchange broadcast events.
const (
	supabaseEventJoin      = "phx_join"
	supabaseEventReply     = "phx_reply"
	supabaseEventError     = "phx_error"
	supabaseEventClose     = "phx_close"
	supabaseEventHeartbeat = "heartbeat"
	supabaseEventBroadcast = "broadcast"
	supabaseBroadcastEvent = "msg"
	supabaseHeartbeatRate  = 25 * time.Second
	supabaseJoinTimeout    = 12 * time.Second
)

const supabaseTopicPrefix = "realtime:watchparty:"

// dialSupabase connects to the project's Realtime endpoint and joins the room
// channel. Joining happens before the transport is returned so no room message
// can be lost between connect and join.
func dialSupabase(ctx context.Context, cfg Config, key string, role Role, name string) (transport, error) {
	endpoint, err := cfg.SupabaseEndpoint()
	if err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, supabaseJoinTimeout)
	defer cancel()
	conn, resp, err := websocket.Dial(dialCtx, endpoint, nil)
	if err != nil {
		if resp != nil && resp.StatusCode >= 400 {
			return nil, fmt.Errorf("Supabase отклонил подключение (%d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("не удалось подключиться к Supabase")
	}
	t := &supabaseTransport{
		wsTransport: newWSTransport(conn),
		topic:       supabaseTopicPrefix + key,
		role:        role,
		name:        name,
	}
	if err := t.join(dialCtx); err != nil {
		t.close()
		return nil, err
	}
	return t, nil
}

type supabaseTransport struct {
	*wsTransport

	topic string
	role  Role
	name  string

	refMu sync.Mutex
	ref   int64
}

func (t *supabaseTransport) nextRef() string {
	t.refMu.Lock()
	defer t.refMu.Unlock()
	t.ref++
	return fmt.Sprintf("%d", t.ref)
}

func (t *supabaseTransport) join(ctx context.Context) error {
	ref := t.nextRef()
	frame := map[string]any{
		"topic": t.topic,
		"event": supabaseEventJoin,
		"payload": map[string]any{
			"config": map[string]any{
				"broadcast": map[string]any{"ack": false, "self": false},
				"presence":  map[string]any{"key": ""},
			},
		},
		"ref": ref,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := t.writeRaw(data); err != nil {
		return fmt.Errorf("не удалось войти в комнату Supabase")
	}
	for {
		raw, err := t.read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("Supabase не ответил на вход в комнату")
			}
			return closeError(err)
		}
		frame, ok := decodePhoenixFrame(raw)
		if !ok || frame.Event != supabaseEventReply || frame.Ref != ref {
			continue
		}
		if frame.Status != "ok" {
			return fmt.Errorf("Supabase отклонил комнату: %s", supabaseReason(frame.Payload))
		}
		return nil
	}
}

func supabaseReason(payload json.RawMessage) string {
	var value struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(payload, &value) == nil && value.Reason != "" {
		return value.Reason
	}
	return "неизвестная ошибка"
}

type phoenixFrame struct {
	Topic   string          `json:"topic"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
	Ref     string          `json:"ref"`
	Status  string          `json:"-"`
}

// decodePhoenixFrame also lifts payload.status into the frame. Phoenix nests
// the reply status inside the payload, which is awkward to express with tags.
func decodePhoenixFrame(raw []byte) (phoenixFrame, bool) {
	var probe struct {
		Topic   string          `json:"topic"`
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
		Ref     json.RawMessage `json:"ref"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return phoenixFrame{}, false
	}
	frame := phoenixFrame{
		Topic:   probe.Topic,
		Event:   probe.Event,
		Payload: probe.Payload,
	}
	ref := strings.Trim(string(probe.Ref), `"`)
	if ref != "null" {
		frame.Ref = ref
	}
	if probe.Event == supabaseEventReply && len(probe.Payload) > 0 {
		var reply struct {
			Status   string          `json:"status"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(probe.Payload, &reply) == nil {
			frame.Status = reply.Status
			if len(reply.Response) > 0 {
				frame.Payload = reply.Response
			}
		}
	}
	return frame, true
}

func (t *supabaseTransport) write(msg message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	frame := map[string]any{
		"topic": t.topic,
		"event": supabaseEventBroadcast,
		"payload": map[string]any{
			"type":    supabaseEventBroadcast,
			"event":   supabaseBroadcastEvent,
			"payload": json.RawMessage(payload),
		},
		"ref": t.nextRef(),
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return t.writeRaw(data)
}

func (t *supabaseTransport) run(ctx context.Context, deliver func(message)) error {
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go t.heartbeat(heartbeatCtx)

	for {
		raw, err := t.read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return closeError(err)
		}
		frame, ok := decodePhoenixFrame(raw)
		if !ok {
			continue
		}
		switch frame.Event {
		case supabaseEventBroadcast:
			var envelope struct {
				Event   string          `json:"event"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(frame.Payload, &envelope) != nil {
				continue
			}
			if envelope.Event != supabaseBroadcastEvent {
				continue
			}
			var msg message
			if json.Unmarshal(envelope.Payload, &msg) != nil {
				continue
			}
			deliver(msg)
		case supabaseEventError, supabaseEventClose:
			return errors.New("канал комнаты закрыт")
		case supabaseEventReply:
			if frame.Status != "" && frame.Status != "ok" {
				return errors.New("Supabase отклонил сообщение комнаты")
			}
		}
	}
}

func (t *supabaseTransport) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(supabaseHeartbeatRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frame := map[string]any{
				"topic":   "phoenix",
				"event":   supabaseEventHeartbeat,
				"payload": map[string]any{},
				"ref":     t.nextRef(),
			}
			data, err := json.Marshal(frame)
			if err != nil {
				continue
			}
			_ = t.writeRaw(data)
		}
	}
}
