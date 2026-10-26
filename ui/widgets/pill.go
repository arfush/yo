package widgets

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yonime/ui/draw"
)

// PillStyle is the shared capsule used by statuses, filters, menu items and
// shortcut keycaps. The caller owns the label and the palette.
type PillStyle struct {
	Text     color.NRGBA
	Fill     color.NRGBA
	Border   color.NRGBA
	TextSize unit.Sp
	Radius   unit.Dp
}

// LayoutPill paints a capsule and centers label on its cap-height band.
func (s PillStyle) LayoutPill(gtx layout.Context, typo Text, rect image.Rectangle, label string, face font.Font) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(s.Radius)
	if radius <= 0 {
		radius = gtx.Dp(unit.Dp(7))
	}
	draw.FillRounded(gtx, s.Border, rect, radius)
	draw.FillRounded(gtx, s.Fill, rect.Inset(gtx.Dp(unit.Dp(1))), max(0, radius-gtx.Dp(unit.Dp(1))))
	// Every pill is centered on the same cap-height band measured from one
	// reference glyph. Measuring each label's own ink makes labels with a
	// descender, an accent or a round overshoot sit half a pixel lower or
	// higher than their neighbours even though they share the baseline.
	capHeight := 0
	if _, refInk := typo.Measure(gtx, "Н", face, s.TextSize); !refInk.Empty() {
		capHeight = refInk.Dy()
	}
	typo.PaintCenteredOnBaseline(
		gtx,
		label,
		face,
		s.TextSize,
		s.Text,
		f32.Pt(float32(rect.Min.X)+float32(rect.Dx())/2, float32(rect.Min.Y)+float32(rect.Dy())/2),
		capHeight,
	)
}
