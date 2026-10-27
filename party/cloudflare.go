package party

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

// dialCloudflare opens a room on the Worker deployed from party/deploy/cloudflare.
// The Worker answers with a welcome message that tells the caller whether a
// host is already present. It only learns the hashed room address.
func dialCloudflare(ctx context.Context, cfg Config, address string, role Role) (transport, error) {
	endpoint, err := cfg.CloudflareEndpoint(address, role)
	if err != nil {
		return nil, err
	}
	conn, resp, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": []string{"yoanime"}},
	})
	if err != nil {
		if resp != nil {
			return nil, handshakeError(resp.StatusCode)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrServerUnreachable
		}
		return nil, fmt.Errorf("не удалось подключиться к комнате: %w", err)
	}
	return &cloudflareTransport{wsTransport: newWSTransport(conn)}, nil
}

// handshakeError explains why the Worker refused the WebSocket upgrade. A 200
// means the address is alive but serves something else, which is what an
// undeployed or replaced Worker looks like.
func handshakeError(status int) error {
	switch status {
	case http.StatusBadRequest:
		return fmt.Errorf("сервер отклонил ключ комнаты")
	case http.StatusNotFound:
		return fmt.Errorf("комната не найдена")
	case http.StatusOK:
		return fmt.Errorf("по адресу реле работает не комната (ответ 200)")
	}
	return fmt.Errorf("сервер комнаты ответил %d", status)
}

type cloudflareTransport struct {
	*wsTransport
}

func (t *cloudflareTransport) run(ctx context.Context, deliver func(frame)) error {
	for {
		data, err := t.read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return closeError(err)
		}
		var msg frame
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == messageError && msg.Error == "" {
			msg.Error = "ошибка комнаты"
		}
		deliver(msg)
	}
}
