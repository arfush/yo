package playback

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arfush/yonime/internal/appdata"
)

// The header is shipped as a Lua script and copied to the per-user mpv
// directory at playback start. Keeping the source in the repository lets the
// script evolve with the rest of the UI while still satisfying mpv's need for
// a real file on disk.
//
//go:embed yonime-header.lua
var headerScript []byte

const headerScriptName = "yonime-header.lua"

// headerAssets are the files mpv needs for the custom window header.
type headerAssets struct {
	ScriptPath string
	FontsDir   string
}

// installHeaderAssets writes the Lua header and the Inter faces used by its
// OSD text into %APPDATA%\yonime\mpv. Files are only rewritten when their
// contents change, so normal playback does not touch the disk beyond checking
// the installed copy.
func installHeaderAssets(fonts map[string][]byte) (headerAssets, error) {
	dir, err := appdata.MPVDir()
	if err != nil {
		return headerAssets{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return headerAssets{}, fmt.Errorf("создать каталог mpv %s: %w", dir, err)
	}

	scriptPath := filepath.Join(dir, headerScriptName)
	if err := writeFileIfChanged(scriptPath, headerScript); err != nil {
		return headerAssets{}, fmt.Errorf("установить заголовок mpv: %w", err)
	}
	assets := headerAssets{ScriptPath: scriptPath}
	if len(fonts) == 0 {
		return assets, nil
	}

	fontsDir := filepath.Join(dir, "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		return headerAssets{}, fmt.Errorf("создать каталог шрифтов mpv %s: %w", fontsDir, err)
	}
	for name, data := range fonts {
		base := filepath.Base(strings.TrimSpace(name))
		if base == "" || base == "." || base != name {
			return headerAssets{}, fmt.Errorf("некорректное имя шрифта %q", name)
		}
		if err := writeFileIfChanged(filepath.Join(fontsDir, base), data); err != nil {
			return headerAssets{}, fmt.Errorf("установить шрифт mpv %s: %w", base, err)
		}
	}
	assets.FontsDir = fontsDir
	return assets, nil
}

// writeFileIfChanged avoids rewriting large font files on every playback.
func writeFileIfChanged(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}
