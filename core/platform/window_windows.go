//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // -4

	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

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

	icon      []byte
	iconSmall windows.Handle
	iconBig   windows.Handle
	stale     []windows.Handle

	procCreateIconFromResourceEx *windows.LazyProc
	procDestroyIcon              *windows.LazyProc
	procGetAsyncKeyState         *windows.LazyProc
	procGetCursorPos             *windows.LazyProc
	procInvalidateRect           *windows.LazyProc
	procIsWindow                 *windows.LazyProc
	procPostMessageW             *windows.LazyProc
	procScreenToClient           *windows.LazyProc
	procSetThreadDpiAwareness    *windows.LazyProc
}

// NewWindow builds the native frame for one application window. A nil logger
// discards frame logs.
func NewWindow(logger *slog.Logger) *Window {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	return &Window{
		logger:                       logger,
		procCreateIconFromResourceEx: user32.NewProc("CreateIconFromResourceEx"),
		procDestroyIcon:              user32.NewProc("DestroyIcon"),
		procGetAsyncKeyState:         user32.NewProc("GetAsyncKeyState"),
		procGetCursorPos:             user32.NewProc("GetCursorPos"),
		procInvalidateRect:           user32.NewProc("InvalidateRect"),
		procIsWindow:                 user32.NewProc("IsWindow"),
		procPostMessageW:             user32.NewProc("PostMessageW"),
		procScreenToClient:           user32.NewProc("ScreenToClient"),
		procSetThreadDpiAwareness:    user32.NewProc("SetThreadDpiAwarenessContext"),
	}
}

// winPoint mirrors the Win32 POINT layout.
type winPoint struct {
	X, Y int32
}

// HoverPollingSupported reports that the system cursor can be read directly.
func (*Window) HoverPollingSupported() bool { return true }

// CursorInRect reports whether the system cursor is inside r. r is in Gio's
// device-pixel client coordinates, which match Win32 client coordinates for
// this per-monitor DPI-aware window. The header drag regions are non-client,
// so Gio stops delivering pointer events there and when the cursor leaves the
// window; polling the real cursor is the only way to clear a stale hover.
func (w *Window) CursorInRect(r image.Rectangle) bool {
	handle := w.handle.Load()
	if handle == 0 {
		return true
	}
	// Gio makes only its window thread per-monitor DPI aware, while layout runs
	// on another thread. Match the window's coordinate space for this call.
	previous, _, _ := w.procSetThreadDpiAwareness.Call(dpiAwarenessContextPerMonitorAwareV2)
	if previous != 0 {
		defer w.procSetThreadDpiAwareness.Call(previous)
	}
	var point winPoint
	if ok, _, _ := w.procGetCursorPos.Call(uintptr(unsafe.Pointer(&point))); ok == 0 {
		return true
	}
	if ok, _, _ := w.procScreenToClient.Call(handle, uintptr(unsafe.Pointer(&point))); ok == 0 {
		return true
	}
	return image.Pt(int(point.X), int(point.Y)).In(r)
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
		w.applyIcon(hwnd)
		w.startHeldRepaint(hwnd)
	}
	w.setIntAttribute(hwnd, dwmwaUseImmersiveDarkMode, 1)
	w.setIntAttribute(hwnd, dwmwaWindowCornerPreference, dwmwcpRound)
	w.setColorAttribute(hwnd, dwmwaBorderColor, border)
}

// SetIcon stores the ICO bytes and applies them to the window. Before Gio
// reports the HWND the bytes wait for HandleViewEvent.
func (w *Window) SetIcon(ico []byte) {
	w.icon = append(w.icon[:0], ico...)
	if hwnd := windows.HWND(w.handle.Load()); hwnd != 0 {
		w.applyIcon(hwnd)
	}
}

// applyIcon replaces the small and big window icons with the 16x16 and 32x32
// images of the stored ICO. A broken ICO is logged and leaves the old icon.
func (w *Window) applyIcon(hwnd windows.HWND) {
	if len(w.icon) == 0 {
		return
	}
	smallDIB, bigDIB, err := iconDIBs(w.icon)
	if err != nil {
		w.logger.Warn("window icon decode failed", "error", err)
		return
	}
	small, err := w.createIcon(smallDIB)
	if err != nil {
		w.logger.Warn("window small icon failed", "error", err)
		return
	}
	big, err := w.createIcon(bigDIB)
	if err != nil {
		w.destroyIcon(small)
		w.logger.Warn("window big icon failed", "error", err)
		return
	}
	// Gio owns the window on its own locked OS thread, so a synchronous
	// SendMessageW from the UI goroutine deadlocks: that thread is blocked
	// handing the next event to the UI goroutine and cannot pump the queue.
	// PostMessageW returns immediately and the window thread applies the icon.
	//
	// The previous pair may still be attached to the window until it drains
	// the posted messages, so destroy it only on the next switch.
	w.destroyIcons(w.stale)
	w.stale = append(w.stale[:0], w.iconSmall, w.iconBig)
	w.procPostMessageW.Call(uintptr(hwnd), wmSetIcon, iconSmall, uintptr(small))
	w.procPostMessageW.Call(uintptr(hwnd), wmSetIcon, iconBig, uintptr(big))
	w.iconSmall, w.iconBig = small, big
}

func (w *Window) createIcon(dib []byte) (windows.Handle, error) {
	if len(dib) == 0 {
		return 0, errors.New("empty icon bitmap")
	}
	result, _, callErr := w.procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&dib[0])),
		uintptr(len(dib)),
		1, // fIcon
		0x00030000,
		0,
		0,
		0, // LR_DEFAULTCOLOR
	)
	if result == 0 {
		if callErr == nil {
			callErr = errors.New("unknown error")
		}
		return 0, fmt.Errorf("CreateIconFromResourceEx: %w", callErr)
	}
	return windows.Handle(result), nil
}

func (w *Window) destroyIcon(icon windows.Handle) {
	if icon == 0 {
		return
	}
	w.procDestroyIcon.Call(uintptr(icon))
}

func (w *Window) destroyIcons(icons []windows.Handle) {
	for _, icon := range icons {
		w.destroyIcon(icon)
	}
}

// iconDIBs extracts the 16x16 and 32x32 PNG entries of an ICO and converts
// them to icon bitmaps.
func iconDIBs(ico []byte) (small, big []byte, err error) {
	if len(ico) < 6 {
		return nil, nil, errors.New("truncated ICO header")
	}
	if binary.LittleEndian.Uint16(ico[0:2]) != 0 || binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		return nil, nil, errors.New("invalid ICO header")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count == 0 || len(ico) < 6+16*count {
		return nil, nil, errors.New("invalid ICO directory")
	}
	for i := 0; i < count; i++ {
		entry := ico[6+16*i:]
		size := int(entry[0])
		if size == 0 {
			size = 256
		}
		if size != int(entry[1]) {
			continue
		}
		bytesInRes := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if bytesInRes <= 0 || offset < 0 || offset+bytesInRes > len(ico) {
			continue
		}
		if size != 16 && size != 32 {
			continue
		}
		img, decodeErr := png.Decode(bytes.NewReader(ico[offset : offset+bytesInRes]))
		if decodeErr != nil {
			continue
		}
		dib, dibErr := dibFromImage(img)
		if dibErr != nil {
			continue
		}
		if size == 16 {
			small = dib
		} else {
			big = dib
		}
	}
	if len(small) == 0 || len(big) == 0 {
		return nil, nil, errors.New("ICO has no 16x16 and 32x32 entries")
	}
	return small, big, nil
}

// dibFromImage builds a 32-bit icon bitmap: a BITMAPINFOHEADER, bottom-up BGRA
// pixels and a zero AND mask. The alpha channel carries transparency.
func dibFromImage(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("empty icon image")
	}
	maskStride := ((width + 31) / 32) * 4
	imageSize := width*height*4 + maskStride*height
	const headerSize = 40
	buf := make([]byte, headerSize+imageSize)
	binary.LittleEndian.PutUint32(buf[0:4], headerSize)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(width))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(height*2))
	binary.LittleEndian.PutUint16(buf[12:14], 1)
	binary.LittleEndian.PutUint16(buf[14:16], 32)
	binary.LittleEndian.PutUint32(buf[20:24], uint32(imageSize))
	offset := headerSize
	for y := height - 1; y >= 0; y-- {
		for x := 0; x < width; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			buf[offset+0] = c.B
			buf[offset+1] = c.G
			buf[offset+2] = c.R
			buf[offset+3] = c.A
			offset += 4
		}
	}
	return buf, nil
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
