//go:build windows

package window

import (
	"image"
	"math"
	"sync/atomic"
	"unsafe"

	"gioui.org/app"
	"gioui.org/io/event"
	"golang.org/x/sys/windows"

	"github.com/arfush/yo/catalog"
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
	procGetSystemMetrics             = user32.NewProc("GetSystemMetrics")
)

var (
	pendingAppRect    Rect
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

// GetSystemMetrics indexes for the virtual-screen rectangle that spans every
// monitor in physical pixels. When a monitor sits to the left of or above the
// primary one its origin is negative.
const (
	smXvirtualscreen  = 76
	smYvirtualscreen  = 77
	smCxvirtualscreen = 78
	smCyvirtualscreen = 79

	// minVisiblePixels is the smallest overlap with the virtual screen that
	// still counts as reachable. Windows parks minimized windows at
	// (-32000,-32000), and stale rows can point at monitors that are no longer
	// attached; neither may ever be restored or stored. A window that merely
	// sticks out of the edge of a real monitor keeps its spot.
	minVisiblePixels = 64
)

const vkLeftButton = 0x01

type winPoint struct {
	X, Y int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

func HoverPollingSupported() bool { return true }

// MouseLeftPress reports the current physical left-button state and whether it
// was pressed since the previous call. The pressed flag is used for presses in
// non-client areas, where Gio never receives pointer events and Windows may run
// a modal move loop before the app can observe the button being held.
func MouseLeftPress() (down, pressed bool) {
	state, _, _ := procGetAsyncKeyState.Call(vkLeftButton)
	return state&0x8000 != 0, state&0x0001 != 0
}

// CursorInRect reports whether the system cursor is inside r. r is in Gio's
// device-pixel client coordinates, which match Win32 client coordinates for
// this per-monitor DPI-aware window.
func CursorInRect(r image.Rectangle) bool {
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

// CursorInWindow reports whether the system cursor is inside the client area.
// It is used to cancel a drag that leaves the window even if Gio's captured
// pointer stops delivering move events.
func CursorInWindow() bool {
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
	return CursorInRect(image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom)))
}

func HandleViewEvent(e event.Event) {
	view, ok := e.(app.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}

	hwnd := windows.HWND(view.HWND)
	previous := windowHandle.Swap(view.HWND)
	// The section chosen at startup was stored before the window existed. A
	// recreated HWND also needs the last HICON again, otherwise the taskbar
	// falls back to the executable icon until the next section switch.
	if previous != view.HWND {
		go applyPendingAppIcon()
	}
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

// AppWindowBounds returns the application window rectangle in device
// independent pixels. Chromium's --window-size/--window-position switches use
// DIPs, while GetWindowRect reports physical pixels.
func AppWindowBounds(scale float64) (catalog.Bounds, bool) {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return catalog.Bounds{}, false
	}
	if scale <= 0 {
		scale = 1
	}
	var rect winRect
	if ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return catalog.Bounds{}, false
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return catalog.Bounds{}, false
	}
	return catalog.Bounds{
		X:      int(math.Round(float64(rect.Left) / scale)),
		Y:      int(math.Round(float64(rect.Top) / scale)),
		Width:  int(math.Round(float64(rect.Right-rect.Left) / scale)),
		Height: int(math.Round(float64(rect.Bottom-rect.Top) / scale)),
	}, true
}

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func metric(index int32) int {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(int32(value))
}

// VirtualScreen returns the bounding rectangle of all monitors in physical
// pixels: the union of every display, including ones left of or above the
// primary one.
func VirtualScreen() (x, y, width, height int) {
	return metric(smXvirtualscreen), metric(smYvirtualscreen), metric(smCxvirtualscreen), metric(smCyvirtualscreen)
}

// OnScreen reports whether rect overlaps the virtual screen enough to be
// reachable by the user. Windows reports minimized windows at
// (-32000,-32000), and a settings row can point at a monitor that has been
// unplugged since; both would park the window where nobody can see it.
func OnScreen(rect Rect) bool {
	if rect.Width <= 0 || rect.Height <= 0 {
		return false
	}
	left := max(rect.X, metric(smXvirtualscreen))
	top := max(rect.Y, metric(smYvirtualscreen))
	right := min(rect.X+rect.Width, metric(smXvirtualscreen)+metric(smCxvirtualscreen))
	bottom := min(rect.Y+rect.Height, metric(smYvirtualscreen)+metric(smCyvirtualscreen))
	return right-left >= minVisiblePixels && bottom-top >= minVisiblePixels
}

// AppWindowRect returns the physical window rectangle. mpv's --geometry uses
// physical pixels, so this deliberately skips the DIP conversion used for
// Chromium.
func AppWindowRect() (Rect, bool) {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return Rect{}, false
	}
	var rect winRect
	if ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return Rect{}, false
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return Rect{}, false
	}
	return Rect{
		X:      int(rect.Left),
		Y:      int(rect.Top),
		Width:  int(rect.Right - rect.Left),
		Height: int(rect.Bottom - rect.Top),
	}, true
}

func Hide() {
	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(uintptr(hwnd), swHide)
}

// Remember stores the rectangle the main window should come back
// to. It is applied as soon as the window handle exists, on the first view
// event of the session.
func Remember(rect Rect) {
	pendingAppRect = rect
	pendingAppRectSet = true
}

// takeRememberedWindowRect claims the stored rectangle exactly once. Claiming it
// on the UI goroutine keeps repeated view events from scheduling the move twice.
func takeRememberedWindowRect() (Rect, bool) {
	if !pendingAppRectSet {
		return Rect{}, false
	}
	pendingAppRectSet = false
	return pendingAppRect, true
}

// setAppWindowRect moves the freshly created window to the spot the user kept
// it in during the previous session. The frame is placed unchanged and left
// inactive, so the first launch does not steal focus from itself. It must run
// off the UI goroutine; see HandleViewEvent.
func setAppWindowRect(hwnd windows.HWND, rect Rect) {
	// A stale row (unplugged monitor) or the (-32000,-32000) minimized
	// position must not park the window off-screen; the default placement of
	// the freshly created window is always reachable.
	if !OnScreen(rect) {
		return
	}
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

func Show() {
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
