package kit

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/appearance"
)

// Button geometry, in device-independent dp.
const (
	buttonHeight   unit.Dp = 35
	buttonPaddingX unit.Dp = 14
	buttonIconSize unit.Dp = 13
	buttonIconGap  unit.Dp = 8
)

// Button is a clickable label with an optional leading icon. A nil Icon draws
// no icon. The caller picks the look: LayoutNeutral ignores the active site
// palette, LayoutAccent follows it.
type Button struct {
	Label string
	Icon  *widget.Icon
	Click widget.Clickable
}

// buttonStyle is the resolved color set of one button look.
type buttonStyle struct {
	fill   color.NRGBA
	border color.NRGBA
	label  color.NRGBA
	focus  color.NRGBA
	hover  color.NRGBA
}

// LayoutNeutral draws the quiet outlined button. Its colors come from the
// neutral base palette, so the active site palette does not affect it.
func (b *Button) LayoutNeutral(gtx layout.Context, typo appearance.Typography) layout.Dimensions {
	return b.layout(gtx, typo, neutralButtonStyle())
}

// LayoutAccent draws the filled primary button. Its fill, label and focus
// follow the active palette.
func (b *Button) LayoutAccent(gtx layout.Context, typo appearance.Typography, palette appearance.Palette) layout.Dimensions {
	return b.layout(gtx, typo, accentButtonStyle(palette))
}

func (b *Button) layout(gtx layout.Context, typo appearance.Typography, style buttonStyle) layout.Dimensions {
	return b.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(b.Label).Add(gtx.Ops)
		return b.draw(gtx, typo, style)
	})
}

// draw measures the content, paints the button background and centers the
// optional icon and the label inside it.
func (b *Button) draw(gtx layout.Context, typo appearance.Typography, style buttonStyle) layout.Dimensions {
	height := min(gtx.Dp(buttonHeight), gtx.Constraints.Max.Y)
	if height <= 0 {
		return layout.Dimensions{}
	}

	macro := op.Record(gtx.Ops)
	textGtx := gtx
	textGtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, height)}
	textDims := Text(textGtx, typo, appearance.RoleBody, b.Label, style.label)
	label := macro.Stop()

	padding := gtx.Dp(buttonPaddingX)
	contentWidth := textDims.Size.X
	iconSize := 0
	if b.Icon != nil {
		iconSize = gtx.Dp(buttonIconSize)
		contentWidth += iconSize + gtx.Dp(buttonIconGap)
	}
	width := min(max(gtx.Constraints.Min.X, contentWidth+2*padding), gtx.Constraints.Max.X)
	if width <= 0 {
		return layout.Dimensions{}
	}

	r := image.Rect(0, 0, width, height)
	radius := gtx.Dp(appearance.RadiusS)
	line := max(1, gtx.Dp(appearance.Hairline))
	border := style.border
	if gtx.Focused(&b.Click) {
		border = style.focus
	}
	FillRounded(gtx, border, r, radius)
	inner := r.Inset(line)
	FillRounded(gtx, style.fill, inner, max(0, radius-line))
	if b.Click.Hovered() && !b.Click.Pressed() {
		FillRounded(gtx, style.hover, inner, max(0, radius-line))
	}

	x := (width - contentWidth) / 2
	if b.Icon != nil {
		y := (height - iconSize) / 2
		drawButtonIcon(gtx, b.Icon, image.Rect(x, y, x+iconSize, y+iconSize), style.label)
		x += iconSize + gtx.Dp(buttonIconGap)
	}
	offset := op.Offset(image.Pt(x, (height-textDims.Size.Y)/2)).Push(gtx.Ops)
	label.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(width, height)}
}

// neutralButtonStyle keeps the neutral base colors regardless of the active
// site palette.
func neutralButtonStyle() buttonStyle {
	p := appearance.BasePalette()
	return buttonStyle{
		fill:   p.SurfaceAlt,
		border: p.Edge,
		label:  p.Text,
		focus:  p.Focus,
		hover:  WithAlpha(p.Text, 0x08),
	}
}

// accentButtonStyle takes the fill, label and focus from the active palette.
func accentButtonStyle(p appearance.Palette) buttonStyle {
	return buttonStyle{
		fill:   p.Accent,
		border: p.Accent,
		label:  p.Text,
		focus:  p.Focus,
		hover:  WithAlpha(p.Text, 0x10),
	}
}

// drawButtonIcon paints icon into r with col.
func drawButtonIcon(gtx layout.Context, icon *widget.Icon, r image.Rectangle, col color.NRGBA) {
	if icon == nil || r.Empty() {
		return
	}
	stack := op.Offset(r.Min).Push(gtx.Ops)
	defer stack.Pop()
	iconGtx := gtx
	iconGtx.Constraints = layout.Exact(r.Size())
	icon.Layout(iconGtx, col)
}
