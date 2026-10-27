package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yo/ui/draw"
)

// ToggleStyle describes the palette of the shared settings switch.
type ToggleStyle struct {
	OffEdge color.NRGBA
	OnColor color.NRGBA

	OnHover1 color.NRGBA
	OnHover2 color.NRGBA

	OffFill  color.NRGBA
	OffHover color.NRGBA

	KnobOn  color.NRGBA
	KnobOff color.NRGBA
}

// DrawToggle paints a settings switch. The geometry is intentionally
// independent of the row: callers can put the same switch in a panel, a relay
// row or a future inline control.
func DrawToggle(gtx layout.Context, rect image.Rectangle, on bool, hover float32, style ToggleStyle) {
	if rect.Empty() {
		return
	}
	radius := rect.Dy() / 2
	border := style.OffEdge
	if on {
		border = style.OnColor
	}
	draw.FillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	innerRadius := radius - gtx.Dp(unit.Dp(1))
	if innerRadius < 0 {
		innerRadius = 0
	}
	if on {
		c1 := draw.MixColor(style.OnColor, style.OnHover1, hover)
		c2 := draw.MixColor(style.OnColor, style.OnHover2, hover)
		draw.FillRoundedGradient(gtx, inner, inner, innerRadius, c1, c2)
	} else {
		fill := draw.MixColor(style.OffFill, style.OffHover, hover)
		draw.FillRounded(gtx, fill, inner, innerRadius)
	}

	knobRadius := rect.Dy()/2 - gtx.Dp(unit.Dp(3))
	if knobRadius < 1 {
		knobRadius = 1
	}
	knobX := rect.Min.X + rect.Dy()/2
	knobColor := style.KnobOff
	if on {
		knobX = rect.Max.X - rect.Dy()/2
		knobColor = style.KnobOn
	}
	draw.FillCircle(gtx, image.Pt(knobX, rect.Min.Y+rect.Dy()/2), knobRadius, knobColor)
}
