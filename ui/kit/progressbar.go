package kit

import (
	"image"
	"math"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// ProgressBarOptions is the constructor input of ProgressBar.
type ProgressBarOptions struct {
	Width, Height float32
	Percent       int
	Text          string
}

// ProgressBar draws a fixed-size track with an accent fill and a centered text.
// Build it with NewProgressBar; update the percent and text with the setters.
// The zero value is invalid.
type ProgressBar struct {
	width   float32
	height  float32
	percent int
	text    string
}

// NewProgressBar validates opts and builds a progress bar. A non-positive size
// is a programmer error and panics. Percent is clamped to 0..100.
func NewProgressBar(opts ProgressBarOptions) ProgressBar {
	if opts.Width <= 0 || opts.Height <= 0 {
		panic("kit: non-positive progress bar size")
	}
	return ProgressBar{
		width:   opts.Width,
		height:  opts.Height,
		percent: clampPercent(opts.Percent),
		text:    opts.Text,
	}
}

// SetPercent replaces the fill percent, clamped to 0..100.
func (p *ProgressBar) SetPercent(percent int) {
	if p == nil {
		panic("kit: nil progress bar")
	}
	p.percent = clampPercent(percent)
}

// SetText replaces the centered text.
func (p *ProgressBar) SetText(text string) {
	if p == nil {
		panic("kit: nil progress bar")
	}
	p.text = text
}

// Size returns the progress bar size in logical pixels.
func (p *ProgressBar) Size() image.Point {
	if p == nil {
		panic("kit: nil progress bar")
	}
	return image.Pt(
		int(math.Round(float64(p.width))),
		int(math.Round(float64(p.height))),
	)
}

// Paint draws the track, the accent fill and the centered text at the painter
// origin.
func (p *ProgressBar) Paint(q *qt6.QPainter, app appearance.Appearance) {
	if p == nil {
		panic("kit: nil progress bar")
	}
	size := p.Size()
	palette := app.Palette()
	r := image.Rectangle{Max: size}
	line := max(1, int(appearance.Hairline))
	radius := math.Min(float64(appearance.RadiusS), float64(size.Y)/2)
	FillRounded(q, palette.Border, r, radius)
	FillRounded(q, palette.ProgressBar, r.Inset(line), max(0, radius-float64(line)))

	if p.percent > 0 {
		fill := image.Rect(0, 0, size.X*p.percent/100, size.Y)
		FillRounded(q, palette.Accent, fill, math.Min(radius, float64(fill.Dx())/2))
	}
	if p.text == "" {
		return
	}
	label := MeasureText(app, appearance.RoleLabel, p.text)
	q.Save()
	q.Translate2(
		float64((size.X-label.X)/2),
		float64((size.Y-label.Y)/2),
	)
	Text(q, app, appearance.RoleLabel, p.text, palette.Text)
	q.Restore()
}

// clampPercent keeps percent in the 0..100 range used by the fill.
func clampPercent(percent int) int {
	return min(max(percent, 0), 100)
}
