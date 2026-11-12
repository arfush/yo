package kit

import (
	"image"
	"math"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

// Dialog geometry, in logical pixels.
const (
	dialogRadius      float32 = 12
	dialogTileSize    float32 = 38
	dialogIconSize    float32 = 24
	dialogCloseSize   float32 = 24
	dialogCloseOffset float32 = 4
)

// DialogOptions is the constructor input of Dialog.
type DialogOptions struct {
	Width, Height float32
	Icon          resource.Path
	Title         string
	Description   string
	ShowClose     bool
	CloseIcon     resource.Path
	OnClose       func()
	Button        ButtonOptions
}

// Dialog is a fixed-size panel with an icon tile, a title, a description, an
// optional close control and a button. Build it with NewDialog; the zero value
// is invalid.
type Dialog struct {
	width       float32
	height      float32
	icon        resource.Path
	title       string
	description string
	showClose   bool
	closeIcon   resource.Path
	onClose     func()
	button      Button
}

// NewDialog validates opts and builds a dialog. An empty icon, an empty close
// icon while ShowClose is set, a non-positive size or an invalid button is a
// programmer error and panics.
func NewDialog(opts DialogOptions) Dialog {
	if opts.Icon == "" {
		panic("kit: empty dialog icon")
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		panic("kit: non-positive dialog size")
	}
	if opts.ShowClose && opts.CloseIcon == "" {
		panic("kit: empty dialog close icon")
	}
	return Dialog{
		width:       opts.Width,
		height:      opts.Height,
		icon:        opts.Icon,
		title:       opts.Title,
		description: opts.Description,
		showClose:   opts.ShowClose,
		closeIcon:   opts.CloseIcon,
		onClose:     opts.OnClose,
		button:      NewButton(opts.Button),
	}
}

// Size returns the dialog size in logical pixels.
func (d *Dialog) Size() image.Point {
	if d == nil {
		panic("kit: nil dialog")
	}
	return image.Pt(
		int(math.Round(float64(d.width))),
		int(math.Round(float64(d.height))),
	)
}

// Button returns the action button so the caller can forward activation.
func (d *Dialog) Button() *Button {
	if d == nil {
		panic("kit: nil dialog")
	}
	return &d.button
}

// Close runs the close callback.
func (d *Dialog) Close() {
	if d == nil {
		panic("kit: nil dialog")
	}
	if d.onClose != nil {
		d.onClose()
	}
}

// Paint draws the panel and its content at the painter origin.
func (d *Dialog) Paint(p *qt.QPainter, app appearance.Appearance) {
	if d == nil {
		panic("kit: nil dialog")
	}
	size := d.Size()
	palette := app.Palette()
	r := image.Rectangle{Max: size}
	line := max(1, int(appearance.Hairline))
	radius := float64(dialogRadius)
	FillRounded(p, palette.Border, r, radius)
	FillRounded(p, palette.Panel, r.Inset(line), max(0, radius-float64(line)))

	padding := int(appearance.SpaceContent)
	if size.X <= 2*padding || size.Y <= 2*padding {
		return
	}
	content := image.Rect(padding, padding, size.X-padding, size.Y-padding)

	p.Save()
	clip := roundedPath(r.Inset(line), max(0, radius-float64(line)))
	p.SetClipPath(clip)
	d.paintHeader(p, app, content)
	d.paintText(p, app, content)
	d.paintButton(p, app, content)
	p.Restore()
	clip.Delete()
}

// paintHeader paints the icon tile and the optional close control.
func (d *Dialog) paintHeader(p *qt.QPainter, app appearance.Appearance, content image.Rectangle) {
	palette := app.Palette()
	tile := int(dialogTileSize)
	tileRect := image.Rect(content.Min.X, content.Min.Y, content.Min.X+tile, content.Min.Y+tile)
	FillRounded(p, palette.SurfaceAlt, tileRect, float64(appearance.RadiusS))

	iconSize := int(dialogIconSize)
	inset := (tile - iconSize) / 2
	Icon(p, d.icon, image.Rect(0, 0, iconSize, iconSize).Add(tileRect.Min.Add(image.Pt(inset, inset))), palette.Ink)

	if !d.showClose {
		return
	}
	closeSize := int(dialogCloseSize)
	offset := int(dialogCloseOffset)
	closeRect := image.Rect(0, 0, closeSize, closeSize).Add(
		image.Pt(content.Max.X-closeSize+offset, content.Min.Y-offset))
	Icon(p, d.closeIcon, closeRect, palette.ControlIcon)
}

// paintText draws the title and the description above the button.
func (d *Dialog) paintText(p *qt.QPainter, app appearance.Appearance, content image.Rectangle) {
	palette := app.Palette()
	top := content.Min.Y + int(dialogTileSize) + int(appearance.SpaceXL)
	buttonTop := content.Max.Y - d.button.Size().Y
	if buttonTop < content.Min.Y {
		buttonTop = content.Min.Y
	}
	width := content.Dx()

	p.Save()
	p.SetClipRect2(content.Min.X, content.Min.Y, width, max(0, buttonTop-content.Min.Y))
	p.Translate2(float64(content.Min.X), float64(top))
	title := TextBlock(p, app, appearance.RoleTitle, d.title, palette.Text, width)
	p.Restore()

	top += title.Y + int(appearance.SpaceM)
	if top >= buttonTop {
		return
	}
	p.Save()
	p.SetClipRect2(content.Min.X, top, width, buttonTop-top)
	p.Translate2(float64(content.Min.X), float64(top))
	TextBlock(p, app, appearance.RoleBody, d.description, palette.TextMuted, width)
	p.Restore()
}

// paintButton pins the button to the bottom of the content area.
func (d *Dialog) paintButton(p *qt.QPainter, app appearance.Appearance, content image.Rectangle) {
	top := content.Max.Y - d.button.Size().Y
	if top < content.Min.Y {
		top = content.Min.Y
	}
	p.Save()
	p.Translate2(float64(content.Min.X), float64(top))
	d.button.Paint(p, app)
	p.Restore()
}
