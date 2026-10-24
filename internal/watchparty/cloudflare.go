package watchparty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

// dialCloudflare opens a room on the Worker deployed from deploy/cloudflare.
// The Worker answers with a welcome message that tells the caller whether a
// host is already present.
func dialCloudflare(ctx context.Context, cfg Config, key string, role Role) (transport, error) {
	endpoint, err := cfg.CloudflareEndpoint(key, role)
	if err != nil {
		return nil, err
	}
	conn, resp, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": []string{"Yonime"}},
	})
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusBadRequest:
				return nil, fmt.Errorf("сервер отклонил ключ комнаты")
			case http.StatusNotFound:
				return nil, fmt.Errorf("комната не найдена")
			}
			return nil, fmt.Errorf("сервер комнаты ответил %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("не удалось подключиться к комнате")
	}
	return &cloudflareTransport{wsTransport: newWSTransport(conn)}, nil
}

type cloudflareTransport struct {
	*wsTransport
}

func (t *cloudflareTransport) run(ctx context.Context, deliver func(message)) error {
	for {
		data, err := t.read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return closeError(err)
		}
		var msg message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == messageError {
			if msg.Error == "" {
				msg.Error = "ошибка комнаты"
			}
			deliver(msg)
			continue
		}
		deliver(msg)
	}
}
