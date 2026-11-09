package mascot

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
)

// Motion periods and Alert hand geometry. Hand coordinates are design units.
const (
	wavePeriod          = 2400 * time.Millisecond
	loadingRingPeriod   = 1400 * time.Millisecond
	panicTremblePeriod  = 90 * time.Millisecond
	panicTremble        = 0.6
	alertTogetherPeriod = time.Second

	alertHandRestX     = 11
	alertHandRestY     = 3.5
	alertHandTogetherX = 3.2
	alertHandTogetherY = 8.8
)

// scene is one resolved frame of the mascot.
type scene struct {
	m     Mascot
	g     grid
	phase float64
	style moodStyle
	pose  moodPose
}

// paint draws the character with an optional horizontal jitter in design units.
// Panic repaints clipped slices through it to build the glitch.
func (s scene) paint(gtx layout.Context, jitter float32) {
	c := s.style.colors
	bob := float32(math.Sin(s.phase*2*math.Pi/s.style.period.Seconds())) * s.style.amplitude
	motion := op.Affine(f32.Affine2D{}.Offset(f32.Pt(jitter*s.g.scale, bob*s.g.scale))).Push(gtx.Ops)
	defer motion.Pop()

	if s.style.loading {
		drawRing(gtx, s.g, s.phase, c.HairLight)
	}
	drawTail(gtx, s.g, c.HairShadow)
	drawBody(gtx, s.g, c)
	drawHead(gtx, s.g, c)
	drawFace(gtx, s.g, c, s.pose, s.m.palette)
	s.m.hat.draw(gtx, wardrobe{g: s.g, pal: s.m.palette, hats: hatColors()})
	left, right := s.hands()
	drawArms(gtx, s.g, c, left, right)
	drawPaw(gtx, s.g, left, c.Paw, c.HairShadow)
	drawPaw(gtx, s.g, right, c.Paw, c.HairShadow)
}

// hands returns the paw centers for this frame. Alert folds both hands onto the
// chest; Panic presses them to the cheeks with a tremble.
func (s scene) hands() (f32.Point, f32.Point) {
	if s.pose.displeased {
		return alertHands(s.g.pt, s.pose.together)
	}
	if s.pose.panic {
		tremble := float32(math.Sin(s.phase*2*math.Pi/panicTremblePeriod.Seconds())) * panicTremble
		return s.g.pt(-11+tremble, -1.6), s.g.pt(11+tremble, -1.6)
	}
	wave := float32(math.Sin(s.phase * 2 * math.Pi / wavePeriod.Seconds()))
	leftLift := smoothStep(wave*0.5+0.5) * 4.4
	rightLift := smoothStep(0.5-wave*0.5) * 4.4
	return s.g.pt(-11, 3.5-leftLift), s.g.pt(11, 3.5-rightLift)
}

// grid maps the 42x42 design space onto a centered square slot.
type grid struct {
	scale  float32
	center image.Point
}

func newGrid(rect image.Rectangle) grid {
	size := min(rect.Dx(), rect.Dy())
	return grid{
		scale:  float32(size) / designSize,
		center: image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2),
	}
}

func (g grid) px(v float32) int { return int(math.Round(float64(v * g.scale))) }

func (g grid) rect(x0, y0, x1, y1 float32) image.Rectangle {
	return image.Rect(
		g.center.X+g.px(x0), g.center.Y+g.px(y0),
		g.center.X+g.px(x1), g.center.Y+g.px(y1),
	)
}

func (g grid) pt(x, y float32) f32.Point {
	return f32.Pt(float32(g.center.X)+x*g.scale, float32(g.center.Y)+y*g.scale)
}

func (g grid) point(x, y float32) image.Point {
	return image.Pt(g.center.X+g.px(x), g.center.Y+g.px(y))
}

func fillEllipse(gtx layout.Context, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Ellipse(r).Op(gtx.Ops))
}

func fillCircle(gtx layout.Context, col color.NRGBA, center image.Point, radius int) {
	kit.FillRounded(gtx, col, image.Rect(
		center.X-radius, center.Y-radius,
		center.X+radius, center.Y+radius,
	), radius)
}

func fillGradient(gtx layout.Context, r image.Rectangle, radius int, top, bottom color.NRGBA) {
	if r.Empty() {
		return
	}
	stack := clip.UniformRRect(r, radius).Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(r.Min.X), float32(r.Min.Y)),
		Color1: top,
		Stop2:  f32.Pt(float32(r.Max.X), float32(r.Max.Y)),
		Color2: bottom,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

func fillPolygon(gtx layout.Context, col color.NRGBA, points []f32.Point) {
	if len(points) < 3 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(points[0])
	for _, point := range points[1:] {
		path.LineTo(point)
	}
	path.Close()
	stack := clip.Outline{Path: path.End()}.Op().Push(gtx.Ops)
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

func strokeArc(gtx layout.Context, center f32.Point, radius, start, sweep, width float32, col color.NRGBA) {
	if radius <= 0 || width <= 0 || sweep == 0 {
		return
	}
	steps := max(2, int(math.Ceil(math.Abs(float64(sweep))/(2*math.Pi)*96)))
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(arcPoint(center, radius, start))
	for step := 1; step <= steps; step++ {
		path.LineTo(arcPoint(center, radius, start+sweep*float32(step)/float32(steps)))
	}
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

func strokeCircle(gtx layout.Context, center f32.Point, radius, width float32, col color.NRGBA) {
	strokeArc(gtx, center, radius, 0, 2*math.Pi, width, col)
}

func arcPoint(center f32.Point, radius, angle float32) f32.Point {
	return f32.Pt(
		center.X+radius*float32(math.Cos(float64(angle))),
		center.Y+radius*float32(math.Sin(float64(angle))),
	)
}

// drawRing paints the Loading spinner: a faint full track plus a bright arc.
func drawRing(gtx layout.Context, g grid, phase float64, col color.NRGBA) {
	center := g.pt(0, 0)
	radius := 18.4 * g.scale
	width := max(float32(1.6), float32(g.px(1.8)))
	strokeCircle(gtx, center, radius, width, kit.WithAlpha(col, 0x33))
	period := loadingRingPeriod.Seconds()
	turn := math.Mod(phase, period) / period
	strokeArc(gtx, center, radius, float32(turn*2*math.Pi)-math.Pi/2, float32(1.5*math.Pi), width, col)
}

func drawTail(gtx layout.Context, g grid, col color.NRGBA) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(g.pt(6, 12))
	path.CubeTo(g.pt(16.5, 13.5), g.pt(19, 5), g.pt(15, 0))
	stack := clip.Stroke{Path: path.End(), Width: float32(max(1, g.px(2.2)))}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

func drawArms(gtx layout.Context, g grid, c appearance.MascotColors, left, right f32.Point) {
	width := max(float32(1.5), float32(g.px(2.1)))
	strokeLine(gtx, g.pt(-6.5, 7.5), left, width, c.HairShadow)
	strokeLine(gtx, g.pt(6.5, 7.5), right, width, c.HairShadow)
}

func drawBody(gtx layout.Context, g grid, c appearance.MascotColors) {
	body := g.rect(-8.5, 4.5, 8.5, 16.5)
	fillGradient(gtx, body, g.px(5), c.Hair, c.Dress)
	kit.FillRounded(gtx, c.DressLight, g.rect(-7.2, 12, 7.2, 12.9), g.px(0.45))
}

func drawHead(gtx layout.Context, g grid, c appearance.MascotColors) {
	fillEllipse(gtx, c.Hair, g.rect(-12.2, -13.5, 12.2, 6.5))
	fillPolygon(gtx, c.Hair, []f32.Point{
		g.pt(-11.5, -8), g.pt(-14, -18.5), g.pt(-4, -12.5), g.pt(-10.8, -5.6),
	})
	fillPolygon(gtx, c.Hair, []f32.Point{
		g.pt(11.5, -8), g.pt(14, -18.5), g.pt(4, -12.5), g.pt(10.8, -5.6),
	})
	fillPolygon(gtx, c.EarInner, []f32.Point{
		g.pt(-10.2, -9.5), g.pt(-12.4, -16), g.pt(-6.3, -12.2),
	})
	fillPolygon(gtx, c.EarInner, []f32.Point{
		g.pt(10.2, -9.5), g.pt(12.4, -16), g.pt(6.3, -12.2),
	})

	fillEllipse(gtx, c.Face, g.rect(-8.2, -8.7, 8.2, 5.2))
	kit.FillRounded(gtx, c.Paw, g.rect(-3.6, 4.9, 3.6, 6.6), g.px(0.9))
	fillCircle(gtx, c.Hair, image.Pt(g.center.X, g.center.Y+g.px(5.8)), max(1, g.px(0.5)))

	fillEllipse(gtx, c.Hair, g.rect(-9.5, -7.5, -5.4, -0.5))
	fillEllipse(gtx, c.Hair, g.rect(5.4, -7.5, 9.5, -0.5))
	fillEllipse(gtx, c.Hair, g.rect(-7.6, -10.2, -0.4, -3.1))
	fillEllipse(gtx, c.Hair, g.rect(-0.6, -10.2, 7.6, -3.1))
	fillEllipse(gtx, c.Hair, g.rect(-4.6, -11, 0.8, -4.2))
	fillEllipse(gtx, kit.WithAlpha(c.HairLight, 0x9a), g.rect(-7.2, -8.1, -3.6, -6.1))
}

func drawFace(gtx layout.Context, g grid, c appearance.MascotColors, pose moodPose, pal appearance.MascotPalette) {
	for _, side := range []float32{-1, 1} {
		x := side * 3.5
		if pose.panic {
			fillEllipse(gtx, c.HairLight, g.rect(x-1.5, -3.3, x+1.5, 2))
			fillEllipse(gtx, pal.Eye, g.rect(x-0.6, -1.8, x+0.6, 0.9))
		} else {
			eye := g.rect(x-1.15, -2.5, x+1.15, 1.2)
			fillEllipse(gtx, pal.Eye, eye)
			fillCircle(gtx, pal.EyeHighlight, image.Pt(
				eye.Min.X+g.px(0.8), eye.Min.Y+g.px(0.85),
			), max(1, g.px(0.48)))
		}
		fillEllipse(gtx, c.Blush, g.rect(side*6.2-1.2, 1.6, side*6.2+1.2, 2.8))
	}

	switch {
	case pose.panic:
		fillEllipse(gtx, pal.Eye, g.rect(-1.6, 2.3, 1.6, 4.7))
	case pose.displeased:
		brow := max(1, float32(g.px(0.65)))
		strokeLine(gtx, g.pt(-5.2, -3.1), g.pt(-2, -2.3), brow, pal.Eye)
		strokeLine(gtx, g.pt(5.2, -3.1), g.pt(2, -2.3), brow, pal.Eye)
		strokeLine(gtx, g.pt(-1.7, 4.1), g.pt(0, 3.1), brow, pal.Eye)
		strokeLine(gtx, g.pt(0, 3.1), g.pt(1.7, 4.1), brow, pal.Eye)
	default:
		fillEllipse(gtx, pal.Eye, g.rect(-1.1, 2.8, 1.1, 3.9))
		lip := max(1, float32(g.px(0.45)))
		strokeLine(gtx, g.pt(-1.2, 3.2), g.pt(-0.4, 4.3), lip, c.Face)
		strokeLine(gtx, g.pt(1.2, 3.2), g.pt(0.4, 4.3), lip, c.Face)
	}
}

func drawPaw(gtx layout.Context, g grid, center f32.Point, paw, toes color.NRGBA) {
	radius := max(2, int(math.Round(float64(3*g.scale))))
	base := image.Pt(int(math.Floor(float64(center.X))), int(math.Floor(float64(center.Y))))
	frac := f32.Pt(center.X-float32(base.X), center.Y-float32(base.Y))
	stack := op.Affine(f32.Affine2D{}.Offset(frac)).Push(gtx.Ops)
	defer stack.Pop()
	fillCircle(gtx, paw, base, radius)
	pad := max(1, int(math.Round(float64(0.7*g.scale))))
	toe := kit.WithAlpha(toes, 0xb0)
	spread := int(math.Round(float64(1.1 * g.scale)))
	fillCircle(gtx, toe, image.Pt(base.X-spread, base.Y+pad), pad)
	fillCircle(gtx, toe, image.Pt(base.X+spread, base.Y+pad), pad)
}

// alertHands returns the mirrored paw centers for an Alert pose. together is
// the eased progress from 0 (hands apart) to 1 (hands on the chest).
func alertHands(pt func(x, y float32) f32.Point, together float32) (left, right f32.Point) {
	x := lerp(alertHandRestX, alertHandTogetherX, together)
	y := lerp(alertHandRestY, alertHandTogetherY, together)
	return pt(-x, y), pt(x, y)
}

func lerp(a, b, t float32) float32 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return a + (b-a)*t
}

func smoothStep(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}
