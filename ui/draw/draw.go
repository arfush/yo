// Package draw contains the low-level painting helpers shared by the UI.
//
// The helpers only depend on Gio and image primitives. Screen-specific layout
// and state stay in the root ui package so this package can be reused by the
// generic widgets without creating an import cycle.
package draw

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Fill paints an axis-aligned rectangle.
func Fill(gtx layout.Context, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}

// FillRounded paints a rectangle with rounded corners.
func FillRounded(gtx layout.Context, col color.NRGBA, r image.Rectangle, radius int) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

// FillRoundedGradient fills clipRect with a linear gradient that spans
// gradientRect. Separating the two rectangles keeps a short progress fill from
// compressing the gradient into a narrow strip.
func FillRoundedGradient(gtx layout.Context, clipRect, gradientRect image.Rectangle, radius int, c1, c2 color.NRGBA) {
	if clipRect.Empty() {
		return
	}
	stack := clip.UniformRRect(clipRect, radius).Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(gradientRect.Min.X), float32(gradientRect.Min.Y)),
		Color1: c1,
		Stop2:  f32.Pt(float32(gradientRect.Max.X), float32(gradientRect.Max.Y)),
		Color2: c2,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

// FillCircle paints a circle using a rounded rectangle whose radius is half
// its height. At UI sizes the difference from a true circle is invisible.
func FillCircle(gtx layout.Context, center image.Point, radius int, col color.NRGBA) {
	if radius <= 0 {
		return
	}
	FillRounded(gtx, col, image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius), radius)
}

// StrokeRoundedRect outlines r with circular corners. The stroke is centered
// on the path, so half of its width falls outside the rectangle.
func StrokeRoundedRect(gtx layout.Context, r image.Rectangle, radius, width float32, col color.NRGBA) {
	if r.Empty() {
		return
	}
	x0, y0 := float32(r.Min.X), float32(r.Min.Y)
	x1, y1 := float32(r.Max.X), float32(r.Max.Y)
	if maxRadius := float32(min(r.Dx(), r.Dy())) / 2; radius > maxRadius {
		radius = maxRadius
	}
	if radius < 0 {
		radius = 0
	}
	if width <= 0 {
		return
	}

	const kappa = 0.5522847498
	k := radius * kappa

	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(x0+radius, y0))
	path.LineTo(f32.Pt(x1-radius, y0))
	path.CubeTo(f32.Pt(x1-radius+k, y0), f32.Pt(x1, y0+radius-k), f32.Pt(x1, y0+radius))
	path.LineTo(f32.Pt(x1, y1-radius))
	path.CubeTo(f32.Pt(x1, y1-radius+k), f32.Pt(x1-radius+k, y1), f32.Pt(x1-radius, y1))
	path.LineTo(f32.Pt(x0+radius, y1))
	path.CubeTo(f32.Pt(x0+radius-k, y1), f32.Pt(x0, y1-radius+k), f32.Pt(x0, y1-radius))
	path.LineTo(f32.Pt(x0, y0+radius))
	path.CubeTo(f32.Pt(x0, y0+radius-k), f32.Pt(x0+radius-k, y0), f32.Pt(x0+radius, y0))
	path.Close()

	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

// StrokeLine draws a straight line with the given width.
func StrokeLine(gtx layout.Context, from, to f32.Point, width float32, col color.NRGBA) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(from)
	path.LineTo(to)
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

// Glow draws a soft edge using several low-alpha layers.
func Glow(gtx layout.Context, rect image.Rectangle, radius int, col color.NRGBA) {
	// Several low-alpha layers approximate a soft glow. A single translucent
	// ring reads as a border rather than a glow at this size.
	for _, inset := range []int{4, 3, 2, 1} {
		FillRounded(gtx, col, rect.Inset(-inset), radius+inset)
	}
}

// CenterOf returns the center point of r.
func CenterOf(rect image.Rectangle) image.Point {
	return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
}

// ColorMaterial records a paint color that callers can reuse across glyphs.
func ColorMaterial(gtx layout.Context, col color.NRGBA) op.CallOp {
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	return macro.Stop()
}

// WithAlpha replaces the alpha channel of c.
func WithAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// MixColor linearly interpolates between a and b.
func MixColor(a, b color.NRGBA, t float32) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	lerp := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: lerp(a.A, b.A)}
}

// Lighten raises every color channel by amount without wrapping.
func Lighten(c color.NRGBA, amount uint8) color.NRGBA {
	add := func(v uint8) uint8 {
		if int(v)+int(amount) > 255 {
			return 255
		}
		return v + amount
	}
	return color.NRGBA{R: add(c.R), G: add(c.G), B: add(c.B), A: c.A}
}

// DrawImage stretches src into r.
func DrawImage(gtx layout.Context, src image.Image, r image.Rectangle) {
	if src == nil || r.Empty() {
		return
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return
	}

	scale := f32.Pt(
		float32(r.Dx())/float32(bounds.Dx()),
		float32(r.Dy())/float32(bounds.Dy()),
	)

	defer clip.Rect(r).Push(gtx.Ops).Pop()
	defer op.Offset(r.Min).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), scale)).Push(gtx.Ops).Pop()

	imageOp := paint.NewImageOp(src)
	imageOp.Filter = paint.FilterLinear
	imageOp.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// DrawImageCoverRounded fills a rounded rectangle with an image, cropping the
// overflow instead of stretching the artwork.
func DrawImageCoverRounded(gtx layout.Context, src image.Image, r image.Rectangle, radius int) {
	if src == nil || r.Empty() {
		return
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return
	}
	scale := max(
		float32(r.Dx())/float32(bounds.Dx()),
		float32(r.Dy())/float32(bounds.Dy()),
	)
	scaledWidth := float32(bounds.Dx()) * scale
	scaledHeight := float32(bounds.Dy()) * scale
	offset := image.Pt(
		r.Min.X+int((float32(r.Dx())-scaledWidth)/2),
		r.Min.Y+int((float32(r.Dy())-scaledHeight)/2),
	)

	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	defer op.Offset(offset).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(scale, scale))).Push(gtx.Ops).Pop()

	imageOp := paint.NewImageOp(src)
	imageOp.Filter = paint.FilterLinear
	imageOp.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
