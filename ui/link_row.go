package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

type wpLinkAction struct {
	label   string
	click   *widget.Clickable
	onClick func()
}

// layoutLinkRow centers a row of small links under the card content.
func (w *partyWindow) layoutLinkRow(gtx layout.Context, cardWidth int, px func(float32) int, actions []wpLinkAction) {
	if len(actions) == 0 {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	padX := px(8)
	padY := px(6)
	gap := px(6)
	widths := make([]int, len(actions))
	total := 0
	height := 0
	for index, action := range actions {
		_, ink := w.typo.measure(gtx, action.label, face, wpLinkSize)
		widths[index] = ink.Dx() + 2*padX
		if actionHeight := ink.Dy() + 2*padY; actionHeight > height {
			height = actionHeight
		}
		total += widths[index]
	}
	if len(actions) > 1 {
		total += gap * (len(actions) - 1)
	}
	if total <= 0 || height <= 0 {
		return
	}
	top := px(wpLinkTop)
	x := (cardWidth - total) / 2
	for index, action := range actions {
		rect := image.Rect(x, top, x+widths[index], top+height)
		if !rect.Empty() {
			clicked := action.click.Clicked(gtx)
			col := wpLinkColor
			if action.click.Hovered() {
				col = wpLinkHoverColor
			}
			w.typo.drawCentered(gtx, action.label, face, wpLinkSize, col, centerOf(rect))
			btnGtx := gtx
			btnGtx.Constraints = layout.Exact(rect.Size())
			stack := op.Offset(rect.Min).Push(gtx.Ops)
			action.click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: rect.Size()}
			})
			stack.Pop()
			if clicked && action.onClick != nil {
				action.onClick()
			}
		}
		x += widths[index] + gap
	}
}
