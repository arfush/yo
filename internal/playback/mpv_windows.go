//go:build windows

package playback

import (
	"image"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func configureMPVCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

func mpvPipePath(name string) string {
	return `\\.\pipe\` + name
}

var (
	procEnumWindows            = windows.NewLazySystemDLL("user32.dll").NewProc("EnumWindows")
	procGetWindowThreadProcess = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	procGetWindowRect          = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowRect")
	procIsWindowVisible        = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowVisible")
)

type win32Rect struct {
	Left, Top, Right, Bottom int32
}

// windowRect finds the top-level window of the mpv process and returns its
// rectangle in physical pixels. mpv is a GUI player with one window, so the
// first visible top-level window owned by its PID is the player window.
func (m *mpvProcess) windowRect() (image.Rectangle, bool) {
	if m == nil || m.cmd == nil || m.cmd.Process == nil {
		return image.Rectangle{}, false
	}
	pid := uint32(m.cmd.Process.Pid)
	best := win32Rect{}
	found := false

	enum := func(hwnd, _ uintptr) uintptr {
		var owner uint32
		if result, _, _ := procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&owner))); result == 0 {
			return 1
		}
		if owner != pid {
			return 1
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var rect win32Rect
		if result, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); result == 0 {
			return 1
		}
		if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
			return 1
		}
		best = rect
		found = true
		return 1
	}
	procEnumWindows.Call(windows.NewCallback(enum), 0)
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rectangle{
		Min: image.Pt(int(best.Left), int(best.Top)),
		Max: image.Pt(int(best.Right), int(best.Bottom)),
	}, true
}
