// Package assets owns the embedded resources used by the desktop UI.
//
// Fonts, the application image and IconVG glyphs live in separate
// subdirectories so it is clear what is a typeface, a bitmap and a vector
// icon. IconVG data is parsed lazily through gioui.org/widget.
package assets

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"sync"

	"gioui.org/widget"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

//go:embed images/app-icon-48.png
var appIconPNG []byte

//go:embed icons/*.iconvg
var iconFS embed.FS

// ReadFont returns the raw TTF for name, e.g. "Inter-Regular.ttf".
func ReadFont(name string) ([]byte, error) {
	data, err := fontFS.ReadFile("fonts/" + name)
	if err != nil {
		return nil, fmt.Errorf("read font %s: %w", name, err)
	}
	return data, nil
}

// AppIcon decodes the 48px header icon. The header draws it at 24dp, so the
// asset covers 2x DPI natively and still downsamples cleanly at 1x.
func AppIcon() (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		return nil, fmt.Errorf("decode app icon: %w", err)
	}
	return img, nil
}

// IconNames lists the embedded IconVG files. It is used by tests to ensure a
// missing asset is caught before a screen silently renders no glyph.
var IconNames = []string{
	"chevron_down",
	"chevron_down_heavy",
	"chevron_down_thick",
	"chevron_down_thin",
	"chevron_left",
	"chevron_left_heavy",
	"chevron_left_thick",
	"chevron_left_thin",
	"chevron_right",
	"chevron_right_heavy",
	"chevron_right_thick",
	"chevron_right_thin",
	"chevron_up",
	"chevron_up_heavy",
	"chevron_up_thick",
	"chevron_up_thin",
	"close",
	"copy",
	"download",
	"log",
	"maximize",
	"minimize",
	"people",
	"play",
	"restore",
	"settings",
	"trash",
	"undo",
}

var (
	iconsMu sync.Mutex
	icons   = make(map[string]*widget.Icon, len(IconNames))
)

// Icon returns the named IconVG glyph. A missing or malformed asset returns
// nil; the UI then leaves that glyph blank instead of panicking on a paint
// path. Tests cover the embedded set.
func Icon(name string) *widget.Icon {
	iconsMu.Lock()
	defer iconsMu.Unlock()
	if icon, ok := icons[name]; ok {
		return icon
	}
	data, err := iconFS.ReadFile("icons/" + name + ".iconvg")
	if err != nil {
		return nil
	}
	icon, err := widget.NewIcon(data)
	if err != nil {
		return nil
	}
	icons[name] = icon
	return icon
}
