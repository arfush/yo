package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
)

// toggleSwitch paints the shared settings switch. The geometry is intentionally
// independent of the row: callers can put the same switch in a panel, a relay
// row or a future inline control.
func toggleSwitch(gtx layout.Context, rect image.Rectangle, on bool, hover float32) {
	if rect.Empty() {
		return
	}
	radius := rect.Dy() / 2
	border := settingsToggleOffEdge
	if on {
		border = settingsToggleOnColor
	}
	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	innerRadius := radius - gtx.Dp(unit.Dp(1))
	if innerRadius < 0 {
		innerRadius = 0
	}
	if on {
		c1 := mixColor(settingsToggleOnColor, color.NRGBA{R: 0xae, G: 0x8b, B: 0xff, A: 0xff}, hover)
		c2 := mixColor(mainAccentColor2, color.NRGBA{R: 0x82, G: 0x3e, B: 0xee, A: 0xff}, hover)
		fillRoundedGradient(gtx, inner, inner, innerRadius, c1, c2)
	} else {
		fill := mixColor(settingsToggleOffFill, color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}, hover)
		fillRounded(gtx, fill, inner, innerRadius)
	}

	knobRadius := rect.Dy()/2 - gtx.Dp(unit.Dp(3))
	if knobRadius < 1 {
		knobRadius = 1
	}
	knobX := rect.Min.X + rect.Dy()/2
	knobColor := settingsToggleKnobOff
	if on {
		knobX = rect.Max.X - rect.Dy()/2
		knobColor = settingsToggleKnob
	}
	fillCircle(gtx, image.Pt(knobX, rect.Min.Y+rect.Dy()/2), knobRadius, knobColor)
}
