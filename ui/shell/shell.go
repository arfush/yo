// Package shell draws the undecorated application window: the border, header,
// footer band and window controls. It owns where the active screen is drawn.
//
// Imports: std, gioui.org/app, gioui.org/layout, gioui.org/op, gioui.org/op/clip,
// gioui.org/op/paint, gioui.org/widget, ui/appearance, ui/kit, ui/resource.
// Must not import: core/*, root, ui/navigator, ui/screen.
//
// Invariants: a screen is never nil; New and SetScreen panic on nil. Layout
// draws the screen only inside the content rectangle and its footer only inside
// the footer band. A dialog, when set, is drawn over the body; nil clears it.
// Only the UI goroutine calls the shell.
package shell

import (
	"image"

	"gioui.org/app"
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

// Dialog is the modal content contract. The shell dims the screen body, then
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
	s.layoutDialog(gtx, g, content)
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

// layoutScreen records the active screen in the clipped content rectangle and
// draws it once. The recorded call lets the dialog layer replay the content for
// the blur without laying the screen out again.
func (s *Shell) layoutScreen(gtx layout.Context, g geometry) op.CallOp {
	bounds := g.contentRect()
	if bounds.Empty() {
		return op.CallOp{}
	}
	macro := op.Record(gtx.Ops)
	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	screenGtx := gtx
	screenGtx.Constraints = layout.Exact(bounds.Size())
	s.screen.Layout(screenGtx)
	offset.Pop()
	call := macro.Stop()

	clipStack := clip.Rect(bounds).Push(gtx.Ops)
	call.Add(gtx.Ops)
	clipStack.Pop()
	return call
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

// layoutDialog dims the body and draws the modal above it. The blur replays the
// recorded screen five times with small offsets and low opacity, clipped to the
// body, then the palette overlay and the dialog are painted on top.
func (s *Shell) layoutDialog(gtx layout.Context, g geometry, content op.CallOp) {
	if s.dialog == nil {
		return
	}
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}

	clipStack := clip.Rect(bounds).Push(gtx.Ops)
	defer clipStack.Pop()

	radius := g.px(3)
	taps := [...]image.Point{
		{},
		{X: radius},
		{X: -radius},
		{Y: radius},
		{Y: -radius},
	}
	for _, tap := range taps {
		offset := op.Offset(tap).Push(gtx.Ops)
		opacity := paint.PushOpacity(gtx.Ops, 0.2)
		content.Add(gtx.Ops)
		opacity.Pop()
		offset.Pop()
	}
	fill(gtx, s.appearance.Palette().Overlay, bounds)

	offset := op.Offset(bounds.Min).Push(gtx.Ops)
	defer offset.Pop()
	dialogGtx := gtx
	dialogGtx.Constraints = layout.Exact(bounds.Size())
	s.dialog.Layout(dialogGtx)
}
