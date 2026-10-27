//go:build windows

package window

import (
	"errors"
	"image"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

	biRGB        = 0
	dibRGBColors = 0
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procSendMessageW       = user32.NewProc("SendMessageW")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap       = gdi32.NewProc("CreateBitmap")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

type bitmapV5Header struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
	redMask       uint32
	greenMask     uint32
	blueMask      uint32
	alphaMask     uint32
	cstype        uint32
	endpoints     [36]byte
	gammaRed      uint32
	gammaGreen    uint32
	gammaBlue     uint32
	intent        uint32
	profileData   uint32
	profileSize   uint32
	reserved      uint32
}

type iconInfo struct {
	icon     int32
	xHotspot uint32
	yHotspot uint32
	mask     uintptr
	color    uintptr
}

var (
	iconMu      sync.Mutex
	pendingIcon image.Image
	currentIcon uintptr
)

// SetAppIcon schedules a new window/taskbar icon. The HICON conversion and
// WM_SETICON run on a worker goroutine: sending the message from the UI
// goroutine can re-enter the window procedure during layout.
func SetAppIcon(img image.Image) {
	if img == nil {
		return
	}
	iconMu.Lock()
	pendingIcon = img
	iconMu.Unlock()
	if windowHandle.Load() != 0 {
		go applyPendingAppIcon()
	}
}

func applyPendingAppIcon() {
	iconMu.Lock()
	defer iconMu.Unlock()

	hwnd := windows.HWND(windowHandle.Load())
	if hwnd == 0 {
		return
	}
	img := pendingIcon
	pendingIcon = nil
	if img == nil {
		// A new HWND can appear without a new section selection. Reuse the
		// HICON that is already owned by this process.
		if currentIcon != 0 {
			procSendMessageW.Call(uintptr(hwnd), wmSetIcon, iconBig, currentIcon)
			procSendMessageW.Call(uintptr(hwnd), wmSetIcon, iconSmall, currentIcon)
		}
		return
	}
	h, err := iconFromImage(img)
	if err != nil {
		return
	}

	// Both slots use the same 48px artwork. Windows scales it for the title
	// bar and the taskbar; setting both makes Alt+Tab and the taskbar agree.
	procSendMessageW.Call(uintptr(hwnd), wmSetIcon, iconBig, h)
	procSendMessageW.Call(uintptr(hwnd), wmSetIcon, iconSmall, h)

	if currentIcon != 0 && currentIcon != h {
		procDestroyIcon.Call(currentIcon)
	}
	currentIcon = h
}

// iconFromImage converts an arbitrary NRGBA image into a 32bpp HICON. This is
// the same CreateDIBSection/CreateIconIndirect path used by the tray packages;
// it keeps the alpha channel and avoids requiring a separate .ico per section.
func iconFromImage(src image.Image) (uintptr, error) {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, errors.New("empty icon")
	}

	stride := width * 4
	pixels := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := y*stride + x*4
			pixels[offset+0] = byte(b >> 8)
			pixels[offset+1] = byte(g >> 8)
			pixels[offset+2] = byte(r >> 8)
			pixels[offset+3] = byte(a >> 8)
		}
	}

	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, errors.New("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)

	header := bitmapV5Header{
		size:        uint32(unsafe.Sizeof(bitmapV5Header{})),
		width:       int32(width),
		height:      int32(-height),
		planes:      1,
		bitCount:    32,
		compression: biRGB,
		alphaMask:   0xff000000,
		redMask:     0x00ff0000,
		greenMask:   0x0000ff00,
		blueMask:    0x000000ff,
	}
	var bits unsafe.Pointer
	hColor, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&header)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hColor == 0 {
		return 0, errors.New("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hColor)
	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)

	maskStride := ((width + 15) / 16) * 2
	mask := make([]byte, maskStride*height)
	hMask, _, _ := procCreateBitmap.Call(
		uintptr(width), uintptr(height), 1, 1,
		uintptr(unsafe.Pointer(&mask[0])),
	)
	if hMask == 0 {
		return 0, errors.New("CreateBitmap failed")
	}
	defer procDeleteObject.Call(hMask)

	info := iconInfo{
		icon:  1,
		mask:  hMask,
		color: hColor,
	}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	if hIcon == 0 {
		return 0, errors.New("CreateIconIndirect failed")
	}
	return hIcon, nil
}
