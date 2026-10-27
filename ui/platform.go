package ui

import (
	"image"

	"gioui.org/io/event"

	"github.com/arfush/yo/catalog"
	platformwindow "github.com/arfush/yo/platform/window"
)

// The Win32 window primitives live in platform/window. These wrappers keep the
// screen code short and make the non-Windows build fall back to the same no-op
// API.
type windowRect = platformwindow.Rect

func handleViewEvent(e event.Event) { platformwindow.HandleViewEvent(e) }

func hoverPollingSupported() bool { return platformwindow.HoverPollingSupported() }

func mouseLeftPress() (down, pressed bool) { return platformwindow.MouseLeftPress() }

func cursorInRect(r image.Rectangle) bool { return platformwindow.CursorInRect(r) }

func cursorInWindow() bool { return platformwindow.CursorInWindow() }

func appWindowBounds(scale float64) (catalog.Bounds, bool) {
	return platformwindow.AppWindowBounds(scale)
}

func appWindowRect() (windowRect, bool) { return platformwindow.AppWindowRect() }

func rememberAppWindowRect(rect windowRect) { platformwindow.Remember(rect) }

func hideAppWindow() { platformwindow.Hide() }

func showAppWindow() { platformwindow.Show() }
