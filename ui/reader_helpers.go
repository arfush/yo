package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
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

func rectFromCenter(center image.Point, size int) image.Rectangle {
	half := size / 2
	return image.Rect(center.X-half, center.Y-half, center.X-half+size, center.Y-half+size)
}

func chapterIndexForKey(chapters []string, key string) int {
	for index := range chapters {
		if chapters[index] == key {
			return index
		}
	}
	return -1
}

func drawCloseIcon(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	width := float32(max(1, size/7))
	strokeLine(gtx, f32.Pt(float32(center.X-size/2), float32(center.Y-size/2)), f32.Pt(float32(center.X+size/2), float32(center.Y+size/2)), width, col)
	strokeLine(gtx, f32.Pt(float32(center.X-size/2), float32(center.Y+size/2)), f32.Pt(float32(center.X+size/2), float32(center.Y-size/2)), width, col)
}
