//go:build windows

package window

import (
	"image"
	"image/color"
	"testing"
)

func TestIconFromImageCreatesHandle(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	img.Set(3, 4, color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff})

	icon, err := iconFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if icon == 0 {
		t.Fatal("iconFromImage returned a zero HICON")
	}
	procDestroyIcon.Call(icon)
}
