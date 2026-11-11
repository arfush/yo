package kit

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/appearance"
)

// buttonIconSize is the icon size inside a button, in dp. It equals the native
// frame of the icon pack, so the 1.6 stroke is not scaled down.
const buttonIconSize unit.Dp = 24

// ButtonOptions is the constructor input of Button.
type ButtonOptions struct {
	Width, Height unit.Dp
	Icon          *widget.Icon
	Text          string
	Accent        bool
	OnClick       func()
}

// Button is a themed push button. Build it with NewButton; the zero value is
// invalid. OnClick runs on the UI goroutine during Layout.
type Button struct {
	click   widget.Clickable
	width   unit.Dp
	height  unit.Dp
	icon    *widget.Icon
	text    string
	accent  bool
	onClick func()
}

// NewButton validates opts and builds a button. A nil icon or a non-positive
// size is a programmer error and panics.
func NewButton(opts ButtonOptions) Button {
	if opts.Icon == nil {
		panic("kit: nil button icon")
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		panic("kit: non-positive button size")
	}
	return Button{
		width:   opts.Width,
		height:  opts.Height,
		icon:    opts.Icon,
		text:    opts.Text,
		accent:  opts.Accent,
		onClick: opts.OnClick,
	}
}

// Layout draws the button and calls OnClick when the user clicks it.
func (b *Button) Layout(gtx layout.Context, app appearance.Appearance) layout.Dimensions {
	if b == nil {
		panic("kit: nil button")
	}
	size := image.Pt(gtx.Dp(b.width), gtx.Dp(b.height))
	gtx.Constraints = layout.Exact(size)
	clicked := b.click.Clicked(gtx)
	dims := b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		if b.text != "" {
			semantic.DescriptionOp(b.text).Add(gtx.Ops)
		}
		b.draw(gtx, app)
		return layout.Dimensions{Size: size}
	})
	if clicked && b.onClick != nil {
		b.onClick()
	}
	return dims
}

// draw paints the background and the centered icon and text. The accent
// variant uses the site accent; the plain one uses the neutral button tokens.
func (b *Button) draw(gtx layout.Context, app appearance.Appearance) {
	palette := app.Palette()
	bg, border, content := palette.Button, palette.ButtonBorder, palette.Text
	if b.accent {
		bg, border, content = palette.Accent, palette.Accent, palette.OnAccent
	}

	size := gtx.Constraints.Max
	r := image.Rectangle{Max: size}
	line := max(1, gtx.Dp(appearance.Hairline))
	radius := gtx.Dp(appearance.RadiusS)
	FillRounded(gtx, border, r, radius)
	FillRounded(gtx, bg, r.Inset(line), max(0, radius-line))

	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(buttonIconSize)
				Icon(gtx, b.icon, image.Rect(0, 0, iconSize, iconSize), content)
				return layout.Dimensions{Size: image.Pt(iconSize, iconSize)}
			}),
			layout.Rigid(layout.Spacer{Width: appearance.SpaceS}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Text(gtx, app.Typography(), appearance.RoleLabel, b.text, content)
			}),
		)
	})
}
