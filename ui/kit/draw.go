package kit

import (
	"image"
	"image/color"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/resource"
)

// WithAlpha returns col with its alpha replaced, so a token can be used as a
// dim tint over the surface below.
func WithAlpha(col color.NRGBA, alpha uint8) color.NRGBA {
	col.A = alpha
	return col
}

// FillRect paints col over the axis-aligned rectangle r.
func FillRect(p *qt.QPainter, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	c := qcolor(col)
	p.FillRect5(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), c)
	c.Delete()
}

// FillRounded paints col over r with uniformly rounded corners. The radius is
// used as given: callers keep it within half the shorter side.
func FillRounded(p *qt.QPainter, col color.NRGBA, r image.Rectangle, radius float64) {
	if r.Empty() {
		return
	}
	if radius <= 0 {
		FillRect(p, col, r)
		return
	}
	path := roundedPath(r, radius)
	brush := qt.NewQBrush3(qcolor(col))
	p.FillPath(path, brush)
	brush.Delete()
	path.Delete()
}

// Icon fills icon in the square at r.Min with the shorter side of r as the
// size. An empty path or an empty rectangle draws nothing.
func Icon(p *qt.QPainter, icon resource.Path, r image.Rectangle, col color.NRGBA) {
	if icon == "" || r.Empty() {
		return
	}
	size := min(r.Dx(), r.Dy())
	if size <= 0 {
		return
	}
	scale := float64(size) / iconBox

	p.Save()
	p.Translate2(float64(r.Min.X), float64(r.Min.Y))
	if scale != 1 {
		p.Scale(scale, scale)
	}
	path := iconPath(icon)
	brush := qt.NewQBrush3(qcolor(col))
	p.SetPenWithStyle(qt.NoPen)
	p.SetBrush(brush)
	p.DrawPath(path)
	brush.Delete()
	p.Restore()
}

// roundedPath builds a rounded rectangle path.
func roundedPath(r image.Rectangle, radius float64) *qt.QPainterPath {
	path := qt.NewQPainterPath()
	path.AddRoundedRect2(
		float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()),
		radius, radius,
	)
	return path
}

// qcolor converts a palette token into a Qt color.
func qcolor(c color.NRGBA) *qt.QColor {
	return qt.NewQColor11(int(c.R), int(c.G), int(c.B), int(c.A))
}
