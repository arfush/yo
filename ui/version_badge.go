package ui

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
)

// versionBadge is the version pill and the placement of its label.
type versionBadge struct {
	// pill is the outer rectangle, border included.
	pill image.Rectangle
	// ink is the visual bounds of the label in its own coordinates; center is
	// where the painter puts its midpoint.
	ink image.Rectangle
	// textWidth is the typographic width of the label in device pixels. It
	// sizes the pill and is the value handed to mpv, which cannot measure text
	// itself.
	textWidth float32
	// center is the fractional point the label ink is centered on.
	center f32.Point
	// cap is the distance from the top of the label ink to the baseline. The
	// label is centered by this block, not by the full ink: the dot in "v1.2"
	// must not lift the digits above the middle of the pill.
	cap int
}

// measureVersionBadge sizes the pill from the label advance so any version
// string, short or long, keeps the same padding around its text. The advance,
// not the ink, is what both renderers can agree on: mpv centres its ASS line
// by it as well.
func measureVersionBadge(gtx layout.Context, typo *typography, text string, g geometry) (versionBadge, bool) {
	glyphs, ink := typo.measure(gtx, text, font.Font{Typeface: fontFamily, Weight: font.Medium}, badgeSize)
	if ink.Empty() || len(glyphs) == 0 {
		return versionBadge{}, false
	}
	last := glyphs[len(glyphs)-1]
	advance := float32(last.X+last.Advance) / 64

	border := g.leastPx(1)
	pad := g.px(badgePaddingX)
	height := g.px(badgeHeight)
	width := int(math.Round(float64(advance))) + 2*pad + 2*border
	top := g.headerCenterY - height/2
	pill := image.Rect(g.badgeX, top, g.badgeX+width, top+height)
	inner := pill.Inset(border)

	baseline := int(glyphs[0].Y)
	cap := baseline - ink.Min.Y
	if cap <= 0 || cap > ink.Dy() {
		cap = ink.Dy()
	}

	return versionBadge{
		pill:      pill,
		ink:       ink,
		textWidth: advance,
		center: f32.Pt(
			float32(inner.Min.X)+float32(inner.Dx())/2,
			float32(inner.Min.Y)+float32(inner.Dy())/2,
		),
		cap: cap,
	}, true
}
