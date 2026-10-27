package filmlib

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"

	"github.com/arfush/yo/platform/appdata"
)

// Default mirror hosts. Only the wrapper origin is hardcoded: the players
// endpoint is derived from it (p.<host>) and the theatre host is discovered
// from the players response on every load. That way a theatre move needs no
// application update at all, and a players move needs either a redirect on the
// wrapper or an edited filmlib.json.
const (
	defaultWrapper = "https://linkpp.ink/"
	defaultPlayers = "https://p.linkpp.ink/api/players"
)

// wrapperCandidates are tried in order when the players endpoint cannot be
// reached. The mirror has historically moved between domains while keeping the
// same paths, so a different zone is usually enough.
var wrapperCandidates = []string{
	"https://linkpp.ink/",
	"https://linkpp.cx/",
	"https://linkpp.ru/",
}

// Config overrides the mirror hosts. The file is optional: users only need it
// when a mirror moves before the application ships an update. Fields left empty
// keep their defaults.
type Config struct {
	Wrapper string `json:"wrapper"`
	Players string `json:"players"`
	Theatre string `json:"theatre"`
}

func (c Config) withDefaults() Config {
	out := c
	out.Wrapper = normalizeHost(out.Wrapper, defaultWrapper, true)
	if strings.TrimSpace(out.Players) == "" {
		out.Players = normalizeHost(playersFromWrapper(out.Wrapper), defaultPlayers, false)
	} else if parsed, err := url.Parse(strings.TrimSpace(out.Players)); err == nil && parsed.Host != "" {
		out.Players = strings.TrimRight(strings.TrimSpace(out.Players), "/")
	} else {
		out.Players = defaultPlayers
	}
	if strings.TrimSpace(out.Theatre) == "" {
		out.Theatre = "https://theatre.stravers.live/"
	}
	return out
}

// LoadConfig reads the optional filmlib.json from the data directory. A
// missing or broken file is not an error: the defaults still work.
func LoadConfig() Config {
	var config Config
	path, err := configPath()
	if err != nil {
		return config.withDefaults()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn("cannot read filmlib config", "error", err)
		}
		return config.withDefaults()
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		logger.Warn("cannot parse filmlib config", "error", err)
		return Config{}.withDefaults()
	}
	logger.Debug("filmlib config loaded", "wrapper", config.withDefaults().Wrapper)
	return config.withDefaults()
}

// configPath prefers filmlib.json and falls back to the old file name so an
// existing fork override keeps working after the package rename.
func configPath() (string, error) {
	path, err := appdata.FilmlibConfigPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	legacy, legacyErr := appdata.LegacyFilmlibConfigPath()
	if legacyErr != nil {
		return path, nil
	}
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}
	return path, nil
}

// playersFromWrapper derives the players endpoint from a wrapper origin, which
// is what the mirror does itself (linkpp.ink -> p.linkpp.ink).
func playersFromWrapper(wrapper string) string {
	parsed, err := url.Parse(strings.TrimSpace(wrapper))
	if err != nil || parsed.Host == "" {
		return defaultPlayers
	}
	return "https://p." + parsed.Hostname() + "/api/players"
}

// normalizeHost makes a configured host absolute and keeps its trailing slash.
func normalizeHost(raw, fallback string, slash bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fallback
	}
	out := parsed.String()
	if slash {
		out = strings.TrimRight(out, "/") + "/"
	} else {
		out = strings.TrimRight(out, "/")
	}
	return out
}

// referer returns the value the theatre page expects in the Referer header. Any
// value is accepted by the current mirror, but sending the wrapper keeps the
// request identical to the site's own traffic.
func (c Config) referer() string {
	if c.Wrapper == "" {
		return defaultWrapper
	}
	return c.Wrapper
}
