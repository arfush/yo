//go:build !windows

package ui

import (
	"image"

	"gioui.org/io/event"

	"github.com/arfush/yonime/internal/browser"
)

func handleViewEvent(event.Event) {}

func hoverPollingSupported() bool { return false }

// mouseLeftPress is only needed for the Windows non-client header, which other
// platforms do not use.
func mouseLeftPress() (down, pressed bool) { return false, false }

func cursorInRect(image.Rectangle) bool { return true }

func cursorInWindow() bool { return true }

func appWindowBounds(float64) (browser.WindowBounds, bool) {
	return browser.WindowBounds{}, false
}

type windowRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func appWindowRect() (windowRect, bool) { return windowRect{}, false }

func hideAppWindow() {}

func showAppWindow() {}
