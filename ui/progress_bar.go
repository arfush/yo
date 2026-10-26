package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
)

// progressBarStyle describes the shared rounded progress track. A zero border
// skips the outer stroke; glow adds the soft download-button halo.
type progressBarStyle struct {
	track   color.NRGBA
	border  color.NRGBA
	fill1   color.NRGBA
	fill2   color.NRGBA
	glow    bool
	minFill int
}

// drawProgressBar paints the track and its filled part, returning the inner
// rectangle so callers can position a label over the bar.
func drawProgressBar(gtx layout.Context, rect image.Rectangle, value float32, style progressBarStyle) image.Rectangle {
	if rect.Empty() {
		return image.Rectangle{}
	}
	radius := rect.Dy() / 2
	inner := rect
	if style.border.A != 0 {
		fillRounded(gtx, style.border, rect, radius)
		inset := max(1, int(math.Round(float64(rect.Dy())*0.02)))
		inner = rect.Inset(inset)
		if inner.Empty() {
			return image.Rectangle{}
		}
		radius = inner.Dy() / 2
	}
	fillRounded(gtx, style.track, inner, radius)
	if value <= 0 {
		return inner
	}
	if value > 1 {
		value = 1
	}
	width := int(math.Round(float64(inner.Dx()) * float64(value)))
	if style.minFill > 0 && width < style.minFill {
		width = style.minFill
	}
	if width > inner.Dx() {
		width = inner.Dx()
	}
	if width <= 0 {
		return inner
	}
	filled := image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+width, inner.Max.Y)
	fillRadius := radius
	if fillRadius > width/2 {
		fillRadius = width / 2
	}
	if style.glow {
		drawFirstRunGlow(gtx, filled, fillRadius)
	}
	fillRoundedGradient(gtx, filled, inner, fillRadius, style.fill1, style.fill2)
	return inner
}
