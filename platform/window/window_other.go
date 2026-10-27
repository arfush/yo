//go:build !windows

package window

import (
	"image"

	"gioui.org/io/event"

	"github.com/arfush/yoanime/catalog"
)

func HandleViewEvent(event.Event) {}

func HoverPollingSupported() bool { return false }

// MouseLeftPress is only needed for the Windows non-client header, which other
// platforms do not use.
func MouseLeftPress() (down, pressed bool) { return false, false }

func CursorInRect(image.Rectangle) bool { return true }

func CursorInWindow() bool { return true }

func AppWindowBounds(float64) (catalog.Bounds, bool) {
	return catalog.Bounds{}, false
}

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func AppWindowRect() (Rect, bool) { return Rect{}, false }

func Remember(Rect) {}

func Hide() {}

func Show() {}
