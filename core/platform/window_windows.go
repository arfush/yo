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

// Window owns the native window frame and the background repaint loop that
// keeps frames coming while the left mouse button is held. Use NewWindow to
// build it and Close to stop the loop.
type Window struct {
	logger *slog.Logger
	handle atomic.Uintptr
	cancel chan struct{}

	procGetAsyncKeyState *windows.LazyProc
	procInvalidateRect   *windows.LazyProc
	procIsWindow         *windows.LazyProc
}

// NewWindow builds the native frame for one application window. A nil logger
// discards frame logs.
func NewWindow(logger *slog.Logger) *Window {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	return &Window{
		logger:               logger,
		procGetAsyncKeyState: user32.NewProc("GetAsyncKeyState"),
		procInvalidateRect:   user32.NewProc("InvalidateRect"),
		procIsWindow:         user32.NewProc("IsWindow"),
	}
}

// HandleViewEvent applies the application palette to the native window frame.
// Gio reports the HWND once per window; a recreated window reports it again.
// Events other than a valid Win32ViewEvent are ignored.
func (w *Window) HandleViewEvent(e event.Event, border color.NRGBA) {
	view, ok := e.(app.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}
	hwnd := windows.HWND(view.HWND)
	if previous := w.handle.Swap(view.HWND); previous != view.HWND {
		w.logger.Debug("window handle ready", "hwnd", view.HWND)
		w.startHeldRepaint(hwnd)
	}
	w.setIntAttribute(hwnd, dwmwaUseImmersiveDarkMode, 1)
	w.setIntAttribute(hwnd, dwmwaWindowCornerPreference, dwmwcpRound)
	w.setColorAttribute(hwnd, dwmwaBorderColor, border)
}

// Close stops the repaint loop. It is idempotent and runs on the UI goroutine.
func (w *Window) Close() {
	if w.cancel == nil {
		return
	}
	close(w.cancel)
	w.cancel = nil
}

// startHeldRepaint keeps frames coming while the left mouse button is down.
// Dragging by the custom header enters a Win32 modal move loop, and Gio's run
// loop stops drawing until that loop returns. InvalidateRect makes the modal
// loop dispatch WM_PAINT, which Gio turns into a frame, so the mascot keeps
// animating while the window is moved or held still.
func (w *Window) startHeldRepaint(hwnd windows.HWND) {
	w.Close()
	cancel := make(chan struct{})
	w.cancel = cancel
	go w.repaintWhileHeld(hwnd, cancel)
}

func (w *Window) repaintWhileHeld(hwnd windows.HWND, cancel <-chan struct{}) {
	ticker := time.NewTicker(heldFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-cancel:
			w.logger.Debug("held repaint stopped", "reason", "cancelled")
			return
		case <-ticker.C:
		}
		if alive, _, _ := w.procIsWindow.Call(uintptr(hwnd)); alive == 0 {
			w.logger.Debug("held repaint stopped", "reason", "window gone")
			return
		}
		if down, _, _ := w.procGetAsyncKeyState.Call(vkLeftButton); down&0x8000 != 0 {
			_, _, _ = w.procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
	}
}

func (w *Window) setIntAttribute(hwnd windows.HWND, attribute uint32, value int32) {
	err := windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)),
	)
	if err != nil {
		w.logger.Warn("window frame attribute failed", "attribute", attribute, "error", err)
	}
}

func (w *Window) setColorAttribute(hwnd windows.HWND, attribute uint32, value color.NRGBA) {
	// COLORREF is 0x00BBGGRR, not the NRGBA byte order.
	colorref := uint32(value.R) | uint32(value.G)<<8 | uint32(value.B)<<16
	err := windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&colorref), uint32(unsafe.Sizeof(colorref)),
	)
	if err != nil {
		w.logger.Warn("window frame attribute failed", "attribute", attribute, "error", err)
	}
}
