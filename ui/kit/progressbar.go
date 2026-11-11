package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/arfush/yo/ui/appearance"
)

// ProgressBarOptions is the constructor input of ProgressBar.
type ProgressBarOptions struct {
	Width, Height unit.Dp
	Percent       int
	Text          string
}

// ProgressBar draws a fixed-size track with an accent fill and a centered text.
// Build it with NewProgressBar; update the percent and text with the setters.
// The zero value is invalid.
type ProgressBar struct {
	width   unit.Dp
	height  unit.Dp
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

// Layout draws the track, the accent fill and the centered text.
func (p *ProgressBar) Layout(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	if p == nil {
		panic("kit: nil progress bar")
	}
	size := image.Pt(gtx.Dp(p.width), gtx.Dp(p.height))
	gtx.Constraints = layout.Exact(size)
	palette := app.Palette()
	r := image.Rectangle{Max: size}
	line := max(1, gtx.Dp(appearance.Hairline))
	radius := min(gtx.Dp(appearance.RadiusS), size.Y/2)
	FillRounded(gtx, palette.Border, r, radius)
	FillRounded(gtx, palette.ProgressBar, r.Inset(line), max(0, radius-line))

	if p.percent > 0 {
		fill := image.Rect(0, 0, size.X*p.percent/100, size.Y)
		FillRounded(gtx, palette.Accent, fill, min(radius, fill.Dx()/2))
	}
	if p.text != "" {
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return Text(gtx, app.Typography(), appearance.RoleLabel, p.text, palette.Text)
		})
	}
	return layout.Dimensions{Size: size}
}

// clampPercent keeps percent in the 0..100 range used by the fill.
func clampPercent(percent int) int {
	return min(max(percent, 0), 100)
}
