//go:build !windows

package platform

import (
	"image/color"
	"log/slog"

	"gioui.org/io/event"
)

// Window is a no-op where the native frame is not managed by DWM.
type Window struct{}

// NewWindow returns a no-op window frame.
func NewWindow(*slog.Logger) *Window { return &Window{} }

// HandleViewEvent ignores every event.
func (*Window) HandleViewEvent(event.Event, color.NRGBA) {}

// SetIcon ignores the icon where the native frame is not managed by DWM.
func (*Window) SetIcon([]byte) {}

// Close does nothing.
func (*Window) Close() {}
