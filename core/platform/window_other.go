//go:build !windows

package platform

import (
	"image/color"
	"log/slog"

	"gioui.org/io/event"
)

// HandleViewEvent is a no-op where the native frame is not managed by DWM.
func HandleViewEvent(event.Event, color.NRGBA, *slog.Logger) {}
