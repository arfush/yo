package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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
