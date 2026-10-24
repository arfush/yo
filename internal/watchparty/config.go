package watchparty

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// These are filled by the release build through -ldflags so a fork can ship a
// ready relay. They stay empty in a normal build and the user supplies the
// endpoints in watchparty.json instead.
var (
	CloudflareURL = ""
	SupabaseURL   = ""
	SupabaseKey   = ""
)

// Config is the relay configuration stored next to the other Yonime files.
type Config struct {
	CloudflareURL string `json:"cloudflare_url"`
	SupabaseURL   string `json:"supabase_url"`
	SupabaseKey   string `json:"supabase_key"`
}

// LoadConfig reads the watch-party configuration. A missing file is not an
// error: the build-time defaults (if any) are used instead.
func LoadConfig(path string) Config {
	cfg := Config{
		CloudflareURL: CloudflareURL,
		SupabaseURL:   SupabaseURL,
		SupabaseKey:   SupabaseKey,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg.normalized()
	}
	var stored Config
	if json.Unmarshal(raw, &stored) != nil {
		return cfg.normalized()
	}
	if value := strings.TrimSpace(stored.CloudflareURL); value != "" {
		cfg.CloudflareURL = value
	}
	if value := strings.TrimSpace(stored.SupabaseURL); value != "" {
		cfg.SupabaseURL = value
	}
	if value := strings.TrimSpace(stored.SupabaseKey); value != "" {
		cfg.SupabaseKey = value
	}
	return cfg.normalized()
}

func (c Config) normalized() Config {
	c.CloudflareURL = strings.TrimRight(strings.TrimSpace(c.CloudflareURL), "/")
	c.SupabaseURL = strings.TrimRight(strings.TrimSpace(c.SupabaseURL), "/")
	c.SupabaseKey = strings.TrimSpace(c.SupabaseKey)
	// Accept the https:// form shown in the provider dashboards; the client
	// always speaks WebSocket.
	c.CloudflareURL = toWebSocket(c.CloudflareURL)
	c.SupabaseURL = toWebSocket(c.SupabaseURL)
	return c
}

func toWebSocket(value string) string {
	switch {
	case strings.HasPrefix(value, "https://"):
		return "wss://" + strings.TrimPrefix(value, "https://")
	case strings.HasPrefix(value, "http://"):
		return "ws://" + strings.TrimPrefix(value, "http://")
	}
	return value
}

// Configured reports whether the provider has enough settings to dial.
func (c Config) Configured(p Provider) bool {
	switch p {
	case Cloudflare:
		return c.CloudflareURL != ""
	case Supabase:
		return c.SupabaseURL != "" && c.SupabaseKey != ""
	}
	return false
}

// ErrNotConfigured is returned when the selected provider has no endpoint.
var ErrNotConfigured = errors.New("провайдер не настроен")

// CloudflareEndpoint is the WebSocket URL of one room.
func (c Config) CloudflareEndpoint(key string, role Role) (string, error) {
	if c.CloudflareURL == "" {
		return "", ErrNotConfigured
	}
	parsed, err := url.Parse(c.CloudflareURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("адрес Cloudflare указан неверно")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/room"
	query := parsed.Query()
	query.Set("key", key)
	query.Set("role", string(role))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// SupabaseEndpoint is the Realtime WebSocket URL of the project.
func (c Config) SupabaseEndpoint() (string, error) {
	if c.SupabaseURL == "" || c.SupabaseKey == "" {
		return "", ErrNotConfigured
	}
	parsed, err := url.Parse(c.SupabaseURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("адрес Supabase указан неверно")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/realtime/v1/websocket"
	query := parsed.Query()
	query.Set("apikey", c.SupabaseKey)
	query.Set("vsn", "1.0.0")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
