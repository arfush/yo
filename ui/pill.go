package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

// pillStyle is the shared capsule used by statuses, filters, menu items and
// shortcut keycaps. The caller owns the label and the palette.
type pillStyle struct {
	text     color.NRGBA
	fill     color.NRGBA
	border   color.NRGBA
	textSize unit.Sp
	radius   unit.Dp
}

func (s pillStyle) layout(gtx layout.Context, typo *typography, rect image.Rectangle, label string) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(s.radius)
	if radius <= 0 {
		radius = gtx.Dp(unit.Dp(7))
	}
	fillRounded(gtx, s.border, rect, radius)
	fillRounded(gtx, s.fill, rect.Inset(gtx.Dp(unit.Dp(1))), max(0, radius-gtx.Dp(unit.Dp(1))))
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	// Every pill is centered on the same cap-height band measured from one
	// reference glyph. Measuring each label's own ink makes labels with a
	// descender, an accent or a round overshoot sit half a pixel lower or
	// higher than their neighbours even though they share the baseline.
	capHeight := 0
	if _, refInk := typo.measure(gtx, "Н", face, s.textSize); !refInk.Empty() {
		capHeight = refInk.Dy()
	}
	typo.paintCenteredOnBaseline(
		gtx,
		label,
		face,
		s.textSize,
		s.text,
		f32.Pt(float32(rect.Min.X)+float32(rect.Dx())/2, float32(rect.Min.Y)+float32(rect.Dy())/2),
		capHeight,
	)
}

func drawPill(gtx layout.Context, typo *typography, rect image.Rectangle, label string, size unit.Sp, textColor, fillColor, borderColor color.NRGBA) {
	pillStyle{text: textColor, fill: fillColor, border: borderColor, textSize: size}.layout(gtx, typo, rect, label)
}
