package shell

import (
	"image"
	"image/color"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/resource"
)

// Chrome geometry, in logical pixels.
const (
	headerHeight      = 48
	footerHeight      = 42
	controlWidth      = 30
	controlGap        = 12
	controlIconSize   = 24
	controlsInset     = 12
	logoInset         = 16
	logoSize          = 25
	labelGap          = 10
	contentGap        = 14
	versionGap        = 14
	badgeHeight       = 20
	badgePaddingX     = 7
	markRadius        = 6
	markInset         = 3
	siteChevronSize   = 8
	siteChevronGap    = 10
	siteChevronStroke = 1.7
	siteMenuOverhang  = 4
	resizeInset       = 6
)

// accentTintAlpha dims AccentMuted behind the brand mark.
const accentTintAlpha uint8 = 0x40

// layout holds every measurement of one frame.
type layout struct {
	size image.Point

	border        int
	inner         image.Rectangle
	headerDivider int
	contentTop    int
	footerDivider int
	footerTop     int
	footerBottom  int
	corner        int
	centerY       int

	logo      image.Rectangle
	titleX    int
	minimize  image.Rectangle
	maximize  image.Rectangle
	close     image.Rectangle
	trigger   image.Rectangle
	chevron   image.Rectangle
	versionX  int
	maximized bool
}

// contentRect returns the drawable area between the header and the footer.
func (l layout) contentRect() image.Rectangle {
	if l.inner.Empty() || l.headerDivider >= l.footerDivider {
		return image.Rectangle{}
	}
	return image.Rect(l.inner.Min.X, l.contentTop, l.inner.Max.X, l.footerDivider)
}

// footerRect returns the band where a screen would draw footer controls.
func (l layout) footerRect() image.Rectangle {
	if l.inner.Empty() || l.footerBottom <= l.footerTop {
		return image.Rectangle{}
	}
	return image.Rect(l.inner.Min.X, l.footerTop, l.inner.Max.X, l.footerBottom)
}

// computeLayout measures the chrome for the current widget size.
func (s *Shell) computeLayout() layout {
	size := image.Pt(s.widget.Width(), s.widget.Height())
	l := layout{size: size, border: 1}
	if size.X > 2*l.border && size.Y > 2*l.border {
		l.inner = image.Rect(l.border, l.border, size.X-l.border, size.Y-l.border)
	}
	l.headerDivider = l.border + headerHeight - 1
	l.contentTop = l.headerDivider + 1
	l.footerBottom = size.Y - l.border - 1
	l.footerTop = l.footerBottom - footerHeight
	l.footerDivider = l.footerTop - 1
	l.centerY = (l.border + l.headerDivider) / 2
	l.maximized = s.widget.IsMaximized()
	if !l.maximized {
		l.corner = int(appearance.RadiusS)
	}

	l.logo = image.Rect(logoInset, l.centerY-logoSize/2, logoInset+logoSize, l.centerY-logoSize/2+logoSize)
	l.titleX = l.logo.Max.X + labelGap

	right := size.X - l.border - controlsInset
	l.close = image.Rect(right-controlWidth, 0, right, l.headerDivider)
	l.maximize = l.close.Add(image.Pt(-(controlWidth + controlGap), 0))
	l.minimize = l.maximize.Add(image.Pt(-(controlWidth + controlGap), 0))

	site := s.site.Value()
	app := s.appearance.Value()
	width := func(value string) int {
		return kit.MeasureText(app, appearance.RoleLabel, value).X
	}
	if len(catalog.Sites()) > 1 {
		anchor := width(site.String())
		for _, other := range catalog.Sites() {
			if w := width(other.String()); w > anchor {
				anchor = w
			}
		}
		maxWidth := max(1, l.minimize.Min.X-l.titleX-(contentGap+siteChevronGap+siteChevronSize))
		anchor = min(anchor, maxWidth)
		left := l.titleX + anchor + siteChevronGap
		top := l.centerY - siteChevronSize/2
		l.chevron = image.Rect(left, top, left+siteChevronSize, top+siteChevronSize)
		l.trigger = image.Rect(l.logo.Min.X, 0, l.chevron.Max.X+siteMenuOverhang, l.headerDivider)
		l.versionX = l.chevron.Max.X + versionGap
	} else {
		l.trigger = image.Rect(l.logo.Min.X, 0, l.titleX+width(site.String()), l.headerDivider)
		l.versionX = l.titleX + width(site.String()) + versionGap
	}
	return l
}

// paintFrame draws the rounded border, the inner surface and the two bands.
func (s *Shell) paintFrame(p *qt.QPainter, l layout) {
	palette := s.palette()
	kit.FillRounded(p, palette.Border, image.Rect(0, 0, l.size.X, l.size.Y), float64(l.corner))
	if l.inner.Empty() {
		return
	}
	p.Save()
	clip := roundedPath(l.inner, float64(l.corner))
	p.SetClipPath(clip)
	kit.FillRect(p, palette.Canvas, l.inner)
	kit.FillRect(p, palette.Surface, image.Rect(l.inner.Min.X, l.inner.Min.Y, l.inner.Max.X, l.headerDivider))
	kit.FillRect(p, palette.Divider, image.Rect(l.inner.Min.X, l.headerDivider, l.inner.Max.X, l.contentTop))
	kit.FillRect(p, palette.Divider, image.Rect(l.inner.Min.X, l.footerDivider, l.inner.Max.X, l.footerTop))
	kit.FillRect(p, palette.Surface, image.Rect(l.inner.Min.X, l.footerTop, l.inner.Max.X, l.footerBottom))
	kit.FillRect(p, palette.Canvas, image.Rect(l.inner.Min.X, l.footerBottom, l.inner.Max.X, l.inner.Max.Y))
	p.Restore()
	clip.Delete()
}

// paintHeader draws the brand, the version and the window controls.
func (s *Shell) paintHeader(p *qt.QPainter, l layout) {
	palette := s.palette()
	app := s.appearance.Value()
	s.paintSiteMark(p, l.logo, palette)
	drawTextLine(p, app, appearance.RoleLabel, s.site.Value().String(), palette.Text, l.titleX, l.centerY)
	if !l.chevron.Empty() {
		paintChevron(p, l.chevron, palette.ControlIcon, s.menuOpen)
	}
	if s.version != "" {
		s.paintVersion(p, l, palette)
	}

	s.paintControl(p, l.minimize, resource.IconMinimize,
		s.controlColor(palette.ControlIcon, palette.ControlIconHover, RegionMinimize))
	s.paintControl(p, l.maximize, s.maximizeIcon(),
		s.controlColor(palette.ControlIcon, palette.ControlIconHover, RegionMaximize))
	s.paintControl(p, l.close, resource.IconClose,
		s.controlColor(palette.ControlIcon, palette.Danger, RegionClose))
}

// maximizeIcon picks the restore glyph while the window is maximized.
func (s *Shell) maximizeIcon() resource.IconKey {
	if s.widget.IsMaximized() {
		return resource.IconUnmaximize
	}
	return resource.IconMaximize
}

// controlColor returns the hover color of a window control.
func (s *Shell) controlColor(normal, hover color.NRGBA, region Region) color.NRGBA {
	if s.hover == region {
		return hover
	}
	return normal
}

// paintControl draws one window-control glyph centered in its hitbox.
func (s *Shell) paintControl(p *qt.QPainter, bounds image.Rectangle, key resource.IconKey, col color.NRGBA) {
	if bounds.Empty() {
		return
	}
	size := min(controlIconSize, min(bounds.Dx(), bounds.Dy()))
	icon := image.Rect(0, 0, size, size).Add(image.Pt(
		bounds.Min.X+(bounds.Dx()-size)/2,
		bounds.Min.Y+(bounds.Dy()-size)/2,
	))
	kit.Icon(p, s.resource.IconPath(key), icon, col)
}

// paintSiteMark draws the rounded site tile with the torii glyph.
func (s *Shell) paintSiteMark(p *qt.QPainter, r image.Rectangle, palette appearance.Palette) {
	if r.Empty() {
		return
	}
	line := max(1, int(appearance.Hairline))
	radius := float64(markRadius)
	tint := palette.AccentMuted
	tint.A = accentTintAlpha
	kit.FillRounded(p, tint, r, radius)
	inner := r.Inset(line)
	kit.FillRounded(p, palette.SurfaceAlt, inner, max(0, radius-float64(line)))
	kit.Icon(p, s.resource.IconPath(resource.IconTorii), inner.Inset(markInset), palette.Accent)
}

// paintVersion draws the accent version pill.
func (s *Shell) paintVersion(p *qt.QPainter, l layout, palette appearance.Palette) {
	app := s.appearance.Value()
	size := kit.MeasureText(app, appearance.RoleCaption, s.version)
	if size.X <= 0 {
		return
	}
	line := max(1, int(appearance.Hairline))
	pad := badgePaddingX
	pill := image.Rect(
		l.versionX,
		l.centerY-badgeHeight/2,
		l.versionX+size.X+2*(pad+line),
		l.centerY+(badgeHeight+1)/2,
	)
	if pill.Max.X > l.minimize.Min.X-contentGap {
		return
	}
	radius := float64(badgeHeight) / 2
	kit.FillRounded(p, palette.Edge, pill, radius)
	kit.FillRounded(p, palette.Badge, pill.Inset(line), max(0, radius-float64(line)))
	drawTextLine(p, app, appearance.RoleCaption, s.version, palette.Ink, pill.Min.X+line+pad, l.centerY)
}

// paintChevron draws the switcher arrow, pointing down when closed and up when
// open.
func paintChevron(p *qt.QPainter, r image.Rectangle, col color.NRGBA, open bool) {
	if r.Empty() {
		return
	}
	x, y := float64(r.Min.X), float64(r.Min.Y)
	w, h := float64(r.Dx()), float64(r.Dy())
	path := qt.NewQPainterPath()
	if open {
		path.MoveTo(qt.NewQPointF3(x+w*0.12, y+h*0.64))
		path.LineTo(qt.NewQPointF3(x+w*0.5, y+h*0.3))
		path.LineTo(qt.NewQPointF3(x+w*0.88, y+h*0.64))
	} else {
		path.MoveTo(qt.NewQPointF3(x+w*0.12, y+h*0.36))
		path.LineTo(qt.NewQPointF3(x+w*0.5, y+h*0.7))
		path.LineTo(qt.NewQPointF3(x+w*0.88, y+h*0.36))
	}
	pen := qt.NewQPen3(qcolor(col))
	pen.SetWidthF(siteChevronStroke)
	pen.SetCapStyle(qt.RoundCap)
	pen.SetJoinStyle(qt.RoundJoin)
	p.SetPenWithPen(pen)
	p.SetBrushWithStyle(qt.NoBrush)
	p.DrawPath(path)
	pen.Delete()
	path.Delete()
}

// drawTextLine draws one vertically centered line and returns its width.
func drawTextLine(p *qt.QPainter, app appearance.Appearance, role appearance.TextRole, value string, col color.NRGBA, x, centerY int) int {
	if value == "" {
		return 0
	}
	size := kit.MeasureText(app, role, value)
	p.Save()
	p.Translate2(float64(x), float64(centerY-size.Y/2))
	kit.Text(p, app, role, value, col)
	p.Restore()
	return size.X
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
