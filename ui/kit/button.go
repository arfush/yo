package kit

import (
	"image"
	"math"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// buttonIconSize is the icon size inside a button, in logical pixels. It equals
// the native frame of the icon pack, so the glyph is not scaled down.
const buttonIconSize float32 = 24

// ButtonOptions is the constructor input of Button.
type ButtonOptions struct {
	Width, Height float32
	Icon          *qt.QPainterPath
	Text          string
	Accent        bool
	OnClick       func()
}

// Button is a themed push button. Build it with NewButton; the zero value is
// invalid. The caller draws it and calls Click when the user activates it.
type Button struct {
	width   float32
	height  float32
	icon    *qt.QPainterPath
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

// Size returns the button size in logical pixels.
func (b *Button) Size() image.Point {
	if b == nil {
		panic("kit: nil button")
	}
	return image.Pt(
		int(math.Round(float64(b.width))),
		int(math.Round(float64(b.height))),
	)
}

// Click runs the click callback.
func (b *Button) Click() {
	if b == nil {
		panic("kit: nil button")
	}
	if b.onClick != nil {
		b.onClick()
	}
}

// Paint draws the button at the painter origin. The accent variant uses the
// site accent; the plain one uses the neutral button tokens.
func (b *Button) Paint(p *qt.QPainter, app appearance.Appearance) {
	if b == nil {
		panic("kit: nil button")
	}
	palette := app.Palette()
	bg, border, content := palette.Button, palette.ButtonBorder, palette.Text
	if b.accent {
		bg, border, content = palette.Accent, palette.Accent, palette.OnAccent
	}

	size := b.Size()
	r := image.Rectangle{Max: size}
	line := max(1, int(appearance.Hairline))
	radius := float64(appearance.RadiusS)
	FillRounded(p, border, r, radius)
	FillRounded(p, bg, r.Inset(line), max(0, radius-float64(line)))

	iconSize := int(buttonIconSize)
	gap := int(appearance.SpaceS)
	label := MeasureText(app, appearance.RoleLabel, b.text)
	x := (size.X - (iconSize + gap + label.X)) / 2
	y := (size.Y - iconSize) / 2
	Icon(p, b.icon, image.Rect(x, y, x+iconSize, y+iconSize), content)

	if b.text != "" {
		p.Save()
		p.Translate2(
			float64(x+iconSize+gap),
			float64((size.Y-label.Y)/2),
		)
		Text(p, app, appearance.RoleLabel, b.text, content)
		p.Restore()
	}
}
