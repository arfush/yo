package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// WithAlpha returns col with its alpha replaced, so a token can be used as a
// dim tint over the surface below.
func WithAlpha(col color.NRGBA, alpha uint8) color.NRGBA {
	col.A = alpha
	return col
}

// FillRect paints col over the axis-aligned rectangle r.
func FillRect(gtx layout.Context, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}

// FillRounded paints col over r with uniformly rounded corners. The radius is
// used as given: callers keep it within half the shorter side.
func FillRounded(gtx layout.Context, col color.NRGBA, r image.Rectangle, radius int) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

// Icon paints icon in the square at r.Min with the shorter side of r as the
// size. A nil icon or an empty rectangle draws nothing.
func Icon(gtx layout.Context, icon *widget.Icon, r image.Rectangle, col color.NRGBA) {
	if icon == nil || r.Empty() {
		return
	}
	size := min(r.Dx(), r.Dy())
	if size <= 0 {
		return
	}
	target := image.Rect(0, 0, size, size).Add(r.Min)
	stack := op.Offset(target.Min).Push(gtx.Ops)
	defer stack.Pop()
	iconGtx := gtx
	iconGtx.Constraints = layout.Exact(target.Size())
	icon.Layout(iconGtx, col)
}
