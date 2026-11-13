package kit

import (
	"image"
	"image/color"

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
