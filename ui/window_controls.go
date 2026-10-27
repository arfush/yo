package ui

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// Window-control glyphs share the 24-unit grid of the supplied icon sheet and
// fade together with the hover outline: the same progress drives both colours.
func (s *shell) drawMinimize(gtx layout.Context, g geometry, progress float32) {
	drawWindowControlIcon(gtx, "minimize", g, progress)
}

func (s *shell) drawMaximize(gtx layout.Context, g geometry, progress float32) {
	drawWindowControlIcon(gtx, "maximize", g, progress)
}

func (s *shell) drawRestore(gtx layout.Context, g geometry, progress float32) {
	drawWindowControlIcon(gtx, "restore", g, progress)
}

func (s *shell) drawClose(gtx layout.Context, g geometry, progress float32) {
	drawWindowControlIcon(gtx, "close", g, progress)
}

func drawWindowControlIcon(gtx layout.Context, name string, g geometry, progress float32) {
	// IconVG is recoloured at draw time, so hover needs no second asset.
	col := mixColor(windowControlIconColor, windowControlHoverIconColor, progress)
	drawAppIcon(gtx, name, g.iconCenter, g.px(24), col)
}

func (s *shell) clickControl(gtx layout.Context, g geometry, rect image.Rectangle, control *widget.Clickable, hover *hoverAnim, hoverColor color.NRGBA, draw func(layout.Context, float32)) bool {
	clicked := control.Clicked(gtx)
	hovered := control.Hovered()
	if hovered && !cursorInRect(rect) {
		hovered = false
	}
	progress := hover.update(gtx, hovered)

	// The empty parts of the header are non-client drag areas, so Windows stops
	// delivering pointer events as soon as the cursor leaves a control for the
	// header. Poll the real cursor while a highlight is visible to clear it.
	if hoverPollingSupported() && hover.Target > 0 && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(hoverPollInterval)})
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(rect.Size())
	control.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		drawControlHover(gtx, g, progress, hoverColor)
		draw(gtx, progress)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()

	return clicked
}
