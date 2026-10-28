package ui

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// layoutClickableRect draws a click->draw surface on the reader. It is the
// package-level counterpart to the title window's method so the reader and the
// shell dropdown can share one implementation.
func layoutClickableRect(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context, image.Rectangle)) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if draw != nil {
			draw(gtx, image.Rect(0, 0, rect.Dx(), rect.Dy()))
		}
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}

func chapterIndexForKey(chapters []string, key string) int {
	for index := range chapters {
		if chapters[index] == key {
			return index
		}
	}
	return -1
}
