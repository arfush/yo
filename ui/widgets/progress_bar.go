package widgets

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"

	"github.com/arfush/yonime/ui/draw"
)

// ProgressBarStyle describes a rounded progress track. A zero border skips
// the outer stroke; Glow adds a soft halo around the filled part.
type ProgressBarStyle struct {
	Track     color.NRGBA
	Border    color.NRGBA
	Fill1     color.NRGBA
	Fill2     color.NRGBA
	Glow      bool
	GlowColor color.NRGBA
	MinFill   int
}

// DrawProgressBar paints the track and its filled part, returning the inner
// rectangle so callers can position a label over the bar.
func DrawProgressBar(gtx layout.Context, rect image.Rectangle, value float32, style ProgressBarStyle) image.Rectangle {
	if rect.Empty() {
		return image.Rectangle{}
	}
	radius := rect.Dy() / 2
	inner := rect
	if style.Border.A != 0 {
		draw.FillRounded(gtx, style.Border, rect, radius)
		inset := max(1, int(math.Round(float64(rect.Dy())*0.02)))
		inner = rect.Inset(inset)
		if inner.Empty() {
			return image.Rectangle{}
		}
		radius = inner.Dy() / 2
	}
	draw.FillRounded(gtx, style.Track, inner, radius)
	if value <= 0 {
		return inner
	}
	if value > 1 {
		value = 1
	}
	width := int(math.Round(float64(inner.Dx()) * float64(value)))
	if style.MinFill > 0 && width < style.MinFill {
		width = style.MinFill
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
	if style.Glow {
		draw.Glow(gtx, filled, fillRadius, style.GlowColor)
	}
	draw.FillRoundedGradient(gtx, filled, inner, fillRadius, style.Fill1, style.Fill2)
	return inner
}
