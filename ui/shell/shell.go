// Package shell draws the undecorated application window: the border, header,
// footer band and window controls. It owns where the active screen is drawn.
//
// Must not import: core/*, root, ui/navigator, ui/screen.
//
// Invariants: a screen, site holder and appearance holder are never nil; New
// and SetScreen panic on a nil screen. The shell syncs the palette from the site
// holder before painting. A screen is painted only inside the content rectangle
// and its footer only inside the footer band. Site changes go through
// Holder.Update. Only the UI goroutine calls the shell.
package shell

import (
	"image"
	"image/draw"
	"log/slog"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

// Window sizes. The designed size is also the smallest usable size: the chrome
// and the screens are verified at exactly this width and height.
const (
	InitialWidth  = 840
	InitialHeight = 550
	MinimumWidth  = InitialWidth
	MinimumHeight = InitialHeight
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

// Options configures the shell chrome.
type Options struct {
	Screen     Screen
	Site       *stdx.Holder[catalog.Site]
	Appearance *stdx.Holder[appearance.Appearance]
	Resource   *resource.Resource
	Version    string
	Logger     *slog.Logger // nil discards logs
}

// Shell is the application window frame drawn by one custom-painted QWidget.
type Shell struct {
	widget     *qt.QWidget
	site       *stdx.Holder[catalog.Site]
	appearance *stdx.Holder[appearance.Appearance]
	resource   *resource.Resource
	version    string
	logger     *slog.Logger
	screen     Screen

	// siteState records the site whose palette was last written into the
	// shared appearance, so external site changes are picked up once.
	siteState catalog.Site

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
	if opts.Screen == nil {
		panic("shell: nil screen")
	}
	if opts.Site == nil {
		panic("shell: nil site holder")
	}
	if opts.Appearance == nil {
		panic("shell: nil appearance holder")
	}
	if opts.Resource == nil {
		panic("shell: nil resources")
	}
	if !knownSite(opts.Site.Value()) {
		panic("shell: unknown site \"" + opts.Site.Value().String() + "\"")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
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
		appearance:   opts.Appearance,
		resource:     opts.Resource,
		version:      opts.Version,
		logger:       logger,
		screen:       opts.Screen,
		menuRowHover: -1,
		pressMenuRow: -1,
		cursor:       qt.LastCursor,
		cursors:      make(map[qt.CursorShape]*qt.QCursor),
	}
	s.syncPalette()
	s.setCursor(qt.ArrowCursor)
	s.bindEvents()
	return s
}

// bindEvents connects the native widget events to the shell.
func (s *Shell) bindEvents() {
	w := s.widget
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

// SetScreen replaces the content painter. A nil screen is a programmer error
// and panics.
func (s *Shell) SetScreen(screen Screen) {
	if screen == nil {
		panic("shell: nil screen")
	}
	s.screen = screen
	s.screenPress = false
	s.menuOpen = false
	s.widget.Update()
}

// SetTitle replaces the native window title.
func (s *Shell) SetTitle(title string) {
	s.widget.SetWindowTitle(title)
}

// palette returns the colors of the current appearance.
func (s *Shell) palette() appearance.Palette {
	app := s.appearance.Value()
	return app.Palette()
}

// interactive returns the active screen as an Interactive, if it is one.
func (s *Shell) interactive() Interactive {
	if screen, ok := s.screen.(Interactive); ok {
		return screen
	}
	return nil
}

// syncPalette copies the active site palette into the shared appearance. It runs
// before painting, so a site change requested by the core is visible next frame.
func (s *Shell) syncPalette() {
	site := s.site.Value()
	if site == s.siteState {
		return
	}
	s.updateAppearance(appearance.PaletteFor(site), appearance.AppIconFor(site))
	s.siteState = site
}

// applyPalette resets the chrome to a neutral palette and the base icon.
func (s *Shell) applyPalette(palette appearance.Palette) {
	s.updateAppearance(palette, resource.AppIconYobase)
}

// updateAppearance replaces the shared palette and application icon together.
func (s *Shell) updateAppearance(palette appearance.Palette, icon resource.AppIconKey) {
	s.appearance.Update(func(a *appearance.Appearance) {
		a.SetPalette(palette)
		a.SetAppIcon(icon)
	})
	s.setAppIcon(icon)
}

// setAppIcon replaces the native window icon. The embedded PNG is full
// resolution, so Windows can pick any size it needs for the title bar and the
// taskbar.
func (s *Shell) setAppIcon(key resource.AppIconKey) {
	img := s.resource.AppIcon(key)
	bounds := img.Bounds()
	if bounds.Empty() {
		return
	}
	nrgba := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, bounds.Min, draw.Src)
	w, h := bounds.Dx(), bounds.Dy()
	// QImage.Copy and QPixmap_FromImage register Go finalizers that delete the
	// native objects again from a non-GUI goroutine. Convert without them and
	// release both objects here, on the UI goroutine.
	source := qt.NewQImage6(&nrgba.Pix[0], w, h, int64(nrgba.Stride), qt.QImage__Format_RGBA8888)
	pixmap := qt.NewQPixmap2(w, h)
	pixmap.ConvertFromImage(source)
	icon := qt.NewQIcon2(pixmap)
	s.widget.SetWindowIcon(icon)
	icon.Delete()
	pixmap.Delete()
	source.Delete()
}

// switchSite applies a selection from the header switcher. The shell updates
// the holder because it owns the writable site source; the palette follows on
// the next paint. A selection always restores the site palette, even when the
// neutral fallback is showing for the same site.
func (s *Shell) switchSite(site catalog.Site) {
	if !knownSite(site) {
		panic("shell: unknown site \"" + site.String() + "\"")
	}
	s.menuOpen = false
	s.menuRowHover = -1
	s.dismissals = 0
	previous := s.site.Value()
	if previous == site {
		s.updateAppearance(appearance.PaletteFor(site), appearance.AppIconFor(site))
		s.siteState = site
		return
	}
	s.site.Update(func(v *catalog.Site) { *v = site })
	s.logger.Info("site switched", "site", site.String(), "previous", previous.String())
	s.syncPalette()
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
		s.applyPalette(appearance.BasePalette())
	}
}

// paint draws one complete frame.
func (s *Shell) paint(p *qt.QPainter) {
	p.SetRenderHints(qt.QPainter__Antialiasing | qt.QPainter__TextAntialiasing | qt.QPainter__SmoothPixmapTransform)
	s.syncPalette()
	l := s.computeLayout()
	s.paintFrame(p, l)
	s.paintHeader(p, l)
	if content := l.contentRect(); !content.Empty() {
		p.Save()
		p.SetClipRect2(content.Min.X, content.Min.Y, content.Dx(), content.Dy())
		s.screen.Paint(p, content, s.palette())
		p.Restore()
	}
	if footer := l.footerRect(); !footer.Empty() {
		if screen, ok := s.screen.(FooterScreen); ok {
			p.Save()
			p.SetClipRect2(footer.Min.X, footer.Min.Y, footer.Dx(), footer.Dy())
			screen.PaintFooter(p, footer, s.palette())
			p.Restore()
		}
	}
	if s.menuOpen {
		s.paintMenu(p, l)
	}
}

// knownSite reports whether site is one of the catalog sites.
func knownSite(site catalog.Site) bool {
	for _, known := range catalog.Sites() {
		if site == known {
			return true
		}
	}
	return false
}
