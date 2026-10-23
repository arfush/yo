//go:build windows

package ui

import (
	"image"
	"math"
	"sync/atomic"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"golang.org/x/sys/windows"

	"github.com/arfush/yonime/internal/browser"
)

const (
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	dwmwcpRound = 2

	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // -4
)

var (
	windowHandle atomic.Uintptr

	user32                           = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos                 = user32.NewProc("GetCursorPos")
	procGetClientRect                = user32.NewProc("GetClientRect")
	procGetWindowRect                = user32.NewProc("GetWindowRect")
	procGetAsyncKeyState             = user32.NewProc("GetAsyncKeyState")
	procScreenToClient               = user32.NewProc("ScreenToClient")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procShowWindow                   = user32.NewProc("ShowWindow")
	procSetForegroundWindow          = user32.NewProc("SetForegroundWindow")
	procIsIconic                     = user32.NewProc("IsIconic")
)

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9
)

const vkLeftButton = 0x01

type winPoint struct {
	X, Y int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

func hoverPollingSupported() bool { return true }

// mouseLeftPress reports the current physical left-button state and whether it
// was pressed since the previous call. The pressed flag is used for presses in
// non-client areas, where Gio never receives pointer events and Windows may run
// a modal move loop before the app can observe the button being held.
func mouseLeftPress() (down, pressed bool) {
	state, _, _ := procGetAsyncKeyState.Call(vkLeftButton)
	return state&0x8000 != 0, state&0x0001 != 0
}

// cursorInRect reports whether the system cursor is inside r. r is in Gio's
// device-pixel client coordinates, which match Win32 client coordinates for
// this per-monitor DPI-aware window.
func cursorInRect(r image.Rectangle) bool {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return true
	}
	// The Gio window thread is per-monitor DPI aware, but this polling code can
	// run on another Go thread. Match the window's coordinate space explicitly.
	if prev, _, _ := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2); prev != 0 {
		defer procSetThreadDpiAwarenessContext.Call(prev)
	}
	var pt winPoint
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); ok == 0 {
		return true
	}
	if ok, _, _ := procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt))); ok == 0 {
		return true
	}
	return image.Pt(int(pt.X), int(pt.Y)).In(r)
}

// cursorInWindow reports whether the system cursor is inside the client area.
// It is used to cancel a drag that leaves the window even if Gio's captured
// pointer stops delivering move events.
func cursorInWindow() bool {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return true
	}
	var rect winRect
	if ok, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return true
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return true
	}
	return cursorInRect(image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom)))
}

func handleViewEvent(e event.Event) {
	view, ok := e.(app.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}

	hwnd := windows.HWND(view.HWND)
	windowHandle.Store(view.HWND)

	dark := int32(1)
	_ = windows.DwmSetWindowAttribute(
		hwnd,
		dwmwaUseImmersiveDarkMode,
		unsafe.Pointer(&dark),
		uint32(unsafe.Sizeof(dark)),
	)

	corner := int32(dwmwcpRound)
	_ = windows.DwmSetWindowAttribute(
		hwnd,
		dwmwaWindowCornerPreference,
		unsafe.Pointer(&corner),
		uint32(unsafe.Sizeof(corner)),
	)

	// COLORREF is 0x00BBGGRR: #202127.
	border := uint32(0x00272120)
	_ = windows.DwmSetWindowAttribute(
		hwnd,
		dwmwaBorderColor,
		unsafe.Pointer(&border),
		uint32(unsafe.Sizeof(border)),
	)
}

// appWindowBounds returns the application window rectangle in device
// independent pixels. Chromium's --window-size/--window-position switches use
// DIPs, while GetWindowRect reports physical pixels.
func appWindowBounds(scale float64) (browser.WindowBounds, bool) {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return browser.WindowBounds{}, false
	}
	if scale <= 0 {
		scale = 1
	}
	var rect winRect
	if ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return browser.WindowBounds{}, false
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return browser.WindowBounds{}, false
	}
	return browser.WindowBounds{
		X:      int(math.Round(float64(rect.Left) / scale)),
		Y:      int(math.Round(float64(rect.Top) / scale)),
		Width:  int(math.Round(float64(rect.Right-rect.Left) / scale)),
		Height: int(math.Round(float64(rect.Bottom-rect.Top) / scale)),
	}, true
}

type windowRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// appWindowRect returns the physical window rectangle. mpv's --geometry uses
// physical pixels, so this deliberately skips the DIP conversion used for
// Chromium.
func appWindowRect() (windowRect, bool) {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return windowRect{}, false
	}
	var rect winRect
	if ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return windowRect{}, false
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return windowRect{}, false
	}
	return windowRect{
		X:      int(rect.Left),
		Y:      int(rect.Top),
		Width:  int(rect.Right - rect.Left),
		Height: int(rect.Bottom - rect.Top),
	}, true
}

func hideAppWindow() {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), swHide)
}

func showAppWindow() {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return
	}
	// SW_RESTORE also removes the maximized state. Only use it for a minimized
	// window; a merely hidden window keeps its size and maximized state.
	if iconic, _, _ := procIsIconic.Call(uintptr(hwnd)); iconic != 0 {
		_, _, _ = procShowWindow.Call(uintptr(hwnd), swRestore)
	} else {
		_, _, _ = procShowWindow.Call(uintptr(hwnd), swShow)
	}
	_, _, _ = procSetForegroundWindow.Call(uintptr(hwnd))
}
