package dialog

import (
	"image"
	"image/color"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
)

// Dependency installation dialog geometry, in device-independent dp.
const (
	dependencyWidth      unit.Dp = 464
	dependencyPadding    unit.Dp = 28
	dependencyIconTile   unit.Dp = 38
	dependencyIconSize   unit.Dp = 17
	dependencyIconRadius unit.Dp = appearance.RadiusS
	dependencyShadow     unit.Dp = 16

	dependencyShadowLayers       = 12
	dependencyShadowAlpha  uint8 = 8
)

// dependencyIconSurface is the fixed tile fill from the reference; unlike the
// card it does not follow the site palette.
var dependencyIconSurface = color.NRGBA{R: 0x22, G: 0x23, B: 0x2c, A: 0xff}

// DependencyOptions configures the dependency installation dialog. Resource and
// Handler are required. ShowLater controls whether the neutral "not now" button
// is offered. OnDownload is optional: nil leaves the primary action as a
// presentation-only request.
type DependencyOptions struct {
	Resource    *resource.Resource
	Handler     navigator.Handler
	Title       string
	Description string
	ShowLater   bool
	OnDownload  func()
}

// DependencyInstallation asks the user to install a missing external component.
// It draws a centered card over the blurred screen and closes itself through the
// navigator handler.
type DependencyInstallation struct {
	app         stdx.ReadonlyHolder[appearance.Appearance]
	resource    *resource.Resource
	handler     navigator.Handler
	title       string
	description string
	showLater   bool
	onDownload  func()

	later    kit.Button
	download kit.Button
}

// NewDependencyInstallation builds the dependency installation dialog. The
// appearance holder is read every frame so the palette and language follow the
// active site. A nil resource or handler is a programmer error and panics.
func NewDependencyInstallation(
	app stdx.ReadonlyHolder[appearance.Appearance],
	opts DependencyOptions,
) *DependencyInstallation {
	if opts.Resource == nil {
		panic("dialog: nil resource")
	}
	if opts.Handler == nil {
		panic("dialog: nil handler")
	}
	return &DependencyInstallation{
		app:         app,
		resource:    opts.Resource,
		handler:     opts.Handler,
		title:       opts.Title,
		description: opts.Description,
		showLater:   opts.ShowLater,
		onDownload:  opts.OnDownload,
	}
}

// Layout centers the card in the dialog body. Clicks are polled before the
// buttons are laid out: laying a Clickable out consumes its pointer events.
func (d *DependencyInstallation) Layout(gtx layout.Context) {
	d.handleClicks(gtx)
	app := d.app.Value()
	gtx.Locale = system.Locale{Language: app.Locale().Tag().String()}
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return d.layoutCard(gtx, app)
	})
}

// LayoutFooter draws nothing: the dialog does not replace the footer band.
func (*DependencyInstallation) LayoutFooter(layout.Context) {}

// layoutCard measures the body, paints the card behind it and replays the body
// inside the padding. Recording keeps the clickable state from being laid out
// twice.
func (d *DependencyInstallation) layoutCard(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	shadow := gtx.Dp(dependencyShadow)
	width := min(gtx.Dp(dependencyWidth), gtx.Constraints.Max.X-2*shadow)
	if width <= 0 {
		return layout.Dimensions{}
	}
	padding := gtx.Dp(dependencyPadding)
	bodyGtx := gtx
	bodyGtx.Constraints = layout.Constraints{Max: image.Pt(
		max(0, width-2*padding),
		max(0, gtx.Constraints.Max.Y-2*padding),
	)}
	macro := op.Record(gtx.Ops)
	body := d.layoutBody(bodyGtx, app)
	call := macro.Stop()
	height := body.Size.Y + 2*padding

	d.drawCard(gtx, app.Palette(), image.Rect(0, 0, width, height))
	offset := op.Offset(image.Pt(padding, padding)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()

	return layout.Dimensions{Size: image.Pt(width, height)}
}

// layoutBody stacks the icon, heading, description and the action row.
func (d *DependencyInstallation) layoutBody(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	typo := app.Typography()
	palette := app.Palette()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutIcon(gtx, palette)
		}),
		layout.Rigid(layout.Spacer{Height: appearance.SpaceM}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Text(gtx, typo, appearance.RoleTitle, d.title, palette.Text)
		}),
		layout.Rigid(layout.Spacer{Height: appearance.SpaceS}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.TextBlock(gtx, typo, appearance.RoleBody, d.description, palette.TextMuted)
		}),
		layout.Rigid(layout.Spacer{Height: appearance.SpaceXL}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutActions(gtx, typo, palette)
		}),
	)
}

// layoutIcon paints the download tile: the badge surface and the glyph in the
// site accent.
func (d *DependencyInstallation) layoutIcon(gtx layout.Context, palette appearance.Palette) layout.Dimensions {
	size := gtx.Dp(dependencyIconTile)
	bounds := image.Rect(0, 0, size, size)
	radius := gtx.Dp(dependencyIconRadius)
	kit.FillRounded(gtx, dependencyIconSurface, bounds, radius)

	glyph := gtx.Dp(dependencyIconSize)
	at := image.Pt((size-glyph)/2, (size-glyph)/2)
	offset := op.Offset(at).Push(gtx.Ops)
	iconGtx := gtx
	iconGtx.Constraints = layout.Exact(image.Pt(glyph, glyph))
	d.resource.Icon(resource.IconDownload).Layout(iconGtx, palette.Accent)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// layoutActions shows the neutral and accent buttons in one full-width row. A
// hidden "not now" button leaves the primary action spanning the whole row.
func (d *DependencyInstallation) layoutActions(
	gtx layout.Context,
	typo appearance.Typography,
	palette appearance.Palette,
) layout.Dimensions {
	d.download.Label = d.app.Value().Locale().Text(resource.TextDependencyDownload)
	d.download.Icon = d.resource.Icon(resource.IconDownload)
	if !d.showLater {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return d.download.LayoutAccent(gtx, typo, palette)
			}),
		)
	}

	d.later.Label = d.app.Value().Locale().Text(resource.TextDependencyLater)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.later.LayoutNeutral(gtx, typo)
		}),
		layout.Rigid(layout.Spacer{Width: appearance.SpaceS}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return d.download.LayoutAccent(gtx, typo, palette)
		}),
	)
}

// handleClicks turns button presses into navigation. It runs before the body is
// laid out so the clickables still hold this frame's pointer events.
func (d *DependencyInstallation) handleClicks(gtx layout.Context) {
	if d.showLater && d.later.Click.Clicked(gtx) {
		d.handler.Navigate(navigator.Back{})
		return
	}
	if d.download.Click.Clicked(gtx) && d.onDownload != nil {
		d.onDownload()
	}
}

// drawCard paints the soft shadow, the border and the card surface.
func (d *DependencyInstallation) drawCard(gtx layout.Context, palette appearance.Palette, bounds image.Rectangle) {
	radius := gtx.Dp(appearance.RadiusL)
	d.drawShadow(gtx, palette, bounds, radius)
	line := max(1, gtx.Dp(appearance.Hairline))
	kit.FillRounded(gtx, palette.Border, bounds, radius)
	kit.FillRounded(gtx, palette.Surface, bounds.Inset(line), max(0, radius-line))
}

// drawShadow stacks translucent rounded layers around the card. More layers
// inward are more opaque, which approximates a soft drop shadow without a blur.
func (d *DependencyInstallation) drawShadow(
	gtx layout.Context,
	palette appearance.Palette,
	bounds image.Rectangle,
	radius int,
) {
	depth := gtx.Dp(dependencyShadow)
	if depth <= 0 {
		return
	}
	shadow := kit.WithAlpha(palette.Overlay, 0)
	for i := range dependencyShadowLayers {
		grow := depth * (dependencyShadowLayers - i) / dependencyShadowLayers
		shadow.A = uint8(int(dependencyShadowAlpha) * (i + 1) / dependencyShadowLayers)
		outer := image.Rect(
			bounds.Min.X-grow, bounds.Min.Y-grow,
			bounds.Max.X+grow, bounds.Max.Y+grow,
		)
		kit.FillRounded(gtx, shadow, outer, radius+grow)
	}
}
