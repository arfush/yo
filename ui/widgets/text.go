package widgets

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
)

// Text is the small subset of the typography renderer that generic widgets
// need. The root ui package provides it through its textRenderer.
type Text interface {
	Measure(gtx layout.Context, txt string, face font.Font, size unit.Sp) ([]text.Glyph, image.Rectangle)
	Draw(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, at image.Point) image.Rectangle
	DrawCentered(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, center image.Point) image.Rectangle
	PaintCenteredOnBaseline(gtx layout.Context, txt string, face font.Font, size unit.Sp, col color.NRGBA, center f32.Point, capHeight int)
}
