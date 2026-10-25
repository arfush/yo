package playback

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arfush/yonime/internal/appdata"
)

// The guest input section is shipped as an mpv input.conf. It is copied to the
// per-user mpv directory at playback start so a watch-party guest whose host
// keeps control can still use the player without seeking or pausing.
//
//go:embed yonime-input.conf
var guestInputConf []byte

const guestInputConfName = "yonime-input.conf"

// guestInputSection is enabled only while a watch-party guest cannot control
// the timeline. Keeping the section in a real input.conf lets mpv keep every
// other binding untouched.
const guestInputSection = "yonime-no-timeline"

// installInputConf writes the guest input bindings into %APPDATA%\yonime\mpv
// and returns their path. The file is only rewritten when its contents change.
func installInputConf() (string, error) {
	dir, err := appdata.MPVDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("создать каталог mpv %s: %w", dir, err)
	}
	path := filepath.Join(dir, guestInputConfName)
	if err := writeFileIfChanged(path, guestInputConf); err != nil {
		return "", fmt.Errorf("установить настройки ввода mpv: %w", err)
	}
	// Older builds drew their own header from a Lua script in this directory.
	// Playback no longer loads it; drop the stale copy so it cannot be picked
	// up by a manually started mpv from the same config directory.
	_ = os.Remove(filepath.Join(dir, "yonime-header.lua"))
	return path, nil
}

// writeFileIfChanged avoids rewriting the file on every playback.
func writeFileIfChanged(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}
