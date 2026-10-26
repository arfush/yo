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

func drawOutlineRect(gtx layout.Context, g geometry, r image.Rectangle) {
	thickness := g.leastPx(1)

	fill(gtx, iconColor, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thickness))
	fill(gtx, iconColor, image.Rect(r.Min.X, r.Max.Y-thickness, r.Max.X, r.Max.Y))
	fill(gtx, iconColor, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thickness, r.Max.Y))
	fill(gtx, iconColor, image.Rect(r.Max.X-thickness, r.Min.Y, r.Max.X, r.Max.Y))
}

// drawLogIcon paints a document with text lines. It is deliberately close to
// the size of the gear and tray glyphs: the old 8.5x10.5 dp document read as a
// speck next to them in the footer.
func drawLogIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	width := int(g.vec(12))
	height := int(g.vec(14))
	if width < 2 || height < 2 {
		return
	}
	rect := image.Rect(
		center.X-width/2,
		center.Y-height/2,
		center.X+width/2,
		center.Y+height/2,
	)
	strokeRoundedRect(gtx, rect, g.vec(2.2), g.vec(1.3), col)
	lineLeft := rect.Min.X + int(g.vec(2.5))
	lineRight := rect.Max.X - int(g.vec(2.5))
	lineHeight := max(1, int(g.vec(1.3)))
	for index := 0; index < 4; index++ {
		y := rect.Min.Y + int(g.vec(3.6)+g.vec(2.5)*float32(index))
		right := lineRight
		if index == 3 {
			// A shorter last line keeps the glyph from looking like a solid
			// striped block at small sizes.
			right -= int(g.vec(2.5))
		}
		fill(gtx, col, image.Rect(lineLeft, y, right, y+lineHeight))
	}
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

// drawPeopleIcon paints two people for the watch-party control. The smaller
// figure stands behind the larger one: its shoulders disappear into the front
// silhouette, so the glyph reads as a pair watching together instead of two
// strangers side by side. bg is the control background used for that knockout;
// it follows the same stroke weight as the gear and the download tray.
func drawPeopleIcon(gtx layout.Context, center image.Point, g geometry, bg, col color.NRGBA) {
	width := g.vec(1.3)
	cx, cy := float32(center.X), float32(center.Y)
	pt := func(x, y float32) f32.Point {
		return f32.Pt(cx+g.vec(x), cy+g.vec(y))
	}

	// Back figure: head up and to the right, shoulders running behind the
	// front figure.
	strokeCircle(gtx, pt(2.6, -3.6), g.vec(1.85), width, col)
	var back clip.Path
	back.Begin(gtx.Ops)
	back.MoveTo(pt(-0.6, 5.4))
	back.CubeTo(pt(-0.1, 1.9), pt(6.6, 1.9), pt(6.6, 5.4))
	backStack := clip.Stroke{Path: back.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	backStack.Pop()

	// Front figure: lower and larger. Its filled body covers the shared part of
	// the back shoulders before the outline goes on top.
	var body clip.Path
	body.Begin(gtx.Ops)
	body.MoveTo(pt(-6.7, 5.4))
	body.CubeTo(pt(-6.7, 2.1), pt(0.9, 2.1), pt(0.9, 5.4))
	body.LineTo(pt(0.9, 6.4))
	body.LineTo(pt(-6.7, 6.4))
	body.Close()
	paint.FillShape(gtx.Ops, bg, clip.Outline{Path: body.End()}.Op())

	strokeCircle(gtx, pt(-2.4, -2.4), g.vec(2.15), width, col)
	var front clip.Path
	front.Begin(gtx.Ops)
	front.MoveTo(pt(-6.7, 5.4))
	front.CubeTo(pt(-6.7, 2.1), pt(0.9, 2.1), pt(0.9, 5.4))
	frontStack := clip.Stroke{Path: front.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	frontStack.Pop()
}

// drawCopyIcon paints two stacked sheets for the "copy room key" control. The
// back sheet is an open corner so the glyph stays readable on the gradient
// button, where a knockout fill would leave a dark patch.
func drawCopyIcon(gtx layout.Context, center image.Point, g geometry, col color.NRGBA) {
	width := g.vec(1.3)
	cx, cy := float32(center.X), float32(center.Y)
	half := g.vec(4.8)
	inner := half * 0.5

	var back clip.Path
	back.Begin(gtx.Ops)
	back.MoveTo(f32.Pt(cx-half, cy+inner))
	back.LineTo(f32.Pt(cx-half, cy-half))
	back.LineTo(f32.Pt(cx+inner, cy-half))
	stack := clip.Stroke{Path: back.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()

	front := image.Rect(
		int(math.Round(float64(cx-inner))),
		int(math.Round(float64(cy-inner))),
		int(math.Round(float64(cx+half))),
		int(math.Round(float64(cy+half))),
	)
	strokeRoundedRect(gtx, front, float32(g.px(1.4)), width, col)
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
