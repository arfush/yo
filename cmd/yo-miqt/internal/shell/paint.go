package shell

import (
	"image"
	"image/color"
	"math"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// paint draws one complete frame.
func (s *Shell) paint(p *qt.QPainter) {
	p.SetRenderHints(qt.QPainter__Antialiasing | qt.QPainter__TextAntialiasing | qt.QPainter__SmoothPixmapTransform)
	l := s.computeLayout()
	s.paintFrame(p, l)
	s.paintHeader(p, l)
	if content := l.contentRect(); s.screen != nil && !content.Empty() {
		p.Save()
		p.SetClipRect2(content.Min.X, content.Min.Y, content.Dx(), content.Dy())
		s.screen.Paint(p, content, s.palette)
		p.Restore()
	}
	if footer := l.footerRect(); !footer.Empty() {
		if screen, ok := s.screen.(FooterScreen); ok {
			p.Save()
			p.SetClipRect2(footer.Min.X, footer.Min.Y, footer.Dx(), footer.Dy())
			screen.PaintFooter(p, footer, s.palette)
			p.Restore()
		}
	}
	if s.menuOpen {
		s.paintMenu(p, l)
	}
}

// paintFrame draws the rounded border, the inner surface and the two bands.
func (s *Shell) paintFrame(p *qt.QPainter, l layout) {
	palette := s.palette
	fillRound(p, image.Rect(0, 0, l.size.X, l.size.Y), float64(l.corner), palette.Border)
	if l.inner.Empty() {
		return
	}
	p.Save()
	clip := roundedPath(l.inner, float64(l.corner))
	p.SetClipPath(clip)
	fillRect(p, l.inner, palette.Canvas)
	fillRect(p, image.Rect(l.inner.Min.X, l.inner.Min.Y, l.inner.Max.X, l.headerDivider), palette.Surface)
	fillRect(p, image.Rect(l.inner.Min.X, l.headerDivider, l.inner.Max.X, l.contentTop), palette.Divider)
	fillRect(p, image.Rect(l.inner.Min.X, l.footerDivider, l.inner.Max.X, l.footerTop), palette.Divider)
	fillRect(p, image.Rect(l.inner.Min.X, l.footerTop, l.inner.Max.X, l.footerBottom), palette.Surface)
	fillRect(p, image.Rect(l.inner.Min.X, l.footerBottom, l.inner.Max.X, l.inner.Max.Y), palette.Canvas)
	p.Restore()
	clip.Delete()
}

// paintHeader draws the brand, the version and the window controls.
func (s *Shell) paintHeader(p *qt.QPainter, l layout) {
	palette := s.palette
	s.paintSiteMark(p, l.logo, palette)
	s.drawText(p, s.fonts.bold, palette.Text, l.titleX, l.centerY, s.site.String())
	if !l.chevron.Empty() {
		s.paintChevron(p, l.chevron, palette.ControlIcon, s.menuOpen)
	}
	if s.version != "" {
		s.paintVersion(p, l, palette)
	}

	s.paintControl(p, l.minimize, "minimize", s.controlColor(palette.ControlIcon, palette.ControlIconHover, RegionMinimize))
	s.paintControl(p, l.maximize, s.maximizeIcon(), s.controlColor(palette.ControlIcon, palette.ControlIconHover, RegionMaximize))
	s.paintControl(p, l.close, "close", s.controlColor(palette.ControlIcon, palette.Danger, RegionClose))
}

// maximizeIcon picks the restore glyph while the window is maximized.
func (s *Shell) maximizeIcon() string {
	if s.widget.IsMaximized() {
		return "unmaximize"
	}
	return "maximize"
}

// controlColor returns the hover color of a window control.
func (s *Shell) controlColor(normal, hover color.NRGBA, region Region) color.NRGBA {
	if s.hover == region {
		return hover
	}
	return normal
}

// paintControl draws one window-control glyph centered in its hitbox.
func (s *Shell) paintControl(p *qt.QPainter, bounds image.Rectangle, name string, col color.NRGBA) {
	if bounds.Empty() {
		return
	}
	size := min(controlIconSize, min(bounds.Dx(), bounds.Dy()))
	icon := image.Rect(0, 0, size, size).Add(image.Pt(
		bounds.Min.X+(bounds.Dx()-size)/2,
		bounds.Min.Y+(bounds.Dy()-size)/2,
	))
	s.drawIcon(p, name, icon, col)
}

// paintSiteMark draws the rounded site tile with the torii glyph.
func (s *Shell) paintSiteMark(p *qt.QPainter, r image.Rectangle, palette appearance.Palette) {
	if r.Empty() {
		return
	}
	line := 1
	radius := float64(markRadius)
	tint := palette.AccentMuted
	tint.A = accentTintAlpha
	fillRound(p, r, radius, tint)
	inner := r.Inset(line)
	fillRound(p, inner, max(0, radius-float64(line)), palette.SurfaceAlt)
	s.drawIcon(p, "torii", inner.Inset(markInset), palette.Accent)
}

// paintVersion draws the accent version pill.
func (s *Shell) paintVersion(p *qt.QPainter, l layout, palette appearance.Palette) {
	width := s.textWidth(s.fonts.medium, s.version)
	if width <= 0 {
		return
	}
	line := 1
	pad := badgePaddingX
	pill := image.Rect(
		l.versionX,
		l.centerY-badgeHeight/2,
		l.versionX+width+2*(pad+line),
		l.centerY+(badgeHeight+1)/2,
	)
	if pill.Max.X > l.minimize.Min.X-contentGap {
		return
	}
	fillRound(p, pill, float64(badgeHeight/2), palette.Edge)
	fillRound(p, pill.Inset(line), float64(badgeHeight/2-line), palette.Badge)
	s.drawText(p, s.fonts.medium, palette.Ink, pill.Min.X+line+pad, l.centerY, s.version)
}

// paintChevron draws the switcher arrow, pointing down when closed and up when
// open.
func (s *Shell) paintChevron(p *qt.QPainter, r image.Rectangle, col color.NRGBA, open bool) {
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

// paintMenu draws the anchored site switcher above the content.
func (s *Shell) paintMenu(p *qt.QPainter, l layout) {
	menu, _, rows, sites := s.menuLayout(l)
	if menu.Empty() || len(rows) == 0 {
		return
	}
	palette := s.palette
	line := 1
	fillRound(p, menu, float64(appearance.RadiusS), palette.Border)
	fillRound(p, menu.Inset(line), float64(appearance.RadiusS)-1, palette.Panel)
	for i, site := range sites {
		row := rows[i]
		if s.menuRowHover == i {
			fillRound(p, row, siteMenuItemRadius, palette.SurfaceAlt)
		}
		iconSize := logoSize
		top := row.Min.Y + (row.Dy()-iconSize)/2
		icon := image.Rect(row.Min.X+siteMenuItemPad, top, row.Min.X+siteMenuItemPad+iconSize, top+iconSize)
		s.paintSiteMark(p, icon, s.target[site])
		textX := row.Min.X + siteMenuItemPad + iconSize + siteMenuItemGapX
		s.drawText(p, s.fonts.bold, palette.Text, textX, row.Min.Y+row.Dy()/2, site.String())
	}
}

// drawText draws one vertically centered line.
func (s *Shell) drawText(p *qt.QPainter, f *qt.QFont, col color.NRGBA, x, centerY int, text string) {
	fm := qt.NewQFontMetrics2(f, s.widget.QPaintDevice)
	ascent, descent := fm.Ascent(), fm.Descent()
	fm.Delete()
	top := centerY - (ascent+descent)/2
	pen := qcolor(col)
	p.SetFont(f)
	p.SetPen(pen)
	p.DrawText3(x, top+ascent, text)
	pen.Delete()
}

// drawIcon draws one glyph from home.svg centered in its hitbox.
func (s *Shell) drawIcon(p *qt.QPainter, name string, r image.Rectangle, col color.NRGBA) {
	drawSVGIcon(p, name, r, col)
}

// drawTextAt draws one vertically centered line in widget coordinates.
func drawTextAt(p *qt.QPainter, f *qt.QFont, col color.NRGBA, x, centerY int, text string) {
	if text == "" {
		return
	}
	fm := qt.NewQFontMetricsF(f)
	ascent, descent := fm.Ascent(), fm.Descent()
	fm.Delete()
	top := float64(centerY) - (ascent+descent)/2
	pen := qcolor(col)
	p.SetFont(f)
	p.SetPen(pen)
	p.DrawText3(x, int(math.Round(top+ascent)), text)
	pen.Delete()
}

// measureText returns the rounded advance of one line.
func measureText(f *qt.QFont, text string) int {
	if text == "" {
		return 0
	}
	fm := qt.NewQFontMetricsF(f)
	width := int(math.Ceil(fm.HorizontalAdvance(text)))
	fm.Delete()
	return width
}

// fillRect fills an integer rectangle.
func fillRect(p *qt.QPainter, r image.Rectangle, col color.NRGBA) {
	if r.Empty() {
		return
	}
	c := qcolor(col)
	p.FillRect5(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), c)
	c.Delete()
}

// fillRound fills a rounded rectangle.
func fillRound(p *qt.QPainter, r image.Rectangle, radius float64, col color.NRGBA) {
	if r.Empty() {
		return
	}
	if radius <= 0 {
		fillRect(p, r, col)
		return
	}
	path := roundedPath(r, radius)
	brush := qt.NewQBrush3(qcolor(col))
	p.FillPath(path, brush)
	brush.Delete()
	path.Delete()
}

// roundedPath builds a rounded rectangle path.
func roundedPath(r image.Rectangle, radius float64) *qt.QPainterPath {
	rect := qt.NewQRectF4(float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()))
	path := qt.NewQPainterPath()
	path.AddRoundedRect(rect, radius, radius)
	rect.Delete()
	return path
}

// qcolor converts a palette token into a Qt color.
func qcolor(c color.NRGBA) *qt.QColor {
	return qt.NewQColor11(int(c.R), int(c.G), int(c.B), int(c.A))
}
