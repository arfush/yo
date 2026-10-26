// Package widgets contains the reusable controls and paint helpers that are
// independent of the application's screen state.
package widgets

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/ui/draw"
	"github.com/arfush/yonime/ui/theme"
)

// IconButtonShape selects how an icon button paints its background.
type IconButtonShape int

const (
	IconButtonRounded IconButtonShape = iota
	IconButtonCircle
)

// IconButtonStyle describes one icon button. The zero value is a transparent
// rounded button, so callers never inherit a colour or a radius by accident.
type IconButtonStyle struct {
	Shape       IconButtonShape
	Fill        color.NRGBA
	FillHover   color.NRGBA
	Border      color.NRGBA
	BorderHover color.NRGBA
	Gradient    bool
	Radius      unit.Dp
}

// Palette returns the fill and border for the current hover progress.
func (s IconButtonStyle) Palette(progress float32) (fill, border color.NRGBA) {
	return draw.MixColor(s.Fill, s.FillHover, progress),
		draw.MixColor(s.Border, s.BorderHover, progress)
}

func (s IconButtonStyle) cornerRadius(gtx layout.Context, local image.Rectangle) int {
	if s.Shape == IconButtonCircle {
		return local.Dy() / 2
	}
	radius := s.Radius
	if radius <= 0 {
		radius = unit.Dp(8)
	}
	return gtx.Dp(radius)
}

// IconButton owns the interaction state of one icon-only control. Screens
// that already keep their clickable and hover state elsewhere can call
// LayoutIconButton directly.
type IconButton struct {
	Click    widget.Clickable
	Hover    theme.Hover
	Style    IconButtonStyle
	Disabled bool
}

// Layout paints the button and reports a click.
func (b *IconButton) Layout(gtx layout.Context, rect image.Rectangle, drawContent func(layout.Context, image.Rectangle, float32)) bool {
	clicked := LayoutIconButton(gtx, rect, &b.Click, &b.Hover, b.Style, true, drawContent)
	if b.Disabled {
		return false
	}
	return clicked
}

// LayoutIconButton paints one shared icon button and reports a click. The
// caller owns the clickable and hover state, which keeps existing row/button
// state intact while reusing the shape, palette and hit-area rules.
func LayoutIconButton(
	gtx layout.Context,
	rect image.Rectangle,
	click *widget.Clickable,
	hover *theme.Hover,
	style IconButtonStyle,
	hoverAllowed bool,
	drawContent func(layout.Context, image.Rectangle, float32),
) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	if hovered && !hoverAllowed {
		hovered = false
	}
	progress := hover.Update(gtx, hovered)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		DrawIconButtonSurface(gtx, local, style, progress)
		if drawContent != nil {
			drawContent(gtx, local, progress)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}

// DrawIconButtonSurface paints only the button background. It is shared by
// controls that already sit inside another clickable region, such as a history
// row, and therefore must not create a second hit area.
func DrawIconButtonSurface(gtx layout.Context, local image.Rectangle, style IconButtonStyle, progress float32) {
	if local.Empty() {
		return
	}
	fillColor, borderColor := style.Palette(progress)
	radius := style.cornerRadius(gtx, local)
	if style.Gradient {
		draw.FillRoundedGradient(gtx, local, local, radius, fillColor, borderColor)
		return
	}
	draw.FillRounded(gtx, borderColor, local, radius)
	inset := gtx.Dp(unit.Dp(1))
	if local.Dx() > 2*inset && local.Dy() > 2*inset {
		draw.FillRounded(gtx, fillColor, local.Inset(inset), max(0, radius-inset))
	}
	draw.StrokeRoundedRect(gtx, local.Inset(inset), float32(radius), float32(inset), borderColor)
}
