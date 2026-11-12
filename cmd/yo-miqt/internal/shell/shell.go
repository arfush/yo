// Package shell draws the undecorated application window of yo-miqt: the
// border, header, footer and the site switcher. It mirrors the Gio shell in
// ui/shell and takes every color from ui/appearance.
package shell

import (
	"image"
	"math"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
)

// Window sizes. The designed size is also the smallest usable size, matching
// the Gio chrome: the chrome and the screens are verified at exactly this width
// and height.
const (
	InitialWidth  = 840
	InitialHeight = 550
	MinimumWidth  = InitialWidth
	MinimumHeight = InitialHeight
)

// Chrome geometry, in logical pixels. The values mirror ui/shell/chrome.go.
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

	siteMenuMinWidth   = 120
	siteMenuGap        = 6
	siteMenuMargin     = 4
	siteMenuPadding    = 4
	siteMenuItemHeight = 42
	siteMenuItemGap    = 2
	siteMenuItemRadius = 6
	siteMenuItemPad    = 7
	siteMenuItemGapX   = 10

	siteMenuBasePaletteAfter = 5
	resizeInset              = 6
	accentTintAlpha          = 0x40
)

// Screen draws the content area between the header and the footer.
type Screen interface {
	Paint(p *qt.QPainter, content image.Rectangle, palette appearance.Palette)
}

// FooterScreen optionally draws the footer band. The shell paints the band
// background first and then calls PaintFooter with the band rectangle.
type FooterScreen interface {
	PaintFooter(p *qt.QPainter, footer image.Rectangle, palette appearance.Palette)
}

// PointerAction identifies the kind of a forwarded pointer event.
type PointerAction uint8

// Pointer actions forwarded to a screen.
const (
	PointerMove PointerAction = iota
	PointerDown
	PointerUp
	PointerLeave
)

// PointerEvent is one pointer event in widget coordinates.
type PointerEvent struct {
	Action PointerAction
	Pos    image.Point
}

// WheelEvent is one wheel event in widget coordinates. Delta is positive when
// the wheel scrolls up, away from the user.
type WheelEvent struct {
	Pos   image.Point
	Delta int
}

// Interactive is an optional screen interface. Pointer and Wheel report whether
// they changed the screen and need a repaint.
type Interactive interface {
	Pointer(event PointerEvent) bool
	Wheel(event WheelEvent) bool
	Cursor(pos image.Point) qt.CursorShape
}

// Region identifies the interactive part of the chrome that is hovered or
// pressed. The zero value is the empty header/surface.
type Region int

// Interactive chrome regions.
const (
	RegionNone Region = iota
	RegionSite
	RegionMinimize
	RegionMaximize
	RegionClose
	RegionFooter
)

// State is the externally visible chrome state. It exists so a snapshot can
// pin the menu and a hover without synthesizing input events.
type State struct {
	Site  catalog.Site
	Menu  bool
	Hover Region
}

// Options configures the shell chrome.
type Options struct {
	Site    catalog.Site
	Version string
	Screen  Screen // nil draws an empty content area
}

// Shell is the application window frame drawn by one custom-painted QWidget.
type Shell struct {
	widget *qt.QWidget
	site   catalog.Site
	// palette is the active chrome palette: the site palette, or BasePalette
	// after repeated menu dismissals.
	palette appearance.Palette
	// target caches the palette each switcher row previews.
	target map[catalog.Site]appearance.Palette

	version string
	screen  Screen
	fonts   *fonts

	menuOpen     bool
	hover        Region
	press        Region
	menuRowHover int
	pressMenuRow int
	dismissals   int
	cursor       qt.CursorShape
	screenPress  bool

	cursors map[qt.CursorShape]*qt.QCursor
}

// New creates the frameless window. It must run after qt.NewQApplication.
func New(opts Options) *Shell {
	if opts.Site == "" {
		panic("shell: empty site")
	}
	targets := make(map[catalog.Site]appearance.Palette)
	for _, site := range catalog.Sites() {
		targets[site] = appearance.PaletteFor(site)
	}
	if _, ok := targets[opts.Site]; !ok {
		panic("shell: unknown site " + opts.Site.String())
	}

	w := qt.NewQWidget2()
	w.SetWindowFlags(qt.WindowType(qt.FramelessWindowHint))
	w.SetAttribute(qt.WA_TranslucentBackground)
	w.SetWindowTitle("yo")
	w.SetMinimumSize2(MinimumWidth, MinimumHeight)
	w.Resize(InitialWidth, InitialHeight)
	w.SetMouseTracking(true)

	s := &Shell{
		widget:       w,
		site:         opts.Site,
		palette:      targets[opts.Site],
		target:       targets,
		version:      opts.Version,
		screen:       opts.Screen,
		fonts:        loadFonts(),
		menuRowHover: -1,
		pressMenuRow: -1,
		cursor:       qt.LastCursor,
		cursors:      make(map[qt.CursorShape]*qt.QCursor),
	}
	s.setCursor(qt.ArrowCursor)

	w.OnPaintEvent(func(super func(event *qt.QPaintEvent), event *qt.QPaintEvent) {
		p := qt.NewQPainter2(w.QPaintDevice)
		s.paint(p)
		p.End()
		p.Delete()
	})
	w.OnMousePressEvent(func(super func(event *qt.QMouseEvent), event *qt.QMouseEvent) {
		s.onPress(event)
	})
	w.OnMouseReleaseEvent(func(super func(event *qt.QMouseEvent), event *qt.QMouseEvent) {
		s.onRelease(event)
	})
	w.OnMouseMoveEvent(func(super func(event *qt.QMouseEvent), event *qt.QMouseEvent) {
		s.onMove(event)
	})
	w.OnMouseDoubleClickEvent(func(super func(event *qt.QMouseEvent), event *qt.QMouseEvent) {
		s.onDoubleClick(event)
	})
	w.OnWheelEvent(func(super func(event *qt.QWheelEvent), event *qt.QWheelEvent) {
		s.onWheel(event)
	})
	w.OnLeaveEvent(func(super func(event *qt.QEvent), event *qt.QEvent) {
		s.hover = RegionNone
		s.menuRowHover = -1
		if screen := s.interactive(); screen != nil && !s.screenPress {
			screen.Pointer(PointerEvent{Action: PointerLeave})
		}
		s.setCursor(qt.ArrowCursor)
		w.Update()
	})
	w.OnChangeEvent(func(super func(event *qt.QEvent), event *qt.QEvent) {
		w.Update()
	})
	return s
}

// Widget returns the top-level widget, for snapshots and embedding.
func (s *Shell) Widget() *qt.QWidget { return s.widget }

// Show displays the window. The caller runs the Qt event loop.
func (s *Shell) Show() { s.widget.Show() }

// Close closes the window.
func (s *Shell) Close() { s.widget.Close() }

// Resize sets the logical window size.
func (s *Shell) Resize(width, height int) {
	s.widget.Resize(width, height)
	s.widget.Update()
}

// SetSite switches the active site and restores its palette.
func (s *Shell) SetSite(site catalog.Site) {
	s.switchSite(site)
	s.widget.Update()
}

// Site returns the active site.
func (s *Shell) Site() catalog.Site { return s.site }

// SetScreen replaces the content painter. A nil screen draws nothing.
func (s *Shell) SetScreen(screen Screen) {
	s.screen = screen
	s.screenPress = false
	s.widget.Update()
}

// interactive returns the active screen as an Interactive, if it is one.
func (s *Shell) interactive() Interactive {
	if screen, ok := s.screen.(Interactive); ok {
		return screen
	}
	return nil
}

// SetState applies a snapshot or test state.
func (s *Shell) SetState(state State) {
	if state.Site != "" && state.Site != s.site {
		s.switchSite(state.Site)
	}
	s.menuOpen = state.Menu
	s.hover = state.Hover
	s.menuRowHover = -1
	s.widget.Update()
}

// State returns the current chrome state.
func (s *Shell) State() State {
	return State{Site: s.site, Menu: s.menuOpen, Hover: s.hover}
}

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

// computeLayout mirrors shell.newGeometry.
func (s *Shell) computeLayout() layout {
	size := image.Pt(s.widget.Width(), s.widget.Height())
	l := layout{size: size, border: 1, centerY: 0}
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

	if len(catalog.Sites()) > 1 {
		anchor := s.textWidth(s.fonts.bold, s.site.String())
		for _, site := range catalog.Sites() {
			if width := s.textWidth(s.fonts.bold, site.String()); width > anchor {
				anchor = width
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
		l.trigger = image.Rect(l.logo.Min.X, 0, l.titleX+s.textWidth(s.fonts.bold, s.site.String()), l.headerDivider)
		l.versionX = l.titleX + s.textWidth(s.fonts.bold, s.site.String()) + versionGap
	}
	return l
}

// menuLayout returns the dropdown box, its padded content and the rows of every
// site except the active one.
func (s *Shell) menuLayout(l layout) (image.Rectangle, image.Rectangle, []image.Rectangle, []catalog.Site) {
	sites := catalog.Sites()
	if len(sites) < 2 || l.trigger.Empty() {
		return image.Rectangle{}, image.Rectangle{}, nil, nil
	}
	rows := len(sites) - 1
	line := 1
	pad := siteMenuPadding
	height := 2*(line+pad) + rows*siteMenuItemHeight + max(0, rows-1)*siteMenuItemGap

	right := min(l.trigger.Max.X, l.size.X-siteMenuMargin)
	left := max(l.trigger.Min.X-(line+pad+siteMenuItemPad), siteMenuMargin)
	if right-left < siteMenuMinWidth {
		left = max(siteMenuMargin, right-siteMenuMinWidth)
	}
	right = max(right, left)
	y := l.trigger.Max.Y + siteMenuGap
	if y+height > l.size.Y-siteMenuMargin {
		above := l.trigger.Min.Y - siteMenuGap - height
		if above >= siteMenuMargin {
			y = above
		} else {
			y = l.size.Y - siteMenuMargin - height
		}
	}
	y = max(y, siteMenuMargin)
	menu := image.Rect(left, y, right, y+height)
	if menu.Empty() {
		return image.Rectangle{}, image.Rectangle{}, nil, nil
	}
	inner := menu.Inset(line + pad)
	rects := make([]image.Rectangle, 0, rows)
	visible := make([]catalog.Site, 0, rows)
	slot := 0
	for _, site := range sites {
		if site == s.site {
			continue
		}
		top := inner.Min.Y + slot*(siteMenuItemHeight+siteMenuItemGap)
		rects = append(rects, image.Rect(inner.Min.X, top, inner.Max.X, top+siteMenuItemHeight))
		visible = append(visible, site)
		slot++
	}
	return menu, inner, rects, visible
}

// switchSite applies a selection from the switcher.
func (s *Shell) switchSite(site catalog.Site) {
	palette, ok := s.target[site]
	if !ok {
		panic("shell: unknown site " + site.String())
	}
	s.site = site
	s.palette = palette
	s.menuOpen = false
	s.menuRowHover = -1
	s.dismissals = 0
}

// dismissMenu closes the menu after a user action and counts the dismissal.
func (s *Shell) dismissMenu() {
	if !s.menuOpen {
		return
	}
	s.menuOpen = false
	s.menuRowHover = -1
	s.dismissals++
	if s.dismissals == siteMenuBasePaletteAfter {
		s.palette = appearance.BasePalette()
	}
}

// onPress routes a left-button press.
func (s *Shell) onPress(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	s.press = RegionNone
	s.pressMenuRow = -1

	if s.menuOpen {
		menu, _, rows, _ := s.menuLayout(l)
		if pos.In(menu) {
			for i, row := range rows {
				if pos.In(row) {
					s.pressMenuRow = i
					break
				}
			}
			return
		}
		s.dismissMenu()
		s.widget.Update()
		return
	}

	if !l.maximized {
		if edges, ok := s.edgeAt(l, pos); ok {
			if handle := s.widget.WindowHandle(); handle != nil {
				handle.StartSystemResize(edges)
			}
			return
		}
	}

	if screen := s.interactive(); screen != nil && s.inScreen(l, pos) {
		s.screenPress = true
		s.press = RegionNone
		screen.Pointer(PointerEvent{Action: PointerDown, Pos: pos})
		s.widget.Update()
		return
	}

	switch {
	case pos.In(l.close):
		s.press = RegionClose
	case pos.In(l.maximize):
		s.press = RegionMaximize
	case pos.In(l.minimize):
		s.press = RegionMinimize
	case pos.In(l.trigger):
		s.press = RegionSite
	case pos.Y < l.headerDivider:
		s.press = RegionNone
		if handle := s.widget.WindowHandle(); handle != nil {
			handle.StartSystemMove()
		}
	default:
		s.press = RegionNone
	}
}

// onRelease completes a press. Clicks count only when the release lands on the
// same region.
func (s *Shell) onRelease(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()

	if s.menuOpen {
		_, _, rows, sites := s.menuLayout(l)
		if s.pressMenuRow >= 0 && s.pressMenuRow < len(rows) && pos.In(rows[s.pressMenuRow]) {
			s.switchSite(sites[s.pressMenuRow])
			s.widget.Update()
		}
		s.pressMenuRow = -1
		return
	}

	if s.screenPress {
		s.screenPress = false
		if screen := s.interactive(); screen != nil {
			screen.Pointer(PointerEvent{Action: PointerUp, Pos: pos})
			s.widget.Update()
		}
		return
	}

	pressed := s.press
	s.press = RegionNone
	switch pressed {
	case RegionSite:
		if pos.In(l.trigger) {
			if s.menuOpen {
				s.dismissMenu()
			} else {
				s.menuOpen = true
				s.pressMenuRow = -1
			}
			s.widget.Update()
		}
	case RegionMinimize:
		if pos.In(l.minimize) {
			s.widget.ShowMinimized()
		}
	case RegionMaximize:
		if pos.In(l.maximize) {
			s.toggleMaximize()
		}
	case RegionClose:
		if pos.In(l.close) {
			s.widget.Close()
		}
	}
}

// onMove updates the hovered region, the menu row highlight and the cursor.
func (s *Shell) onMove(e *qt.QMouseEvent) {
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	region := s.regionAt(l, pos)
	row := -1
	if s.menuOpen {
		_, _, rows, _ := s.menuLayout(l)
		for i, r := range rows {
			if pos.In(r) {
				row = i
				break
			}
		}
	}

	screen := s.interactive()
	if screen != nil && !s.menuOpen {
		if s.screenPress || s.inScreen(l, pos) {
			if screen.Pointer(PointerEvent{Action: PointerMove, Pos: pos}) {
				s.widget.Update()
			}
		} else if screen.Pointer(PointerEvent{Action: PointerLeave}) {
			s.widget.Update()
		}
	}

	changed := region != s.hover || row != s.menuRowHover
	s.hover = region
	s.menuRowHover = row

	switch {
	case screen != nil && s.screenPress:
		s.setCursor(screen.Cursor(pos))
	case !l.maximized && row < 0 && region == RegionNone:
		if edges, ok := s.edgeAt(l, pos); ok {
			s.setCursor(edgeCursor(edges))
		} else {
			s.setCursor(qt.ArrowCursor)
		}
	case row >= 0:
		s.setCursor(qt.PointingHandCursor)
	case region == RegionSite || region == RegionMinimize || region == RegionMaximize || region == RegionClose:
		s.setCursor(qt.PointingHandCursor)
	case screen != nil && s.inScreen(l, pos):
		s.setCursor(screen.Cursor(pos))
	default:
		s.setCursor(qt.ArrowCursor)
	}
	if changed {
		s.widget.Update()
	}
}

// onWheel forwards a wheel event to the screen under the pointer.
func (s *Shell) onWheel(e *qt.QWheelEvent) {
	screen := s.interactive()
	if screen == nil || s.menuOpen {
		return
	}
	point := e.Position()
	pos := image.Pt(int(point.X()), int(point.Y()))
	l := s.computeLayout()
	if !s.inScreen(l, pos) {
		return
	}
	delta := e.AngleDelta().Y() / 120
	if e.HasPixelDelta() {
		delta = e.PixelDelta().Y()
	}
	if delta == 0 {
		return
	}
	if screen.Wheel(WheelEvent{Pos: pos, Delta: int(delta)}) {
		s.widget.Update()
	}
	e.Accept()
}

// inScreen reports whether pos belongs to the content or footer band.
func (s *Shell) inScreen(l layout, pos image.Point) bool {
	return pos.In(l.contentRect()) || pos.In(l.footerRect())
}

// onDoubleClick toggles maximize on the empty part of the header.
func (s *Shell) onDoubleClick(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	if pos.Y >= l.headerDivider {
		return
	}
	if pos.In(l.trigger) || pos.In(l.minimize) || pos.In(l.maximize) || pos.In(l.close) {
		return
	}
	s.toggleMaximize()
}

// toggleMaximize swaps between the maximized and normal window states.
func (s *Shell) toggleMaximize() {
	if s.widget.IsMaximized() {
		s.widget.ShowNormal()
	} else {
		s.widget.ShowMaximized()
	}
}

// edgeAt reports the window edges under pos for a system resize.
func (s *Shell) edgeAt(l layout, pos image.Point) (qt.Edge, bool) {
	var edges qt.Edge
	if pos.X < resizeInset {
		edges |= qt.LeftEdge
	} else if pos.X >= l.size.X-resizeInset {
		edges |= qt.RightEdge
	}
	if pos.Y < resizeInset {
		edges |= qt.TopEdge
	} else if pos.Y >= l.size.Y-resizeInset {
		edges |= qt.BottomEdge
	}
	return edges, edges != 0
}

// regionAt maps a point to the control under it.
func (s *Shell) regionAt(l layout, pos image.Point) Region {
	switch {
	case pos.In(l.close):
		return RegionClose
	case pos.In(l.maximize):
		return RegionMaximize
	case pos.In(l.minimize):
		return RegionMinimize
	case pos.In(l.trigger):
		return RegionSite
	case pos.In(l.footerRect()):
		return RegionFooter
	}
	return RegionNone
}

// edgeCursor returns the resize cursor for a set of window edges.
func edgeCursor(edges qt.Edge) qt.CursorShape {
	switch edges {
	case qt.LeftEdge, qt.RightEdge:
		return qt.SizeHorCursor
	case qt.TopEdge, qt.BottomEdge:
		return qt.SizeVerCursor
	case qt.TopEdge | qt.LeftEdge, qt.BottomEdge | qt.RightEdge:
		return qt.SizeFDiagCursor
	case qt.TopEdge | qt.RightEdge, qt.BottomEdge | qt.LeftEdge:
		return qt.SizeBDiagCursor
	}
	return qt.ArrowCursor
}

// setCursor changes the widget cursor only when the shape differs.
func (s *Shell) setCursor(shape qt.CursorShape) {
	if s.cursor == shape {
		return
	}
	s.cursor = shape
	cursor, ok := s.cursors[shape]
	if !ok {
		cursor = qt.NewQCursor2(shape)
		s.cursors[shape] = cursor
	}
	s.widget.SetCursor(cursor)
}

// textWidth measures text in the logical pixels of the widget device. The
// advance is rounded up: go-text accumulates fractional advances and rounds the
// shaped width up, and the chevron and version must not drift a pixel left.
func (s *Shell) textWidth(f *qt.QFont, text string) int {
	fm := qt.NewQFontMetricsF2(f, s.widget.QPaintDevice)
	width := int(math.Ceil(fm.HorizontalAdvance(text)))
	fm.Delete()
	return width
}
