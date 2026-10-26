package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/ui/draw"
)

// PlacePopupMenu centers a popup on its anchor and flips it above when the
// footer or window edge would clip it. It is used by status pills.
func PlacePopupMenu(anchor image.Rectangle, width, height, drop, margin int, bounds image.Rectangle) image.Rectangle {
	x := anchor.Min.X + (anchor.Dx()-width)/2
	y := anchor.Max.Y + drop
	if x+width > bounds.Max.X-margin {
		x = bounds.Max.X - margin - width
	}
	if x < bounds.Min.X+margin {
		x = bounds.Min.X + margin
	}
	if y+height > bounds.Max.Y-margin {
		y = anchor.Min.Y - drop - height
	}
	if y < bounds.Min.Y+margin {
		y = bounds.Min.Y + margin
	}
	return image.Rect(x, y, x+width, y+height)
}

// PlacePopupList aligns a select list with the left edge of its trigger and
// prefers the space below it.
func PlacePopupList(anchor image.Rectangle, height, gap int, bounds image.Rectangle) image.Rectangle {
	below := image.Rect(anchor.Min.X, anchor.Max.Y+gap, anchor.Max.X, anchor.Max.Y+gap+height)
	if below.Max.Y <= bounds.Max.Y {
		return below
	}
	above := image.Rect(anchor.Min.X, anchor.Min.Y-gap-height, anchor.Max.X, anchor.Min.Y-gap)
	if above.Min.Y >= bounds.Min.Y {
		return above
	}
	return below
}

// DrawPopupMenu paints the shared popup surface. Options are laid out by the
// caller so a status menu and a provider select keep their own content.
func DrawPopupMenu(gtx layout.Context, rect image.Rectangle, border, fill color.NRGBA) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(unit.Dp(8))
	draw.FillRounded(gtx, border, rect, radius)
	draw.FillRounded(gtx, fill, rect.Inset(gtx.Dp(unit.Dp(1))), max(0, radius-gtx.Dp(unit.Dp(1))))
}

// LayoutOverlayScrim registers an invisible full-window click target. It is
// laid out before the menu options, which therefore sit above it.
func LayoutOverlayScrim(gtx layout.Context, rect image.Rectangle, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	// Clicked must run before Layout: Layout drains and discards the click
	// events, so checking afterwards can never report a click.
	clicked := click.Clicked(gtx)
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}
