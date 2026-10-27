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

	"github.com/arfush/yoanime/internal/browser"
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
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
)

var (
	pendingAppRect    windowRect
	pendingAppRectSet bool
)

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9

	swpNoZOrder       = 0x0004
	swpNoActivate     = 0x0010
	swpAsyncWindowPos = 0x4000
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
	// SetWindowPos sends WM_WINDOWPOSCHANGED to the window thread, and that
	// thread waits for this goroutine to return to Window.Event() while it
	// delivers the resulting ConfigEvent. Calling it inline deadlocks both
	// threads, which is the "window is not responding" hang on every launch
	// with a remembered rectangle. Claim the rectangle here and move the
	// window from a separate goroutine, like hideToTray does with ShowWindow.
	if rect, ok := takeRememberedWindowRect(); ok {
		go setAppWindowRect(hwnd, rect)
	}

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

// rememberAppWindowRect stores the rectangle the main window should come back
// to. It is applied as soon as the window handle exists, on the first view
// event of the session.
func rememberAppWindowRect(rect windowRect) {
	pendingAppRect = rect
	pendingAppRectSet = true
}

// takeRememberedWindowRect claims the stored rectangle exactly once. Claiming it
// on the UI goroutine keeps repeated view events from scheduling the move twice.
func takeRememberedWindowRect() (windowRect, bool) {
	if !pendingAppRectSet {
		return windowRect{}, false
	}
	pendingAppRectSet = false
	return pendingAppRect, true
}

// setAppWindowRect moves the freshly created window to the spot the user kept
// it in during the previous session. The frame is placed unchanged and left
// inactive, so the first launch does not steal focus from itself. It must run
// off the UI goroutine; see handleViewEvent.
func setAppWindowRect(hwnd windows.HWND, rect windowRect) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	// GetSystemMetrics virtual screen bounds would need another DLL import;
	// a basic guard keeps the window within the primary monitor instead of
	// letting a stale row push it off-screen.
	_, _, _ = procSetWindowPos.Call(
		uintptr(hwnd),
		0,
		uintptr(int32(rect.X)),
		uintptr(int32(rect.Y)),
		uintptr(int32(rect.Width)),
		uintptr(int32(rect.Height)),
		// SWP_ASYNCWINDOWPOS posts the move to the window thread instead of
		// waiting for it, so a busy message loop can never wedge this goroutine.
		swpNoZOrder|swpNoActivate|swpAsyncWindowPos,
	)
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
