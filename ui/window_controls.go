package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// Control icons are laid out around the center of a control hitbox. The
// offsets below are expressed in design pixels so they scale with the window
// DPI.

func (s *shell) drawMinimize(gtx layout.Context, g geometry) {
	thickness := g.leastPx(1)
	top := g.iconCenter.Y - thickness/2

	fill(gtx, iconColor, image.Rect(
		g.iconCenter.X-g.px(5),
		top,
		g.iconCenter.X+g.px(5),
		top+thickness,
	))
}

func (s *shell) drawMaximize(gtx layout.Context, g geometry) {
	drawOutlineRect(gtx, g, g.iconOffsetRect(-5, -5, 5, 5))
}

func (s *shell) drawRestore(gtx layout.Context, g geometry) {
	back := g.iconOffsetRect(-3, -6, 6, 3)
	front := g.iconOffsetRect(-6, -3, 3, 6)
	thickness := g.leastPx(1)

	// The rear square is partially hidden by the front one: draw only the
	// edges that remain visible.
	fill(gtx, iconColor, image.Rect(
		back.Min.X, back.Min.Y, back.Max.X, back.Min.Y+thickness,
	))
	fill(gtx, iconColor, image.Rect(
		back.Max.X-thickness, back.Min.Y, back.Max.X, back.Max.Y,
	))
	fill(gtx, iconColor, image.Rect(
		front.Max.X, back.Max.Y-thickness, back.Max.X, back.Max.Y,
	))
	fill(gtx, iconColor, image.Rect(
		back.Min.X, back.Min.Y, back.Min.X+thickness, front.Min.Y,
	))

	drawOutlineRect(gtx, g, front)
}

func (s *shell) drawClose(gtx layout.Context, g geometry) {
	cx, cy := float32(g.iconCenter.X), float32(g.iconCenter.Y)
	r := g.vec(4.5)

	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(cx-r, cy-r))
	path.LineTo(f32.Pt(cx+r, cy+r))
	path.MoveTo(f32.Pt(cx+r, cy-r))
	path.LineTo(f32.Pt(cx-r, cy+r))

	stack := clip.Stroke{Path: path.End(), Width: g.vec(1.1)}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, iconColor)
	stack.Pop()
}

func (s *shell) clickControl(gtx layout.Context, g geometry, rect image.Rectangle, control *widget.Clickable, hover *hoverAnim, hoverColor color.NRGBA, draw func(layout.Context)) bool {
	clicked := control.Clicked(gtx)
	hovered := control.Hovered()
	if hovered && !cursorInRect(rect) {
		hovered = false
	}
	progress := hover.update(gtx, hovered)

	// The empty parts of the header are non-client drag areas, so Windows stops
	// delivering pointer events as soon as the cursor leaves a control for the
	// header. Poll the real cursor while a highlight is visible to clear it.
	if hoverPollingSupported() && hover.target > 0 && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(hoverPollInterval)})
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(rect.Size())
	control.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		drawControlHover(gtx, g, progress, hoverColor)
		draw(gtx)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()

	return clicked
}
