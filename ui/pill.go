package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yoanime/ui/widgets"
)

// pillStyle is the screen-facing shape of the shared capsule. The actual
// drawing and cap-height centering live in ui/widgets.
type pillStyle struct {
	text     color.NRGBA
	fill     color.NRGBA
	border   color.NRGBA
	textSize unit.Sp
	radius   unit.Dp
}

func (s pillStyle) layout(gtx layout.Context, typo *typography, rect image.Rectangle, label string) {
	widgets.PillStyle{
		Text:     s.text,
		Fill:     s.fill,
		Border:   s.border,
		TextSize: s.textSize,
		Radius:   s.radius,
	}.LayoutPill(gtx, typo, rect, label, font.Font{Typeface: fontFamily, Weight: font.Medium})
}

func drawPill(gtx layout.Context, typo *typography, rect image.Rectangle, label string, size unit.Sp, textColor, fillColor, borderColor color.NRGBA) {
	pillStyle{text: textColor, fill: fillColor, border: borderColor, textSize: size}.layout(gtx, typo, rect, label)
}
