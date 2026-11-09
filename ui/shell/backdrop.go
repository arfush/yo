package shell

import (
	"image"
	"image/color"

	"gioui.org/gpu/headless"
	"gioui.org/op"
	"gioui.org/op/paint"
)

const (
	// blurRadiusDp is the blur radius of the modal backdrop in dp.
	blurRadiusDp = 4
	// blurPasses is the number of box-blur passes. Three passes give a smooth,
	// close-to-Gaussian falloff; the radius stays small so the screen behind the
	// dialog remains readable, like the reference.
	blurPasses = 3
)

// captureBackdrop renders content once at size over background and returns the
// blurred result. The offscreen window is kept between captures so reopening a
// dialog does not pay for a new GPU device. Capturing is expensive; callers
// cache the image. ok is false when the offscreen renderer is unavailable, so
// callers can fall back to the sharp content.
func (s *Shell) captureBackdrop(content op.CallOp, size image.Point, background color.NRGBA, radius int) (paint.ImageOp, bool) {
	if size.X <= 0 || size.Y <= 0 {
		return paint.ImageOp{}, false
	}
	if s.blurWindow == nil || s.blurSize != size {
		s.releaseBlurWindow()
		window, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			return paint.ImageOp{}, false
		}
		s.blurWindow = window
		s.blurSize = size
	}

	var ops op.Ops
	paint.Fill(&ops, background)
	content.Add(&ops)
	if err := s.blurWindow.Frame(&ops); err != nil {
		s.releaseBlurWindow()
		return paint.ImageOp{}, false
	}

	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := s.blurWindow.Screenshot(img); err != nil {
		s.releaseBlurWindow()
		return paint.ImageOp{}, false
	}
	boxBlur(img, radius, blurPasses)
	return paint.NewImageOp(img), true
}

// releaseBlurWindow drops the offscreen window. It is recreated on demand.
func (s *Shell) releaseBlurWindow() {
	if s.blurWindow != nil {
		s.blurWindow.Release()
		s.blurWindow = nil
	}
	s.blurSize = image.Point{}
}

// boxBlur applies a separable box blur to img, passes times.
func boxBlur(img *image.RGBA, radius, passes int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if radius <= 0 || passes <= 0 || w == 0 || h == 0 {
		return
	}
	radius = min(radius, max(w, h))
	tmp := image.NewRGBA(img.Rect)
	for range passes {
		blurHorizontal(img, tmp, radius)
		blurVertical(tmp, img, radius)
	}
}

// blurHorizontal writes the horizontal box blur of src into dst.
func blurHorizontal(src, dst *image.RGBA, radius int) {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	span := 2*radius + 1
	for y := range h {
		row := src.Pix[y*src.Stride:]
		out := dst.Pix[y*dst.Stride:]
		var r, g, b, a int
		for x := -radius; x <= radius; x++ {
			i := clampIndex(x, w) * 4
			r += int(row[i])
			g += int(row[i+1])
			b += int(row[i+2])
			a += int(row[i+3])
		}
		for x := range w {
			i := x * 4
			out[i] = uint8(r / span)
			out[i+1] = uint8(g / span)
			out[i+2] = uint8(b / span)
			out[i+3] = uint8(a / span)

			remove := clampIndex(x-radius, w) * 4
			add := clampIndex(x+radius+1, w) * 4
			r += int(row[add]) - int(row[remove])
			g += int(row[add+1]) - int(row[remove+1])
			b += int(row[add+2]) - int(row[remove+2])
			a += int(row[add+3]) - int(row[remove+3])
		}
	}
}

// blurVertical writes the vertical box blur of src into dst.
func blurVertical(src, dst *image.RGBA, radius int) {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	span := 2*radius + 1
	for x := range w {
		col := x * 4
		var r, g, b, a int
		for y := -radius; y <= radius; y++ {
			i := clampIndex(y, h)*src.Stride + col
			r += int(src.Pix[i])
			g += int(src.Pix[i+1])
			b += int(src.Pix[i+2])
			a += int(src.Pix[i+3])
		}
		for y := range h {
			i := y*src.Stride + col
			dst.Pix[i] = uint8(r / span)
			dst.Pix[i+1] = uint8(g / span)
			dst.Pix[i+2] = uint8(b / span)
			dst.Pix[i+3] = uint8(a / span)

			remove := clampIndex(y-radius, h)*src.Stride + col
			add := clampIndex(y+radius+1, h)*src.Stride + col
			r += int(src.Pix[add]) - int(src.Pix[remove])
			g += int(src.Pix[add+1]) - int(src.Pix[remove+1])
			b += int(src.Pix[add+2]) - int(src.Pix[remove+2])
			a += int(src.Pix[add+3]) - int(src.Pix[remove+3])
		}
	}
}

// clampIndex clamps i to [0, n-1] so the blur extends edge pixels.
func clampIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
