package ui

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// iconButtonShape selects how the shared icon button paints its background.
type iconButtonShape int

const (
	iconButtonRounded iconButtonShape = iota
	iconButtonCircle
)

// iconButtonStyle describes one icon button. The zero value is a transparent
// rounded button, so callers never inherit a colour or a radius by accident.
type iconButtonStyle struct {
	shape       iconButtonShape
	fill        color.NRGBA
	fillHover   color.NRGBA
	border      color.NRGBA
	borderHover color.NRGBA
	gradient    bool
	radius      unit.Dp
}

func (s iconButtonStyle) palette(progress float32) (fill, border color.NRGBA) {
	return mixColor(s.fill, s.fillHover, progress),
		mixColor(s.border, s.borderHover, progress)
}

func (s iconButtonStyle) cornerRadius(gtx layout.Context, local image.Rectangle) int {
	if s.shape == iconButtonCircle {
		return local.Dy() / 2
	}
	radius := s.radius
	if radius <= 0 {
		radius = unit.Dp(8)
	}
	return gtx.Dp(radius)
}

// iconButton owns the interaction state of one icon-only control. Screens that
// already keep their clickable and hover state elsewhere can call
// layoutIconButton directly; this wrapper exists for new controls.
type iconButton struct {
	click    widget.Clickable
	hover    hoverAnim
	style    iconButtonStyle
	disabled bool
}

func (b *iconButton) layout(gtx layout.Context, rect image.Rectangle, draw func(layout.Context, image.Rectangle, float32)) bool {
	clicked := layoutIconButton(gtx, rect, &b.click, &b.hover, b.style, true, draw)
	if b.disabled {
		return false
	}
	return clicked
}

// layoutIconButton paints one shared icon button and reports a click. The
// caller owns the clickable and hover state, which keeps existing row/button
// state intact while reusing the shape, palette and hit-area rules.
func layoutIconButton(
	gtx layout.Context,
	rect image.Rectangle,
	click *widget.Clickable,
	hover *hoverAnim,
	style iconButtonStyle,
	hoverAllowed bool,
	draw func(layout.Context, image.Rectangle, float32),
) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	if hovered && !hoverAllowed {
		hovered = false
	}
	progress := hover.update(gtx, hovered)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		drawIconButtonSurface(gtx, local, style, progress)
		if draw != nil {
			draw(gtx, local, progress)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}

// drawIconButtonSurface paints only the button background. It is shared by
// controls that already sit inside another clickable region, such as a history
// row, and therefore must not create a second hit area.
func drawIconButtonSurface(gtx layout.Context, local image.Rectangle, style iconButtonStyle, progress float32) {
	if local.Empty() {
		return
	}
	fillColor, borderColor := style.palette(progress)
	radius := style.cornerRadius(gtx, local)
	if style.gradient {
		fillRoundedGradient(gtx, local, local, radius, fillColor, borderColor)
		return
	}
	fillRounded(gtx, borderColor, local, radius)
	inset := gtx.Dp(unit.Dp(1))
	if local.Dx() > 2*inset && local.Dy() > 2*inset {
		fillRounded(gtx, fillColor, local.Inset(inset), max(0, radius-inset))
	}
	strokeRoundedRect(gtx, local.Inset(inset), float32(radius), float32(inset), borderColor)
}
