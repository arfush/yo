package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yonime/ui/draw"
)

// PanelStyle describes the rounded surface used by settings groups, title
// panels, the log card and the shortcuts card. The zero value is transparent.
type PanelStyle struct {
	Fill   color.NRGBA
	Border color.NRGBA
	Radius unit.Dp
}

// LayoutPanel paints the panel and returns its inner rectangle.
func (s PanelStyle) LayoutPanel(gtx layout.Context, rect image.Rectangle) image.Rectangle {
	if rect.Empty() {
		return image.Rectangle{}
	}
	radius := gtx.Dp(s.Radius)
	if radius < 0 {
		radius = 0
	}
	draw.FillRounded(gtx, s.Border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	if inner.Empty() {
		return image.Rectangle{}
	}
	draw.FillRounded(gtx, s.Fill, inner, max(0, radius-gtx.Dp(unit.Dp(1))))
	return inner
}
