package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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

func drawOutlineRect(gtx layout.Context, g geometry, r image.Rectangle) {
	thickness := g.leastPx(1)

	fill(gtx, iconColor, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thickness))
	fill(gtx, iconColor, image.Rect(r.Min.X, r.Max.Y-thickness, r.Max.X, r.Max.Y))
	fill(gtx, iconColor, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thickness, r.Max.Y))
	fill(gtx, iconColor, image.Rect(r.Max.X-thickness, r.Min.Y, r.Max.X, r.Max.Y))
}

// drawDownloadIcon paints a downward arrow over a tray. It marks the update
// control.
func drawDownloadIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	// The painted bounds run from cy-6 (arrow top) to cy+6 (tray), so the icon
	// is visually centered inside the footer button instead of sitting low.
	width := g.vec(1.3)
	cx, cy := center.X, center.Y
	top := cy - g.px(6)
	wings := cy - g.px(1)
	tip := cy + g.px(3)
	trayTop := cy + g.px(3.5)
	tray := cy + g.px(6)

	strokeLine(gtx, f32.Pt(float32(cx), float32(top)), f32.Pt(float32(cx), float32(tip)), width, col)
	strokeLine(
		gtx,
		f32.Pt(float32(cx-g.px(3.5)), float32(wings)),
		f32.Pt(float32(cx), float32(tip)),
		width,
		col,
	)
	strokeLine(
		gtx,
		f32.Pt(float32(cx+g.px(3.5)), float32(wings)),
		f32.Pt(float32(cx), float32(tip)),
		width,
		col,
	)

	left := cx - g.px(5)
	right := cx + g.px(5)
	strokeLine(gtx, f32.Pt(float32(left), float32(trayTop)), f32.Pt(float32(left), float32(tray)), width, col)
	strokeLine(gtx, f32.Pt(float32(left), float32(tray)), f32.Pt(float32(right), float32(tray)), width, col)
	strokeLine(gtx, f32.Pt(float32(right), float32(tray)), f32.Pt(float32(right), float32(trayTop)), width, col)
}

// drawSettingsIcon paints the footer gear. Eight short teeth surround a double
// outline ring, which stays legible at the 34dp footer button size.
func drawSettingsIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	cx, cy := float32(center.X), float32(center.Y)
	width := g.vec(1.25)
	ring := g.vec(4.4)
	toothInner := g.vec(5.0)
	toothOuter := g.vec(6.8)
	toothWidth := g.vec(1.8)

	for index := 0; index < 8; index++ {
		angle := float64(index) * math.Pi / 4
		dirX := float32(math.Cos(angle))
		dirY := float32(math.Sin(angle))
		strokeLine(
			gtx,
			f32.Pt(cx+dirX*toothInner, cy+dirY*toothInner),
			f32.Pt(cx+dirX*toothOuter, cy+dirY*toothOuter),
			toothWidth,
			col,
		)
	}
	strokeCircle(gtx, f32.Pt(cx, cy), ring, width, col)
	strokeCircle(gtx, f32.Pt(cx, cy), g.vec(1.8), width, col)
}

func strokeCircle(gtx layout.Context, center f32.Point, radius, width float32, col color.NRGBA) {
	if radius <= 0 {
		return
	}
	const kappa = 0.5522847498
	k := radius * kappa
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(center.X+radius, center.Y))
	path.CubeTo(
		f32.Pt(center.X+radius, center.Y+k),
		f32.Pt(center.X+k, center.Y+radius),
		f32.Pt(center.X, center.Y+radius),
	)
	path.CubeTo(
		f32.Pt(center.X-k, center.Y+radius),
		f32.Pt(center.X-radius, center.Y+k),
		f32.Pt(center.X-radius, center.Y),
	)
	path.CubeTo(
		f32.Pt(center.X-radius, center.Y-k),
		f32.Pt(center.X-k, center.Y-radius),
		f32.Pt(center.X, center.Y-radius),
	)
	path.CubeTo(
		f32.Pt(center.X+k, center.Y-radius),
		f32.Pt(center.X+radius, center.Y-k),
		f32.Pt(center.X+radius, center.Y),
	)
	path.Close()
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

func strokeLine(gtx layout.Context, from, to f32.Point, width float32, col color.NRGBA) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(from)
	path.LineTo(to)
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}
