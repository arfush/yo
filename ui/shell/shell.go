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
	Appearance appearance.Appearance
	Resources  *resource.Resource
	Title      string
	Version    string
}

// Shell is the application window frame. Use New to construct it, SetScreen to
// replace the content and Layout to draw a frame.
type Shell struct {
	window     *app.Window
	appearance appearance.Appearance
	resources  *resource.Resource
	title      string
	version    string
	screen     Screen
	dialog     Dialog
	maximized  bool

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

// New returns a shell with the given chrome. A nil screen is a programmer error
// and panics: the shell always has content.
func New(opts Options) *Shell {
	if opts.Screen == nil {
		panic("shell: nil screen")
	}
	return &Shell{
		window:     opts.Window,
		appearance: opts.Appearance,
		resources:  opts.Resources,
		title:      opts.Title,
		version:    opts.Version,
		screen:     opts.Screen,
	}
}

// SetScreen replaces the content of the window. A nil screen is a programmer
// error and panics. The navigator calls it when navigation changes the screen.
func (s *Shell) SetScreen(content Screen) {
	if content == nil {
		panic("shell: nil screen")
	}
	s.screen = content
	s.invalidateBackdrop()
}

// SetTitle replaces the header title. The navigator calls it with the active
// screen title. An empty title clears the label but leaves the brand mark.
func (s *Shell) SetTitle(title string) {
	s.title = title
}

// SetDialog installs or clears the modal dialog. A nil dialog closes the modal;
// the navigator calls it when the dialog stack changes.
func (s *Shell) SetDialog(dialog Dialog) {
	s.dialog = dialog
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

// Layout draws one frame of the shell.
func (s *Shell) Layout(gtx layout.Context) {
	if s.screen == nil {
		panic("shell: no screen installed")
	}
	g := newGeometry(gtx, s.maximized)
	s.layoutFrame(gtx, g)
	s.layoutHeader(gtx, g)
	content := s.layoutScreen(gtx, g)
	s.layoutFooter(gtx, g)
	if s.dialog == nil {
		s.layoutContent(gtx, g, content)
	} else {
		s.layoutDialog(gtx, g, content)
	}
}

// layoutFrame paints the window border, the rounded inner surface, the header
// band and the footer band with its dividers.
func (s *Shell) layoutFrame(gtx layout.Context, g geometry) {
	palette := s.appearance.Palette()
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

// layoutDialog replaces the body with a blurred snapshot of the screen, dims
// it, then draws the modal above it. The snapshot is captured once per dialog
// and size: replaying the recorded screen costs one offscreen render instead of
// one full-screen layer per blur sample every frame.
func (s *Shell) layoutDialog(gtx layout.Context, g geometry, content op.CallOp) {
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}

	clipStack := clip.Rect(bounds).Push(gtx.Ops)
	defer clipStack.Pop()

	size := bounds.Size()
	if !s.backdropReady && (!s.backdropTried || s.backdropSize != size) {
		img, ok := s.captureBackdrop(content, size, s.appearance.Palette().Canvas, gtx.Dp(blurRadiusDp))
		s.backdropTried = true
		s.backdropSize = size
		if ok {
			s.backdrop = img
			s.backdropReady = true
		}
	}

	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	if s.backdropReady {
		s.backdrop.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	} else {
		content.Add(gtx.Ops)
	}
	offset.Pop()

	fill(gtx, s.appearance.Palette().Overlay, bounds)

	dialogOffset := op.Offset(bounds.Min).Push(gtx.Ops)
	defer dialogOffset.Pop()
	dialogGtx := gtx
	dialogGtx.Constraints = layout.Exact(bounds.Size())
	s.dialog.Layout(dialogGtx)
}
