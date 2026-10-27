package party

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// These are filled by the release build through -ldflags so a fork can ship a
// ready relay. They stay empty in a normal build and the user supplies the
// endpoints in party.json instead.
var (
	CloudflareURL = ""
	SupabaseURL   = ""
	SupabaseKey   = ""
)

// Config is the relay configuration stored next to the other yoanime files.
type Config struct {
	CloudflareURL string `json:"cloudflare_url"`
	SupabaseURL   string `json:"supabase_url"`
	SupabaseKey   string `json:"supabase_key"`
}

// LoadConfig reads the party configuration. A missing file is not an
// error: the build-time defaults (if any) are used instead. When the file
// exists, it fully describes the relays, including empty fields: that is how
// the settings screen turns a built-in relay off.
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
	cfg.CloudflareURL = stored.CloudflareURL
	cfg.SupabaseURL = stored.SupabaseURL
	cfg.SupabaseKey = stored.SupabaseKey
	return cfg.normalized()
}

// SaveConfig writes the relays to the per-user override file. The write goes
// through a temporary file so a crash cannot leave a half-written JSON behind.
func SaveConfig(path string, cfg Config) error {
	cfg = cfg.normalized()
	if err := cfg.Validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode party config: %w", err)
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create party config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".party-*.json")
	if err != nil {
		return fmt.Errorf("create party config: %w", err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return fmt.Errorf("write party config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("close party config: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("replace party config: %w", err)
	}
	return nil
}

// Normalized returns the config in the form the clients dial.
func (c Config) Normalized() Config {
	return c.normalized()
}

// Validate checks the non-empty fields before they are saved. Partial configs
// are allowed: a provider simply stays unconfigured until both parts arrive.
func (c Config) Validate() error {
	c = c.normalized()
	for _, endpoint := range []struct {
		value string
		label string
	}{
		{c.CloudflareURL, "Cloudflare"},
		{c.SupabaseURL, "Supabase"},
	} {
		if endpoint.value == "" {
			continue
		}
		parsed, err := url.Parse(endpoint.value)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("invalid %s address", endpoint.label)
		}
		switch parsed.Scheme {
		case "ws", "wss":
		default:
			return fmt.Errorf("invalid %s address", endpoint.label)
		}
	}
	if key := strings.ToLower(c.SupabaseKey); strings.HasPrefix(key, "sb_secret_") || strings.HasPrefix(key, "service_role") {
		return ErrSupabaseKey
	}
	return nil
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
// The sentinel itself lives in errors.go.

// CloudflareEndpoint is the WebSocket URL of one room. The room parameter is
// the hashed address, not the key the user shared.
func (c Config) CloudflareEndpoint(address string, role Role) (string, error) {
	if c.CloudflareURL == "" {
		return "", ErrNotConfigured
	}
	parsed, err := url.Parse(c.CloudflareURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid Cloudflare address")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/room"
	query := parsed.Query()
	query.Set("room", address)
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
		return "", fmt.Errorf("invalid Supabase address")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/realtime/v1/websocket"
	query := parsed.Query()
	query.Set("apikey", c.SupabaseKey)
	query.Set("vsn", "1.0.0")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
