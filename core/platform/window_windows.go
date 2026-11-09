//go:build windows

package platform

import (
	"image/color"
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
)

// HandleViewEvent applies the application palette to the native window frame.
// Gio reports the HWND once per window; a recreated window reports it again.
// Events other than a valid Win32ViewEvent are ignored.
func HandleViewEvent(e event.Event, border color.NRGBA) {
	view, ok := e.(app.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}
	hwnd := windows.HWND(view.HWND)
	setIntAttribute(hwnd, dwmwaUseImmersiveDarkMode, 1)
	setIntAttribute(hwnd, dwmwaWindowCornerPreference, dwmwcpRound)
	setColorAttribute(hwnd, dwmwaBorderColor, border)
}

func setIntAttribute(hwnd windows.HWND, attribute uint32, value int32) {
	_ = windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)),
	)
}

func setColorAttribute(hwnd windows.HWND, attribute uint32, value color.NRGBA) {
	// COLORREF is 0x00BBGGRR, not the NRGBA byte order.
	colorref := uint32(value.R) | uint32(value.G)<<8 | uint32(value.B)<<16
	_ = windows.DwmSetWindowAttribute(
		hwnd, attribute, unsafe.Pointer(&colorref), uint32(unsafe.Sizeof(colorref)),
	)
}
