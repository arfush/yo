package shell

import (
	"image"

	"gioui.org/app"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

// Screen is the content contract of the window. The shell never imports a
// concrete screen: the navigator installs it through SetScreen.
type Screen interface {
	Layout(layout.Context)
	LayoutFooter(layout.Context)
}

// Dialog is the modal content contract. The shell blurs the screen body, then
// draws the dialog over it. A dialog may also replace the footer band, like a
// screen does. The shell never imports a concrete dialog.
type Dialog interface {
	Layout(layout.Context)
	LayoutFooter(layout.Context)
}

// Options configures the shell chrome.
type Options struct {
	Screen     Screen
	Window     *app.Window
	Site       *stdx.Holder[catalog.Site]
	Appearance *stdx.Holder[appearance.Appearance]
	Resources  *resource.Resource
	Version    string
}

// Shell is the application window frame. Use New to construct it, SetScreen to
// replace the content and Layout to draw a frame.
type Shell struct {
	window     *app.Window
	site       *stdx.Holder[catalog.Site]
	appearance *stdx.Holder[appearance.Appearance]
	resources  *resource.Resource
	version    string
	screen     Screen
	dialog     Dialog
	maximized  bool
	content    op.CallOp

	siteState catalog.Site
	siteClick widget.Clickable
	siteItems []widget.Clickable
	menu      siteMenu
	scrim     siteScrim

	backdrop      paint.ImageOp
	backdropSize  image.Point
	backdropReady bool
	backdropTried bool
	blurWindow    *headless.Window
	blurSize      image.Point

	minimize widget.Clickable
	maximize widget.Clickable
	close    widget.Clickable
}

// New returns a shell with the given chrome. A nil screen, site holder or
// appearance holder is a programmer error and panics.
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
	return &Shell{
		window:     opts.Window,
		site:       opts.Site,
		appearance: opts.Appearance,
		resources:  opts.Resources,
		version:    opts.Version,
		screen:     opts.Screen,
		siteItems:  make([]widget.Clickable, len(catalog.Sites())),
	}
}

// palette returns the colors of the current appearance.
func (s *Shell) palette() appearance.Palette { return s.appearance.Value().Palette() }

// typography returns the text shaper and role scale of the current appearance.
func (s *Shell) typography() appearance.Typography { return s.appearance.Value().Typography() }

// locale returns the active language of the current appearance.
func (s *Shell) locale() appearance.Locale { return s.appearance.Value().Locale() }

// syncPalette copies the active site palette into the shared appearance. It runs
// before layout, so a site change requested by the core is visible next frame.
func (s *Shell) syncPalette() {
	site := s.site.Value()
	if site == s.siteState {
		return
	}
	palette := appearance.PaletteFor(site)
	s.appearance.Update(func(a *appearance.Appearance) { a.SetPalette(palette) })
	s.siteState = site
}

// SetScreen replaces the content of the window. A nil screen is a programmer
// error and panics. The navigator calls it when navigation changes the screen.
func (s *Shell) SetScreen(content Screen) {
	if content == nil {
		panic("shell: nil screen")
	}
	s.screen = content
	s.menu.close()
	s.invalidateBackdrop()
}

// SetTitle replaces the native window title. The navigator calls it with the
// active screen title. The header itself shows the active site.
func (s *Shell) SetTitle(title string) {
	if s.window == nil {
		return
	}
	s.window.Option(app.Title(title))
}

// SetDialog installs or clears the modal dialog. A nil dialog closes the modal;
// the navigator calls it when the dialog stack changes.
func (s *Shell) SetDialog(dialog Dialog) {
	s.dialog = dialog
	s.menu.close()
	s.invalidateBackdrop()
}

// invalidateBackdrop drops the cached blurred screen so the next dialog frame
// captures it again.
func (s *Shell) invalidateBackdrop() {
	s.backdrop = paint.ImageOp{}
	s.backdropSize = image.Point{}
	s.backdropReady = false
	s.backdropTried = false
}

// SetConfig updates the window state used by the maximize glyph. The
// composition root forwards app.ConfigEvent values here.
func (s *Shell) SetConfig(cfg app.Config) {
	s.maximized = cfg.Mode != app.Windowed
}

// Prepare records the active screen and refreshes the modal backdrop. It must
// run once per frame on the UI goroutine before Layout, with the same context:
// the recorded content is replayed by Layout. The offscreen render stays out of
// Layout, so layout itself never blocks on the GPU.
func (s *Shell) Prepare(gtx layout.Context) {
	if s.screen == nil {
		panic("shell: no screen installed")
	}
	s.syncPalette()
	g := s.newGeometry(gtx)
	s.content = s.layoutScreen(gtx, g)
	s.prepareBackdrop(gtx, g)
}

// Layout draws one frame of the shell from the content recorded by Prepare.
func (s *Shell) Layout(gtx layout.Context) {
	if s.screen == nil {
		panic("shell: no screen installed")
	}
	g := s.newGeometry(gtx)
	s.layoutFrame(gtx, g)
	s.layoutHeader(gtx, g)
	s.layoutFooter(gtx, g)
	if s.dialog == nil {
		s.layoutContent(gtx, g, s.content)
	} else {
		s.layoutDialog(gtx, g, s.content)
	}
	s.layoutSiteMenu(gtx, g)
}

// prepareBackdrop captures the recorded screen once per dialog and size. The
// result is cached until SetDialog, SetScreen or a resize invalidates it.
func (s *Shell) prepareBackdrop(gtx layout.Context, g geometry) {
	if s.dialog == nil {
		return
	}
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}
	size := bounds.Size()
	if s.backdropTried && s.backdropSize == size {
		return
	}
	img, ok := s.captureBackdrop(s.content, size, s.palette().Canvas, gtx.Dp(blurRadiusDp))
	s.backdropTried = true
	s.backdropSize = size
	s.backdropReady = ok
	if ok {
		s.backdrop = img
	}
}

// layoutFrame paints the window border, the rounded inner surface, the header
// band and the footer band with its dividers.
func (s *Shell) layoutFrame(gtx layout.Context, g geometry) {
	palette := s.palette()
	fill(gtx, palette.Border, image.Rectangle{Max: g.size})
	if g.inner.Empty() {
		return
	}

	clipStack := clip.RRect{
		Rect: g.inner,
		NW:   g.cornerRadius,
		NE:   g.cornerRadius,
		SE:   g.cornerRadius,
		SW:   g.cornerRadius,
	}.Push(gtx.Ops)
	defer clipStack.Pop()

	fill(gtx, palette.Canvas, g.inner)
	fill(gtx, palette.Surface, rect(
		g.inner.Min.X, g.inner.Min.Y, g.inner.Max.X, g.headerDivider,
	))
	fill(gtx, palette.Divider, rect(
		g.inner.Min.X, g.headerDivider, g.inner.Max.X, g.contentTop,
	))
	fill(gtx, palette.Divider, rect(
		g.inner.Min.X, g.footerDivider, g.inner.Max.X, g.footerTop,
	))
	fill(gtx, palette.Surface, rect(
		g.inner.Min.X, g.footerTop, g.inner.Max.X, g.footerBottom,
	))
	fill(gtx, palette.Canvas, rect(
		g.inner.Min.X, g.stripTop, g.inner.Max.X, g.inner.Max.Y,
	))
}

// layoutScreen records the active screen in content-local coordinates. The
// recorded call lets the shell draw the content once, or replay it offscreen for
// the modal backdrop, without laying the screen out again.
func (s *Shell) layoutScreen(gtx layout.Context, g geometry) op.CallOp {
	bounds := g.contentRect()
	if bounds.Empty() {
		return op.CallOp{}
	}
	macro := op.Record(gtx.Ops)
	screenGtx := gtx
	screenGtx.Constraints = layout.Exact(bounds.Size())
	s.screen.Layout(screenGtx)
	return macro.Stop()
}

// layoutContent draws the recorded screen in the clipped content rectangle.
func (s *Shell) layoutContent(gtx layout.Context, g geometry, content op.CallOp) {
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}
	clipStack := clip.Rect(bounds).Push(gtx.Ops)
	defer clipStack.Pop()
	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	defer offset.Pop()
	content.Add(gtx.Ops)
}

// layoutFooter lets the active dialog replace the footer band. Without a
// dialog, the active screen fills the band.
func (s *Shell) layoutFooter(gtx layout.Context, g geometry) {
	bounds := g.footerRect()
	if bounds.Empty() {
		return
	}
	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	defer offset.Pop()
	gtx.Constraints = layout.Exact(bounds.Size())
	clipStack := clip.Rect(image.Rectangle{Max: bounds.Size()}).Push(gtx.Ops)
	defer clipStack.Pop()
	if s.dialog != nil {
		s.dialog.LayoutFooter(gtx)
		return
	}
	s.screen.LayoutFooter(gtx)
}

// layoutDialog replaces the body with the cached blurred snapshot of the
// screen, dims it, then draws the modal above it. The snapshot itself is made
// by Prepare; Layout only replays it.
func (s *Shell) layoutDialog(gtx layout.Context, g geometry, content op.CallOp) {
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}

	clipStack := clip.Rect(bounds).Push(gtx.Ops)
	defer clipStack.Pop()

	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	if s.backdropReady {
		s.backdrop.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	} else {
		content.Add(gtx.Ops)
	}
	offset.Pop()

	fill(gtx, s.palette().Overlay, bounds)

	dialogOffset := op.Offset(bounds.Min).Push(gtx.Ops)
	defer dialogOffset.Pop()
	dialogGtx := gtx
	dialogGtx.Constraints = layout.Exact(bounds.Size())
	s.dialog.Layout(dialogGtx)
}
