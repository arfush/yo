package kit

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

// Dialog geometry, in device-independent dp.
const (
	dialogRadius     unit.Dp = 12
	dialogTileSize   unit.Dp = 38
	dialogIconSize   unit.Dp = 24
	dialogCloseSize  unit.Dp = 24
	dialogCloseInset unit.Dp = 6
)

// DialogOptions is the constructor input of Dialog.
type DialogOptions struct {
	Width, Height unit.Dp
	Icon          *widget.Icon
	Title         string
	Description   string
	ShowClose     bool
	CloseIcon     *widget.Icon
	OnClose       func()
	Button        ButtonOptions
}

// Dialog is a fixed-size panel with an icon tile, a title, a description, an
// optional close control and a button. Build it with NewDialog; the zero value
// is invalid. Callbacks run on the UI goroutine during Layout.
type Dialog struct {
	close       widget.Clickable
	width       unit.Dp
	height      unit.Dp
	icon        *widget.Icon
	title       string
	description string
	showClose   bool
	closeIcon   *widget.Icon
	onClose     func()
	button      Button
}

// NewDialog validates opts and builds a dialog. A nil icon, a nil close icon
// while ShowClose is set, a non-positive size or an invalid button is a
// programmer error and panics.
func NewDialog(opts DialogOptions) Dialog {
	if opts.Icon == nil {
		panic("kit: nil dialog icon")
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		panic("kit: non-positive dialog size")
	}
	if opts.ShowClose && opts.CloseIcon == nil {
		panic("kit: nil dialog close icon")
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

// Layout draws the panel and its content at the current origin.
func (d *Dialog) Layout(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	if d == nil {
		panic("kit: nil dialog")
	}
	size := image.Pt(gtx.Dp(d.width), gtx.Dp(d.height))
	gtx.Constraints = layout.Exact(size)
	palette := app.Palette()
	r := image.Rectangle{Max: size}
	line := max(1, gtx.Dp(appearance.Hairline))
	radius := gtx.Dp(dialogRadius)
	FillRounded(gtx, palette.Border, r, radius)
	FillRounded(gtx, palette.Panel, r.Inset(line), max(0, radius-line))

	padding := gtx.Dp(appearance.SpaceContent)
	if size.X <= 2*padding || size.Y <= 2*padding {
		return layout.Dimensions{Size: size}
	}
	content := image.Rect(padding, padding, size.X-padding, size.Y-padding)
	clipStack := clip.Rect(r.Inset(line)).Push(gtx.Ops)
	defer clipStack.Pop()

	d.layoutHeader(gtx, app, content)
	d.layoutText(gtx, app, content)
	d.layoutButton(gtx, app, content)
	return layout.Dimensions{Size: size}
}

// layoutHeader paints the icon tile and the optional close control.
func (d *Dialog) layoutHeader(gtx layout.Context, app appearance.Appearance, content image.Rectangle) {
	palette := app.Palette()
	tile := gtx.Dp(dialogTileSize)
	tileRect := image.Rect(content.Min.X, content.Min.Y, content.Min.X+tile, content.Min.Y+tile)
	FillRounded(gtx, palette.SurfaceAlt, tileRect, gtx.Dp(appearance.RadiusS))

	iconSize := gtx.Dp(dialogIconSize)
	inset := (tile - iconSize) / 2
	iconRect := image.Rect(0, 0, iconSize, iconSize).Add(tileRect.Min.Add(image.Pt(inset, inset)))
	Icon(gtx, d.icon, iconRect, palette.Ink)

	if d.showClose {
		d.layoutClose(gtx, app, content)
	}
}

// layoutClose registers a padded hitbox around the close glyph.
func (d *Dialog) layoutClose(gtx layout.Context, app appearance.Appearance, content image.Rectangle) {
	iconSize := gtx.Dp(dialogCloseSize)
	iconRect := image.Rect(content.Max.X-iconSize, content.Min.Y, content.Max.X, content.Min.Y+iconSize)
	hit := iconRect.Inset(-gtx.Dp(dialogCloseInset))
	clicked := d.close.Clicked(gtx)

	stack := op.Offset(hit.Min).Push(gtx.Ops)
	hitGtx := gtx
	hitGtx.Constraints = layout.Exact(hit.Size())
	d.close.Layout(hitGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(app.Locale().Text(resource.TextWindowClose)).Add(gtx.Ops)
		return layout.Dimensions{Size: hit.Size()}
	})
	stack.Pop()

	Icon(gtx, d.closeIcon, iconRect, app.Palette().ControlIcon)
	if clicked && d.onClose != nil {
		d.onClose()
	}
}

// layoutText draws the title and the description above the button.
func (d *Dialog) layoutText(gtx layout.Context, app appearance.Appearance, content image.Rectangle) {
	palette := app.Palette()
	top := content.Min.Y + gtx.Dp(dialogTileSize) + gtx.Dp(appearance.SpaceXL)
	buttonTop := content.Max.Y - gtx.Dp(d.button.height)
	if buttonTop < content.Min.Y {
		buttonTop = content.Min.Y
	}
	clipStack := clip.Rect(image.Rect(content.Min.X, content.Min.Y, content.Max.X, buttonTop)).Push(gtx.Ops)
	defer clipStack.Pop()

	textGtx := gtx
	textGtx.Constraints = layout.Constraints{Max: image.Pt(content.Dx(), max(0, buttonTop-top))}
	stack := op.Offset(image.Pt(content.Min.X, top)).Push(gtx.Ops)
	title := TextBlock(textGtx, app.Typography(), appearance.RoleTitle, d.title, palette.Text)
	stack.Pop()

	top += title.Size.Y + gtx.Dp(appearance.SpaceM)
	if top >= buttonTop {
		return
	}
	textGtx.Constraints.Max.Y = max(0, buttonTop-top)
	stack = op.Offset(image.Pt(content.Min.X, top)).Push(gtx.Ops)
	TextBlock(textGtx, app.Typography(), appearance.RoleBody, d.description, palette.TextMuted)
	stack.Pop()
}

// layoutButton pins the button to the bottom of the content area.
func (d *Dialog) layoutButton(gtx layout.Context, app appearance.Appearance, content image.Rectangle) {
	top := content.Max.Y - gtx.Dp(d.button.height)
	if top < content.Min.Y {
		top = content.Min.Y
	}
	stack := op.Offset(image.Pt(content.Min.X, top)).Push(gtx.Ops)
	d.button.Layout(gtx, app)
	stack.Pop()
}
