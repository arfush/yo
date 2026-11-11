package shell

import (
	"image"
	"log/slog"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/platform"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/resource"
)

// Screen is the content contract of the window. The shell never imports a
// concrete screen: the navigator installs it through SetScreen.
type Screen interface {
	Layout(layout.Context)
	LayoutFooter(layout.Context)
}

// Options configures the shell chrome.
type Options struct {
	Screen     Screen
	Window     *app.Window
	Platform   *platform.Window // native frame; nil disables cursor polling
	Site       *stdx.Holder[catalog.Site]
	Appearance *stdx.Holder[appearance.Appearance]
	Resource   *resource.Resource
	Version    string
	Logger     *slog.Logger // nil discards logs
}

// Shell is the application window frame. Use New to construct it, SetScreen to
// replace the content and Layout to draw a frame.
type Shell struct {
	window     *app.Window
	platform   *platform.Window
	site       *stdx.Holder[catalog.Site]
	appearance *stdx.Holder[appearance.Appearance]
	resource   *resource.Resource
	version    string
	logger     *slog.Logger
	screen     Screen
	maximized  bool
	frozen     bool
	content    op.CallOp

	siteState catalog.Site
	siteClick widget.Clickable
	siteItems []widget.Clickable
	menu      siteMenu
	scrim     siteScrim

	// siteDismissals counts consecutive user dismissals of the site menu
	// without a switch. At siteMenuBasePaletteAfter the chrome falls back to
	// the neutral yobase palette until the next selection.
	siteDismissals int

	minimize widget.Clickable
	maximize widget.Clickable
	close    widget.Clickable
}

// New returns a shell with the given chrome. A nil screen, site holder or
// appearance holder is a programmer error and panics. A nil logger discards
// shell logs.
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
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Shell{
		window:     opts.Window,
		platform:   opts.Platform,
		site:       opts.Site,
		appearance: opts.Appearance,
		resource:   opts.Resource,
		version:    opts.Version,
		logger:     logger,
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

// Freeze stops palette syncing from the site holder and disables the site
// switcher. The composition root calls it after a recovered panic so the panic
// screen stays on the neutral palette.
func (s *Shell) Freeze() {
	s.frozen = true
	s.menu.close()
}

// syncPalette copies the active site palette into the shared appearance. It runs
// before layout, so a site change requested by the core is visible next frame.
func (s *Shell) syncPalette() {
	if s.frozen {
		return
	}
	site := s.site.Value()
	if site == s.siteState {
		return
	}
	s.applySitePalette(site)
}

// applySitePalette writes the site palette and icon into the shared appearance
// and records the site they belong to.
func (s *Shell) applySitePalette(site catalog.Site) {
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
}

// SetScreen replaces the content of the window. A nil screen is a programmer
// error and panics. The navigator calls it when navigation changes the screen.
func (s *Shell) SetScreen(content Screen) {
	if content == nil {
		panic("shell: nil screen")
	}
	s.screen = content
	s.menu.close()
}

// SetTitle replaces the native window title. The navigator calls it with the
// active screen title. The header itself shows the active site.
func (s *Shell) SetTitle(title string) {
	if s.window == nil {
		return
	}
	s.window.Option(app.Title(title))
}

// SetConfig updates the window state used by the maximize glyph. The
// composition root forwards app.ConfigEvent values here. Losing focus closes the
// site menu, like a native dropdown.
func (s *Shell) SetConfig(cfg app.Config) {
	s.maximized = cfg.Mode != app.Windowed
	if cfg.Focused || !s.menu.open {
		return
	}
	s.dismissSiteMenu()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// Prepare records the active screen. It must run once per frame on the UI
// goroutine before Layout, with the same context: the recorded content is
// replayed by Layout.
func (s *Shell) Prepare(gtx layout.Context) {
	if s.screen == nil {
		panic("shell: no screen installed")
	}
	s.syncPalette()
	g := s.newGeometry(gtx)
	s.content = s.layoutScreen(gtx, g)
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
	s.layoutContent(gtx, g, s.content)
	s.layoutSiteMenu(gtx, g)
}

// layoutFrame paints the window border, the rounded inner surface, the header
// band and the footer band with its dividers.
func (s *Shell) layoutFrame(gtx layout.Context, g geometry) {
	palette := s.palette()
	kit.FillRect(gtx, palette.Border, image.Rectangle{Max: g.size})
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

	kit.FillRect(gtx, palette.Canvas, g.inner)
	kit.FillRect(gtx, palette.Surface, rect(
		g.inner.Min.X, g.inner.Min.Y, g.inner.Max.X, g.headerDivider,
	))
	kit.FillRect(gtx, palette.Divider, rect(
		g.inner.Min.X, g.headerDivider, g.inner.Max.X, g.contentTop,
	))
	kit.FillRect(gtx, palette.Divider, rect(
		g.inner.Min.X, g.footerDivider, g.inner.Max.X, g.footerTop,
	))
	kit.FillRect(gtx, palette.Surface, rect(
		g.inner.Min.X, g.footerTop, g.inner.Max.X, g.footerBottom,
	))
	kit.FillRect(gtx, palette.Canvas, rect(
		g.inner.Min.X, g.stripTop, g.inner.Max.X, g.inner.Max.Y,
	))
}

// layoutScreen records the active screen in content-local coordinates. The
// recorded call lets Layout draw the content without laying the screen out
// again.
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

// layoutFooter draws the footer band of the active screen.
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
	s.screen.LayoutFooter(gtx)
}
