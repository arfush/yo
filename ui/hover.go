package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const (
	hoverDuration     = 140 * time.Millisecond
	hoverPollInterval = 40 * time.Millisecond
)

// hoverAnim eases a control highlight between 0 and 1 when the pointer enters
// or leaves its hitbox. It keeps requesting frames until the transition ends.
type hoverAnim struct {
	progress float32
	target   float32
	from     float32
	start    time.Time
	running  bool
}

func (a *hoverAnim) update(gtx layout.Context, hovered bool) float32 {
	target := float32(0)
	if hovered {
		target = 1
	}
	if target != a.target {
		a.from = a.progress
		a.target = target
		a.start = gtx.Now
		a.running = true
	}
	if !a.running {
		return a.progress
	}

	t := float32(gtx.Now.Sub(a.start)) / float32(hoverDuration)
	if t >= 1 {
		t = 1
		a.running = false
	}
	eased := t * t * (3 - 2*t)
	a.progress = a.from + (a.target-a.from)*eased

	if a.running {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	return a.progress
}

func drawControlHover(gtx layout.Context, g geometry, progress float32, base color.NRGBA) {
	if progress < 0.002 {
		return
	}
	base.A = uint8(float32(base.A) * progress)
	strokeRoundedRect(gtx, g.hoverRect(), float32(g.hoverRadius), float32(g.leastPx(1)), base)
}

// strokeRoundedRect outlines r with circular corners. The stroke is centered
// on the path, so half of its width falls outside the rectangle; the control
// hitbox has enough margin for that.
func strokeRoundedRect(gtx layout.Context, r image.Rectangle, radius, width float32, col color.NRGBA) {
	if r.Empty() {
		return
	}
	x0, y0 := float32(r.Min.X), float32(r.Min.Y)
	x1, y1 := float32(r.Max.X), float32(r.Max.Y)
	if maxRadius := float32(min(r.Dx(), r.Dy())) / 2; radius > maxRadius {
		radius = maxRadius
	}
	if radius < 0 {
		radius = 0
	}
	if width <= 0 {
		return
	}

	const kappa = 0.5522847498
	k := radius * kappa

	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(x0+radius, y0))
	path.LineTo(f32.Pt(x1-radius, y0))
	path.CubeTo(f32.Pt(x1-radius+k, y0), f32.Pt(x1, y0+radius-k), f32.Pt(x1, y0+radius))
	path.LineTo(f32.Pt(x1, y1-radius))
	path.CubeTo(f32.Pt(x1, y1-radius+k), f32.Pt(x1-radius+k, y1), f32.Pt(x1-radius, y1))
	path.LineTo(f32.Pt(x0+radius, y1))
	path.CubeTo(f32.Pt(x0+radius-k, y1), f32.Pt(x0, y1-radius+k), f32.Pt(x0, y1-radius))
	path.LineTo(f32.Pt(x0, y0+radius))
	path.CubeTo(f32.Pt(x0, y0+radius-k), f32.Pt(x0+radius-k, y0), f32.Pt(x0+radius, y0))
	path.Close()

	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}
