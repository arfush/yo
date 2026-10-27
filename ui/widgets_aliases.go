package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/widgets"
)

// Icon button aliases keep the control call sites stable while the shared
// implementation lives in ui/widgets.
type iconButtonStyle = widgets.IconButtonStyle
type iconButtonShape = widgets.IconButtonShape

const (
	iconButtonRounded = widgets.IconButtonRounded
	iconButtonCircle  = widgets.IconButtonCircle
)

func layoutIconButton(
	gtx layout.Context,
	rect image.Rectangle,
	click *widget.Clickable,
	hover *hoverAnim,
	style iconButtonStyle,
	hoverAllowed bool,
	drawContent func(layout.Context, image.Rectangle, float32),
) bool {
	return widgets.LayoutIconButton(gtx, rect, click, &hover.Hover, style, hoverAllowed, drawContent)
}

func drawIconButtonSurface(gtx layout.Context, local image.Rectangle, style iconButtonStyle, progress float32) {
	widgets.DrawIconButtonSurface(gtx, local, style, progress)
}

// progressBarStyle is the screen-facing shape of the shared progress bar.
// Glow always uses the download-card halo color.
type progressBarStyle struct {
	track   color.NRGBA
	border  color.NRGBA
	fill1   color.NRGBA
	fill2   color.NRGBA
	glow    bool
	minFill int
}

func drawProgressBar(gtx layout.Context, rect image.Rectangle, value float32, style progressBarStyle) image.Rectangle {
	return widgets.DrawProgressBar(gtx, rect, value, widgets.ProgressBarStyle{
		Track:     style.track,
		Border:    style.border,
		Fill1:     style.fill1,
		Fill2:     style.fill2,
		Glow:      style.glow,
		GlowColor: firstRunGlowColor,
		MinFill:   style.minFill,
	})
}

func drawSpinner(gtx layout.Context, center image.Point, radius, dotRadius int, phase float64, col color.NRGBA) {
	widgets.DrawSpinner(gtx, center, radius, dotRadius, phase, col)
}

var (
	placePopupMenu     = widgets.PlacePopupMenu
	placePopupList     = widgets.PlacePopupList
	layoutOverlayScrim = widgets.LayoutOverlayScrim
)

func drawPopupMenu(gtx layout.Context, rect image.Rectangle) {
	widgets.DrawPopupMenu(gtx, rect, titlePanelBorder, settingsMenuFill)
}

func toggleSwitch(gtx layout.Context, rect image.Rectangle, on bool, hover float32) {
	widgets.DrawToggle(gtx, rect, on, hover, widgets.ToggleStyle{
		OffEdge:  settingsToggleOffEdge,
		OnColor:  settingsToggleOnColor,
		OnHover1: color.NRGBA{R: 0xae, G: 0x8b, B: 0xff, A: 0xff},
		OnHover2: color.NRGBA{R: 0x82, G: 0x3e, B: 0xee, A: 0xff},
		OffFill:  settingsToggleOffFill,
		OffHover: color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff},
		KnobOn:   settingsToggleKnob,
		KnobOff:  settingsToggleKnobOff,
	})
}
