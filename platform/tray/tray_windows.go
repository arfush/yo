//go:build windows

package tray

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

// The notification-area icon is created with plain Win32 calls instead of a
// third-party package: Shell_NotifyIcon posts its callbacks to a window, so a
// hidden callback window with its own message loop runs on a dedicated locked
// thread. The rest of the application only ever calls the two callbacks below,
// so no tray code has to know about Gio or its window thread.
//
// That hidden window is also the only handle a second launch has on the running
// process: it is the window ActivateRunningInstance posts to.
const (
	trayClassName = "yoanimeTrayWindow"
	trayTip       = "yoanime"

	// wmApp starts the message range the shell uses to notify the callback
	// window; x/sys/windows does not define the WM_ constants.
	wmApp              = 0x8000
	trayCallback       = wmApp + 1
	trayQuit           = wmApp + 2
	trayActivate       = wmApp + 3
	wmNull             = 0x0000
	wmDestroy          = 0x0002
	wmLButtonUp        = 0x0202
	wmLButtonDoubleClk = 0x0203
	wmRButtonUp        = 0x0205

	smCXSmIcon = 49

	notifyAdd    = 0
	notifyModify = 1
	notifyDelete = 2

	notifyFlagMessage = 0x0001
	notifyFlagIcon    = 0x0002
	notifyFlagTip     = 0x0004
	notifyFlagInfo    = 0x0010

	// NIIF_NONE keeps the status icon out of the toast. The series name and
	// text already carry the meaning, and the large blue "i" looks out of place
	// next to the app icon. The string fields are limited by the fixed arrays
	// in NOTIFYICONDATAW.
	notifyInfoNone = 0x00000000

	mfString    = 0x0000
	mfSeparator = 0x0800
	mfDefault   = 0x1000

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	// allowSetForegroundAny is ASFW_ANY (DWORD -1): granting the right to set
	// the foreground window to any process instead of one process id.
	allowSetForegroundAny = ^uintptr(0)

	// dpiAwarenessContextPerMonitorAwareV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2.
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

	menuOpen = 1
	menuExit = 2
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	gdi32                = windows.NewLazySystemDLL("gdi32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procRegisterClassExW             = user32.NewProc("RegisterClassExW")
	procCreateWindowExW              = user32.NewProc("CreateWindowExW")
	procDefWindowProcW               = user32.NewProc("DefWindowProcW")
	procDestroyWindow                = user32.NewProc("DestroyWindow")
	procGetMessageW                  = user32.NewProc("GetMessageW")
	procTranslateMessage             = user32.NewProc("TranslateMessage")
	procDispatchMessageW             = user32.NewProc("DispatchMessageW")
	procPostQuitMessage              = user32.NewProc("PostQuitMessage")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procCreatePopupMenu              = user32.NewProc("CreatePopupMenu")
	procAppendMenuW                  = user32.NewProc("AppendMenuW")
	procDestroyMenu                  = user32.NewProc("DestroyMenu")
	procTrackPopupMenu               = user32.NewProc("TrackPopupMenu")
	procGetSystemMetrics             = user32.NewProc("GetSystemMetrics")
	procCreateIconIndirect           = user32.NewProc("CreateIconIndirect")
	procDestroyIcon                  = user32.NewProc("DestroyIcon")
	procGetDC                        = user32.NewProc("GetDC")
	procReleaseDC                    = user32.NewProc("ReleaseDC")
	procFindWindowW                  = user32.NewProc("FindWindowW")
	procAllowSetForeground           = user32.NewProc("AllowSetForegroundWindow")
	procGetCursorPos                 = user32.NewProc("GetCursorPos")
	procSetForegroundWindow          = user32.NewProc("SetForegroundWindow")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")

	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap     = gdi32.NewProc("CreateBitmap")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
)

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type winPoint struct {
	X int32
	Y int32
}

type winMessage struct {
	Window  windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   winPoint
}

type notifyIconData struct {
	Size             uint32
	Wnd              windows.HWND
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             windows.Handle
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	Guid             windows.GUID
	BalloonIcon      windows.Handle
}

type iconInfo struct {
	IsIcon   int32 // fIcon
	HotspotX uint32
	HotspotY uint32
	Mask     windows.Handle
	Color    windows.Handle
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// tray owns the notification-area icon. open and exit are called from the tray
// thread, so they must be safe to call from another goroutine.
type Tray struct {
	open func()
	exit func()

	hwnd windows.HWND
	// ready carries the result of the window and icon creation back to
	// New; done is closed after the message loop cleaned everything up.
	ready   chan error
	done    chan struct{}
	closing sync.Once

	// notifyMu serializes Shell_NotifyIcon calls from the checker goroutine and
	// the tray thread.
	notifyMu sync.Mutex
}

func Supported() bool { return true }

// ActivateRunningInstance asks the copy that already owns the lock to show its
// window, and reports whether the request was handed over.
//
// A second launch is started while the first copy may be sitting in the
// notification area. The two processes share only the mutex, so the request
// travels as a window message to the hidden window the running copy created for
// its tray icon. Without that window — the icon could not be added, or the
// window never came up — there is nothing to ask, and the caller exits as
// before.
func ActivateRunningInstance() bool {
	// This process has just been started by the user, so it is the one allowed
	// to grant foreground rights; without it Windows only flashes the taskbar
	// button of the running copy instead of raising its window.
	procAllowSetForeground.Call(allowSetForegroundAny)

	className, err := windows.UTF16PtrFromString(trayClassName)
	if err != nil {
		return false
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	if hwnd == 0 {
		return false
	}
	posted, _, _ := procPostMessageW.Call(hwnd, trayActivate, 0, 0)
	return posted != 0
}

// New adds the notification-area icon and parks a message loop in the
// background. A failure is recoverable: the caller keeps the plain close
// behaviour and reports the reason.
func New(icon image.Image, onOpen, onExit func()) (*Tray, error) {
	t := &Tray{
		open:  onOpen,
		exit:  onExit,
		ready: make(chan error, 1),
		done:  make(chan struct{}),
	}
	go t.run(icon)
	if err := <-t.ready; err != nil {
		return nil, err
	}
	return t, nil
}

// Close removes the icon and stops the tray thread. It waits briefly so the
// icon disappears immediately instead of lingering until the pointer happens to
// pass over the notification area.
func (t *Tray) Close() {
	if t == nil {
		return
	}
	t.closing.Do(func() {
		procPostMessageW.Call(uintptr(t.hwnd), trayQuit, 0, 0)
		select {
		case <-t.done:
		case <-time.After(2 * time.Second):
		}
	})
}

func (t *Tray) run(icon image.Image) {
	// The tray callback window and its message loop belong to one OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	hicon, err := iconHandle(icon)
	if err != nil {
		t.ready <- err
		return
	}

	instance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString(trayClassName)
	class := wndClassExW{
		Size:      uint32(unsafe.Sizeof(wndClassExW{})),
		WndProc:   windows.NewCallback(t.windowProc),
		Instance:  windows.Handle(instance),
		ClassName: className,
	}
	atom, _, registerErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 && registerErr != windows.ERROR_CLASS_ALREADY_EXISTS {
		procDestroyIcon.Call(uintptr(hicon))
		t.ready <- errors.New("не удалось зарегистрировать окно трея")
		return
	}

	// A regular hidden window rather than a message-only one: the shell posts
	// the notification-area callbacks to this handle, and only top-level
	// windows are guaranteed to receive them. It has no WS_VISIBLE, so it never
	// appears in the taskbar or on the desktop.
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		0,
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		procDestroyIcon.Call(uintptr(hicon))
		t.ready <- errors.New("не удалось создать окно трея")
		return
	}
	t.hwnd = windows.HWND(hwnd)

	if !t.notify(notifyAdd, hicon) {
		procDestroyWindow.Call(hwnd)
		procDestroyIcon.Call(uintptr(hicon))
		t.ready <- errors.New("Shell_NotifyIcon не принял значок")
		return
	}
	t.ready <- nil

	t.messageLoop()

	t.notify(notifyDelete, 0)
	procDestroyWindow.Call(hwnd)
	procDestroyIcon.Call(uintptr(hicon))
}

func (t *Tray) messageLoop() {
	for {
		var message winMessage
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (t *Tray) windowProc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch uint32(message) {
	case trayCallback:
		switch uint32(lparam) {
		case wmLButtonUp, wmLButtonDoubleClk:
			if t.open != nil {
				t.open()
			}
		case wmRButtonUp:
			t.showMenu(windows.HWND(hwnd))
		}
		return 0
	case trayActivate:
		// A second launch asked for the window; it may be hidden in the
		// notification area, so bringing it back is the whole point.
		if t.open != nil {
			t.open()
		}
		return 0
	case wmDestroy, trayQuit:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}

// buildMenu creates the notification-area context menu. The caller owns the
// handle and destroys it.
func (t *Tray) buildMenu() uintptr {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return 0
	}
	appendMenuItem(menu, mfString|mfDefault, menuOpen, "Открыть yoanime")
	appendMenuItem(menu, mfSeparator, 0, "")
	appendMenuItem(menu, mfString, menuExit, "Выход")
	return menu
}

// showMenu draws the context menu on the notification-area thread.
// TrackPopupMenu runs its own modal loop, so the selection is returned directly
// and the actions run only after the menu is gone.
func (t *Tray) showMenu(hwnd windows.HWND) {
	menu := t.buildMenu()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	var point winPoint
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point))); ok == 0 {
		return
	}
	// The menu is dismissed only when its owner window is in the foreground
	// while it is shown; the posted null message ends the modal loop.
	procSetForegroundWindow.Call(uintptr(hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmNoNotify|tpmReturnCmd,
		uintptr(point.X),
		uintptr(point.Y),
		0,
		uintptr(hwnd),
		0,
	)
	procPostMessageW.Call(uintptr(hwnd), wmNull, 0, 0)

	switch cmd {
	case menuOpen:
		if t.open != nil {
			t.open()
		}
	case menuExit:
		if t.exit != nil {
			t.exit()
		}
	}
}

func appendMenuItem(menu uintptr, flags uint32, id uintptr, label string) {
	if label == "" {
		procAppendMenuW.Call(menu, uintptr(flags), 0, 0)
		return
	}
	text, err := windows.UTF16PtrFromString(label)
	if err != nil {
		return
	}
	procAppendMenuW.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(text)))
}

func (t *Tray) notify(command uint32, icon windows.Handle) bool {
	data := notifyIconData{
		Wnd:             t.hwnd,
		ID:              1,
		Flags:           notifyFlagMessage | notifyFlagIcon | notifyFlagTip,
		CallbackMessage: trayCallback,
		Icon:            icon,
	}
	data.Size = uint32(unsafe.Sizeof(data))
	if icon == 0 {
		data.Flags = notifyFlagMessage | notifyFlagTip
	}
	if tip, err := windows.UTF16FromString(trayTip); err == nil {
		copy(data.Tip[:], tip)
	}
	t.notifyMu.Lock()
	defer t.notifyMu.Unlock()
	ok, _, _ := procShellNotifyIconW.Call(uintptr(command), uintptr(unsafe.Pointer(&data)))
	return ok != 0
}

// NotifyMessage shows a standard Windows notification through the existing
// tray icon. NIF_INFO is queued by Explorer; on Windows 10/11 it appears in the
// notification center like any other application toast.
func (t *Tray) NotifyMessage(title, body string) error {
	if t == nil || t.hwnd == 0 {
		return errors.New("уведомления Windows недоступны")
	}
	data := notifyIconData{
		Wnd:       t.hwnd,
		ID:        1,
		Flags:     notifyFlagInfo,
		InfoFlags: notifyInfoNone,
	}
	data.Size = uint32(unsafe.Sizeof(data))
	if value, err := windows.UTF16FromString(title); err == nil {
		copy(data.InfoTitle[:], value)
	}
	// The shell draws the title and body as adjacent lines. A leading newline
	// leaves a small visual gap between the series name and the episode text.
	// One UTF-16 unit is reserved for it inside the fixed szInfo array.
	if body != "" {
		body = "\n" + TrimNotificationText(body, MaxNotificationBody-1)
	}
	if value, err := windows.UTF16FromString(body); err == nil {
		copy(data.Info[:], value)
	}

	t.notifyMu.Lock()
	defer t.notifyMu.Unlock()
	ok, _, _ := procShellNotifyIconW.Call(uintptr(notifyModify), uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		return errors.New("Windows не принял уведомление")
	}
	return nil
}

// iconHandle converts the embedded PNG into the small icon Explorer expects.
// The notification area uses a different size per DPI, so the icon is scaled
// from the 48px asset instead of being resized by the shell.
func iconHandle(source image.Image) (windows.Handle, error) {
	if source == nil {
		return 0, errors.New("иконка приложения недоступна")
	}
	// The tray thread does not inherit the DPI awareness of the Gio window
	// thread, so ask for the size in the same coordinate space the shell uses.
	if prev, _, _ := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2); prev != 0 {
		defer procSetThreadDpiAwarenessContext.Call(prev)
	}
	size, _, _ := procGetSystemMetrics.Call(smCXSmIcon)
	side := int(size)
	if side <= 0 {
		side = 16
	}

	// image.RGBA stores alpha-premultiplied pixels, exactly what a 32bpp icon
	// wants.
	scaled := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Over, nil)

	header := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(side),
		Height:      -int32(side), // top-down rows, matching image.RGBA
		Planes:      1,
		BitCount:    32,
		Compression: 0, // BI_RGB
	}
	dc, _, _ := procGetDC.Call(0)
	var bits unsafe.Pointer
	color, _, _ := procCreateDIBSection.Call(
		dc,
		uintptr(unsafe.Pointer(&header)),
		0,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if dc != 0 {
		procReleaseDC.Call(0, dc)
	}
	if color == 0 || bits == nil {
		return 0, errors.New("не удалось подготовить значок")
	}

	stride := side * 4
	rows := unsafe.Slice((*byte)(bits), side*stride)
	for y := 0; y < side; y++ {
		src := scaled.Pix[y*scaled.Stride : y*scaled.Stride+stride]
		dst := rows[y*stride : y*stride+stride]
		for x := 0; x < side; x++ {
			dst[x*4+0] = src[x*4+2] // blue
			dst[x*4+1] = src[x*4+1] // green
			dst[x*4+2] = src[x*4+0] // red
			dst[x*4+3] = src[x*4+3] // alpha
		}
	}

	// A zeroed monochrome mask: a 32bpp icon takes its transparency from the
	// color bitmap, and the AND mask is ignored.
	maskBits := make([]byte, ((side+15)/16)*2*side)
	mask, _, _ := procCreateBitmap.Call(uintptr(side), uintptr(side), 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	if mask == 0 {
		procDeleteObject.Call(color)
		return 0, errors.New("не удалось подготовить маску значка")
	}

	info := iconInfo{
		IsIcon: 1,
		Color:  windows.Handle(color),
		Mask:   windows.Handle(mask),
	}
	hicon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	// CreateIconIndirect copies both bitmaps.
	procDeleteObject.Call(color)
	procDeleteObject.Call(mask)
	if hicon == 0 {
		return 0, errors.New("не удалось собрать значок")
	}
	return windows.Handle(hicon), nil
}
