package ui

import "github.com/arfush/yo/ui/draw"

// The root package predates the draw package. These aliases keep the screen
// code readable while the implementation lives in ui/draw and can be shared by
// ui/widgets without importing the screens.
var (
	fill                  = draw.Fill
	fillRounded           = draw.FillRounded
	fillRoundedGradient   = draw.FillRoundedGradient
	fillCircle            = draw.FillCircle
	strokeRoundedRect     = draw.StrokeRoundedRect
	strokeLine            = draw.StrokeLine
	drawGlow              = draw.Glow
	centerOf              = draw.CenterOf
	colorMaterial         = draw.ColorMaterial
	withAlpha             = draw.WithAlpha
	mixColor              = draw.MixColor
	lighten               = draw.Lighten
	drawImage             = draw.DrawImage
	drawImageCoverRounded = draw.DrawImageCoverRounded
)
