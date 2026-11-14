package kit

import (
	"image"
	"image/color"
	"math"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/resource"
)

// WithAlpha returns col with its alpha replaced, so a token can be used as a
// dim tint over the surface below.
func WithAlpha(col color.NRGBA, alpha uint8) color.NRGBA {
	col.A = alpha
	return col
}

// FillRect paints col over the axis-aligned rectangle r.
func FillRect(p *qt6.QPainter, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	c := qcolor(col)
	p.FillRect5(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), c)
	c.Delete()
}

// FillRounded paints col over r with uniformly rounded corners. The radius is
// used as given: callers keep it within half the shorter side.
func FillRounded(p *qt6.QPainter, col color.NRGBA, r image.Rectangle, radius float64) {
	if r.Empty() {
		return
	}
	if radius <= 0 {
		FillRect(p, col, r)
		return
	}
	path := roundedPath(r, radius)
	brush := qt6.NewQBrush3(qcolor(col))
	p.FillPath(path, brush)
	brush.Delete()
	path.Delete()
}

// Icon fills path in the square at r.Min with the shorter side of r as the
// size. The path is expected in resource.IconBox coordinates and stays owned
// by resource. A nil path or an empty rectangle draws nothing.
func Icon(p *qt6.QPainter, path *qt6.QPainterPath, r image.Rectangle, col color.NRGBA) {
	if path == nil || r.Empty() {
		return
	}
	size := min(r.Dx(), r.Dy())
	if size <= 0 {
		return
	}
	scale := float64(size) / resource.IconBox

	p.Save()
	p.Translate2(float64(r.Min.X), float64(r.Min.Y))
	if scale != 1 {
		p.Scale(scale, scale)
	}
	brush := qt6.NewQBrush3(qcolor(col))
	p.SetPenWithStyle(qt6.NoPen)
	p.SetBrush(brush)
	p.DrawPath(path)
	brush.Delete()
	p.Restore()
}

// roundedPath builds a rounded rectangle path.
func roundedPath(r image.Rectangle, radius float64) *qt6.QPainterPath {
	path := qt6.NewQPainterPath()
	path.AddRoundedRect2(
		float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()),
		radius, radius,
	)
	return path
}

// qcolor converts a palette token into a Qt color.
func qcolor(c color.NRGBA) *qt6.QColor {
	return qt6.NewQColor11(int(c.R), int(c.G), int(c.B), int(c.A))
}

// point is a design-space coordinate.
type point struct {
	X, Y float32
}

// pt builds a design-space point.
func pt(x, y float32) point { return point{X: x, Y: y} }

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
	FillRounded(p, col, image.Rect(
		center.X-radius, center.Y-radius,
		center.X+radius, center.Y+radius,
	), float64(radius))
}

func fillGradient(p *qt6.QPainter, r image.Rectangle, radius int, top, bottom color.NRGBA) {
	if r.Empty() {
		return
	}
	path := roundedPath(r, float64(radius))
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
