//go:build windows

package tray

import (
	"image"
	"image/color"
	"testing"
	"unsafe"
)

// The tray structures are handed straight to Win32, so their layout has to
// match the C definitions. A mismatch does not fail loudly: the icon simply
// never appears in the notification area.
func TestTrayStructLayout(t *testing.T) {
	offsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"Wnd", unsafe.Offsetof(notifyIconData{}.Wnd), 8},
		{"ID", unsafe.Offsetof(notifyIconData{}.ID), 16},
		{"Flags", unsafe.Offsetof(notifyIconData{}.Flags), 20},
		{"CallbackMessage", unsafe.Offsetof(notifyIconData{}.CallbackMessage), 24},
		{"Icon", unsafe.Offsetof(notifyIconData{}.Icon), 32},
		{"Tip", unsafe.Offsetof(notifyIconData{}.Tip), 40},
		{"Info", unsafe.Offsetof(notifyIconData{}.Info), 304},
		{"InfoTitle", unsafe.Offsetof(notifyIconData{}.InfoTitle), 820},
		{"Guid", unsafe.Offsetof(notifyIconData{}.Guid), 952},
		{"BalloonIcon", unsafe.Offsetof(notifyIconData{}.BalloonIcon), 968},
		{"Message.WParam", unsafe.Offsetof(winMessage{}.WParam), 16},
		{"Message.LParam", unsafe.Offsetof(winMessage{}.LParam), 24},
		{"Message.Time", unsafe.Offsetof(winMessage{}.Time), 32},
		{"Message.Point", unsafe.Offsetof(winMessage{}.Point), 36},
		{"Class.WndProc", unsafe.Offsetof(wndClassExW{}.WndProc), 8},
		{"Class.Instance", unsafe.Offsetof(wndClassExW{}.Instance), 24},
		{"Class.ClassName", unsafe.Offsetof(wndClassExW{}.ClassName), 64},
	}
	for _, offset := range offsets {
		if offset.got != offset.want {
			t.Errorf("%s offset = %d, want %d", offset.name, offset.got, offset.want)
		}
	}

	sizes := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconData{}), 976},
		{"MSG", unsafe.Sizeof(winMessage{}), 48},
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassExW{}), 80},
		{"ICONINFO", unsafe.Sizeof(iconInfo{}), 32},
		{"BITMAPINFOHEADER", unsafe.Sizeof(bitmapInfoHeader{}), 40},
	}
	for _, size := range sizes {
		if size.got != size.want {
			t.Errorf("%s size = %d, want %d", size.name, size.got, size.want)
		}
	}
}

// Building the icon handle only uses GDI, so it is checked without a shell:
// this is the step that turns an image into the HICON the notification area
// needs.
func TestTrayIconHandle(t *testing.T) {
	icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			icon.Set(x, y, color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff})
		}
	}
	handle, err := iconHandle(icon)
	if err != nil {
		t.Fatalf("build tray icon: %v", err)
	}
	defer procDestroyIcon.Call(uintptr(handle))
	if handle == 0 {
		t.Fatal("build tray icon returned a null handle")
	}
}
