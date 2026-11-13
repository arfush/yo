package mascot

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/mappu/miqt/qt6"

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

// point is a design-space coordinate.
type point struct {
	X, Y float32
}

// pt builds a design-space point.
func pt(x, y float32) point { return point{X: x, Y: y} }

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
func (s scene) paint(p *qt6.QPainter, jitter float32) {
	c := s.style.colors
	bob := float32(math.Sin(s.phase*2*math.Pi/s.style.period.Seconds())) * s.style.amplitude

	p.Save()
	p.Translate2(float64(jitter*s.g.scale), float64(bob*s.g.scale))
	defer p.Restore()

	if s.style.loading {
		drawRing(p, s.g, s.phase, c.HairLight)
	}
	drawTail(p, s.g, c.HairShadow)
	drawBody(p, s.g, c)
	drawHead(p, s.g, c)
	drawFace(p, s.g, c, s.pose, s.m.palette)
	s.m.hat.draw(p, wardrobe{g: s.g, pal: s.m.palette, hats: hatColors()})
	left, right := s.hands()
	drawArms(p, s.g, c, left, right)
	drawPaw(p, s.g, left, c.Paw, c.HairShadow)
	drawPaw(p, s.g, right, c.Paw, c.HairShadow)
}

// hands returns the paw centers for this frame. Alert folds both hands onto the
// chest; Panic presses them to the cheeks with a tremble.
func (s scene) hands() (point, point) {
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

func (g grid) pt(x, y float32) point {
	return pt(float32(g.center.X)+x*g.scale, float32(g.center.Y)+y*g.scale)
}

func (g grid) point(x, y float32) image.Point {
	return image.Pt(g.center.X+g.px(x), g.center.Y+g.px(y))
}

func fillEllipse(p *qt6.QPainter, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	brush := qt6.NewQBrush3(qcolor(col))
	p.Save()
	p.SetPenWithStyle(qt6.NoPen)
	p.SetBrush(brush)
	p.DrawEllipse2(r.Min.X, r.Min.Y, r.Dx(), r.Dy())
	p.Restore()
	brush.Delete()
}

func fillCircle(p *qt6.QPainter, col color.NRGBA, center image.Point, radius int) {
	kit.FillRounded(p, col, image.Rect(
		center.X-radius, center.Y-radius,
		center.X+radius, center.Y+radius,
	), float64(radius))
}

func fillGradient(p *qt6.QPainter, r image.Rectangle, radius int, top, bottom color.NRGBA) {
	if r.Empty() {
		return
	}
	path := roundedRectPath(r, float64(radius))
	gradient := qt6.NewQLinearGradient3(
		float64(r.Min.X), float64(r.Min.Y),
		float64(r.Max.X), float64(r.Max.Y),
	)
	topColor, bottomColor := qcolor(top), qcolor(bottom)
	gradient.SetColorAt(0, topColor)
	gradient.SetColorAt(1, bottomColor)
	brush := qt6.NewQBrush10(gradient.QGradient)
	p.FillPath(path, brush)

	brush.Delete()
	topColor.Delete()
	bottomColor.Delete()
	gradient.Delete()
	path.Delete()
}

func fillPolygon(p *qt6.QPainter, col color.NRGBA, points []point) {
	if len(points) < 3 {
		return
	}
	path := qt6.NewQPainterPath()
	path.MoveTo2(float64(points[0].X), float64(points[0].Y))
	for _, vertex := range points[1:] {
		path.LineTo2(float64(vertex.X), float64(vertex.Y))
	}
	path.CloseSubpath()
	brush := qt6.NewQBrush3(qcolor(col))
	p.FillPath(path, brush)
	brush.Delete()
	path.Delete()
}

func strokeLine(p *qt6.QPainter, from, to point, width float32, col color.NRGBA) {
	path := qt6.NewQPainterPath()
	path.MoveTo2(float64(from.X), float64(from.Y))
	path.LineTo2(float64(to.X), float64(to.Y))
	strokePath(p, path, width, col)
	path.Delete()
}

func strokeArc(p *qt6.QPainter, center point, radius, start, sweep, width float32, col color.NRGBA) {
	if radius <= 0 || width <= 0 || sweep == 0 {
		return
	}
	steps := max(2, int(math.Ceil(math.Abs(float64(sweep))/(2*math.Pi)*96)))
	path := qt6.NewQPainterPath()
	first := arcPoint(center, radius, start)
	path.MoveTo2(float64(first.X), float64(first.Y))
	for step := 1; step <= steps; step++ {
		vertex := arcPoint(center, radius, start+sweep*float32(step)/float32(steps))
		path.LineTo2(float64(vertex.X), float64(vertex.Y))
	}
	strokePath(p, path, width, col)
	path.Delete()
}

func strokeCircle(p *qt6.QPainter, center point, radius, width float32, col color.NRGBA) {
	strokeArc(p, center, radius, 0, 2*math.Pi, width, col)
}

func arcPoint(center point, radius, angle float32) point {
	return pt(
		center.X+radius*float32(math.Cos(float64(angle))),
		center.Y+radius*float32(math.Sin(float64(angle))),
	)
}

// strokePath outlines path with a flat-capped, mitered pen.
func strokePath(p *qt6.QPainter, path *qt6.QPainterPath, width float32, col color.NRGBA) {
	if width <= 0 {
		return
	}
	pen := qt6.NewQPen3(qcolor(col))
	pen.SetWidthF(float64(width))
	pen.SetCapStyle(qt6.FlatCap)
	pen.SetJoinStyle(qt6.MiterJoin)
	p.Save()
	p.SetPenWithPen(pen)
	p.SetBrushWithStyle(qt6.NoBrush)
	p.DrawPath(path)
	p.Restore()
	pen.Delete()
}

// roundedRectPath builds a rounded rectangle path in device pixels.
func roundedRectPath(r image.Rectangle, radius float64) *qt6.QPainterPath {
	path := qt6.NewQPainterPath()
	path.AddRoundedRect2(
		float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()),
		radius, radius,
	)
	return path
}

// qcolor converts a color into a Qt color.
func qcolor(c color.NRGBA) *qt6.QColor {
	return qt6.NewQColor11(int(c.R), int(c.G), int(c.B), int(c.A))
}

// drawRing paints the Loading spinner: a faint full track plus a bright arc.
func drawRing(p *qt6.QPainter, g grid, phase float64, col color.NRGBA) {
	center := g.pt(0, 0)
	radius := 18.4 * g.scale
	width := max(float32(1.6), float32(g.px(1.8)))
	strokeCircle(p, center, radius, width, kit.WithAlpha(col, 0x33))
	period := loadingRingPeriod.Seconds()
	turn := math.Mod(phase, period) / period
	strokeArc(p, center, radius, float32(turn*2*math.Pi)-math.Pi/2, float32(1.5*math.Pi), width, col)
}

func drawTail(p *qt6.QPainter, g grid, col color.NRGBA) {
	path := qt6.NewQPainterPath()
	from := g.pt(6, 12)
	path.MoveTo2(float64(from.X), float64(from.Y))
	control1, control2, end := g.pt(16.5, 13.5), g.pt(19, 5), g.pt(15, 0)
	path.CubicTo2(
		float64(control1.X), float64(control1.Y),
		float64(control2.X), float64(control2.Y),
		float64(end.X), float64(end.Y),
	)
	strokePath(p, path, float32(max(1, g.px(2.2))), col)
	path.Delete()
}

func drawArms(p *qt6.QPainter, g grid, c appearance.MascotColors, left, right point) {
	width := max(float32(1.5), float32(g.px(2.1)))
	strokeLine(p, g.pt(-6.5, 7.5), left, width, c.HairShadow)
	strokeLine(p, g.pt(6.5, 7.5), right, width, c.HairShadow)
}

func drawBody(p *qt6.QPainter, g grid, c appearance.MascotColors) {
	body := g.rect(-8.5, 4.5, 8.5, 16.5)
	fillGradient(p, body, g.px(5), c.Hair, c.Dress)
	kit.FillRounded(p, c.DressLight, g.rect(-7.2, 12, 7.2, 12.9), float64(g.px(0.45)))
}

func drawHead(p *qt6.QPainter, g grid, c appearance.MascotColors) {
	fillEllipse(p, c.Hair, g.rect(-12.2, -13.5, 12.2, 6.5))
	fillPolygon(p, c.Hair, []point{
		g.pt(-11.5, -8), g.pt(-14, -18.5), g.pt(-4, -12.5), g.pt(-10.8, -5.6),
	})
	fillPolygon(p, c.Hair, []point{
		g.pt(11.5, -8), g.pt(14, -18.5), g.pt(4, -12.5), g.pt(10.8, -5.6),
	})
	fillPolygon(p, c.EarInner, []point{
		g.pt(-10.2, -9.5), g.pt(-12.4, -16), g.pt(-6.3, -12.2),
	})
	fillPolygon(p, c.EarInner, []point{
		g.pt(10.2, -9.5), g.pt(12.4, -16), g.pt(6.3, -12.2),
	})

	fillEllipse(p, c.Face, g.rect(-8.2, -8.7, 8.2, 5.2))
	kit.FillRounded(p, c.Paw, g.rect(-3.6, 4.9, 3.6, 6.6), float64(g.px(0.9)))
	fillCircle(p, c.Hair, image.Pt(g.center.X, g.center.Y+g.px(5.8)), max(1, g.px(0.5)))

	fillEllipse(p, c.Hair, g.rect(-9.5, -7.5, -5.4, -0.5))
	fillEllipse(p, c.Hair, g.rect(5.4, -7.5, 9.5, -0.5))
	fillEllipse(p, c.Hair, g.rect(-7.6, -10.2, -0.4, -3.1))
	fillEllipse(p, c.Hair, g.rect(-0.6, -10.2, 7.6, -3.1))
	fillEllipse(p, c.Hair, g.rect(-4.6, -11, 0.8, -4.2))
	fillEllipse(p, kit.WithAlpha(c.HairLight, 0x9a), g.rect(-7.2, -8.1, -3.6, -6.1))
}

func drawFace(p *qt6.QPainter, g grid, c appearance.MascotColors, pose moodPose, pal appearance.MascotPalette) {
	for _, side := range []float32{-1, 1} {
		x := side * 3.5
		if pose.panic {
			fillEllipse(p, c.HairLight, g.rect(x-1.5, -3.3, x+1.5, 2))
			fillEllipse(p, pal.Eye, g.rect(x-0.6, -1.8, x+0.6, 0.9))
		} else {
			eye := g.rect(x-1.15, -2.5, x+1.15, 1.2)
			fillEllipse(p, pal.Eye, eye)
			fillCircle(p, pal.EyeHighlight, image.Pt(
				eye.Min.X+g.px(0.8), eye.Min.Y+g.px(0.85),
			), max(1, g.px(0.48)))
		}
		fillEllipse(p, c.Blush, g.rect(side*6.2-1.2, 1.6, side*6.2+1.2, 2.8))
	}

	switch {
	case pose.panic:
		fillEllipse(p, pal.Eye, g.rect(-1.6, 2.3, 1.6, 4.7))
	case pose.displeased:
		brow := max(1, float32(g.px(0.65)))
		strokeLine(p, g.pt(-5.2, -3.1), g.pt(-2, -2.3), brow, pal.Eye)
		strokeLine(p, g.pt(5.2, -3.1), g.pt(2, -2.3), brow, pal.Eye)
		strokeLine(p, g.pt(-1.7, 4.1), g.pt(0, 3.1), brow, pal.Eye)
		strokeLine(p, g.pt(0, 3.1), g.pt(1.7, 4.1), brow, pal.Eye)
	default:
		fillEllipse(p, pal.Eye, g.rect(-1.1, 2.8, 1.1, 3.9))
		lip := max(1, float32(g.px(0.45)))
		strokeLine(p, g.pt(-1.2, 3.2), g.pt(-0.4, 4.3), lip, c.Face)
		strokeLine(p, g.pt(1.2, 3.2), g.pt(0.4, 4.3), lip, c.Face)
	}
}

func drawPaw(p *qt6.QPainter, g grid, center point, paw, toes color.NRGBA) {
	radius := max(2, int(math.Round(float64(3*g.scale))))
	base := image.Pt(int(math.Floor(float64(center.X))), int(math.Floor(float64(center.Y))))
	fracX := center.X - float32(base.X)
	fracY := center.Y - float32(base.Y)

	p.Save()
	p.Translate2(float64(fracX), float64(fracY))
	fillCircle(p, paw, base, radius)
	pad := max(1, int(math.Round(float64(0.7*g.scale))))
	toe := kit.WithAlpha(toes, 0xb0)
	spread := int(math.Round(float64(1.1 * g.scale)))
	fillCircle(p, toe, image.Pt(base.X-spread, base.Y+pad), pad)
	fillCircle(p, toe, image.Pt(base.X+spread, base.Y+pad), pad)
	p.Restore()
}

// alertHands returns the mirrored paw centers for an Alert pose. together is
// the eased progress from 0 (hands apart) to 1 (hands on the chest).
func alertHands(at func(x, y float32) point, together float32) (left, right point) {
	x := lerp(alertHandRestX, alertHandTogetherX, together)
	y := lerp(alertHandRestY, alertHandTogetherY, together)
	return at(-x, y), at(x, y)
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
