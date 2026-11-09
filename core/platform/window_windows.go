//go:build windows

package platform

import (
	"image/color"
	"log/slog"
	"sync/atomic"
	"time"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"golang.org/x/sys/windows"
)

const (
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	dwmwcpRound = 2

	vkLeftButton = 0x01

	// heldFrameInterval matches the mascot's animation clock.
	heldFrameInterval = time.Second / 60
)

var (
	windowHandle atomic.Uintptr

	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procIsWindow         = user32.NewProc("IsWindow")

	repaintCancel chan struct{}
)

// HandleViewEvent applies the application palette to the native window frame.
// Gio reports the HWND once per window; a recreated window reports it again.
// Events other than a valid Win32ViewEvent are ignored. logger records the new
// handle and DWM attribute failures; the caller passes the platform logger.
func HandleViewEvent(e event.Event, border color.NRGBA, logger *slog.Logger) {
	view, ok := e.(app.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}
	hwnd := windows.HWND(view.HWND)
	if previous := windowHandle.Swap(view.HWND); previous != view.HWND {
		logger.Debug("window handle ready", "hwnd", view.HWND)
		startHeldRepaint(hwnd, logger)
	}
	setIntAttribute(hwnd, dwmwaUseImmersiveDarkMode, 1, logger)
	setIntAttribute(hwnd, dwmwaWindowCornerPreference, dwmwcpRound, logger)
	setColorAttribute(hwnd, dwmwaBorderColor, border, logger)
}

// startHeldRepaint keeps frames coming while the left mouse button is down.
// Dragging by the custom header enters a Win32 modal move loop, and Gio's run
// loop stops drawing until that loop returns. InvalidateRect makes the modal
// loop dispatch WM_PAINT, which Gio turns into a frame, so the mascot keeps
// animating while the window is moved or held still.
func startHeldRepaint(hwnd windows.HWND, logger *slog.Logger) {
	if repaintCancel != nil {
		close(repaintCancel)
	}
	cancel := make(chan struct{})
	repaintCancel = cancel
	go repaintWhileHeld(hwnd, cancel, logger)
}

func repaintWhileHeld(hwnd windows.HWND, cancel <-chan struct{}, logger *slog.Logger) {
	ticker := time.NewTicker(heldFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-cancel:
			logger.Debug("held repaint stopped", "reason", "cancelled")
			return
		case <-ticker.C:
		}
		if alive, _, _ := procIsWindow.Call(uintptr(hwnd)); alive == 0 {
			logger.Debug("held repaint stopped", "reason", "window gone")
			return
		}
		if down, _, _ := procGetAsyncKeyState.Call(vkLeftButton); down&0x8000 != 0 {
			_, _, _ = procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
	}
}

func setIntAttribute(hwnd windows.HWND, attribute uint32, value int32, logger *slog.Logger) {
	err := windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)),
	)
	if err != nil {
		logger.Warn("window frame attribute failed", "attribute", attribute, "error", err)
	}
}

func setColorAttribute(hwnd windows.HWND, attribute uint32, value color.NRGBA, logger *slog.Logger) {
	// COLORREF is 0x00BBGGRR, not the NRGBA byte order.
	colorref := uint32(value.R) | uint32(value.G)<<8 | uint32(value.B)<<16
	err := windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&colorref), uint32(unsafe.Sizeof(colorref)),
	)
	if err != nil {
		logger.Warn("window frame attribute failed", "attribute", attribute, "error", err)
	}
}
