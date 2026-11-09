package shell

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/resource"
)

// Window sizes. The designed size is also the smallest usable size: the chrome
// and the screens are verified at exactly this width and height.
const (
	InitialWidth  = 833
	InitialHeight = 545
	MinimumWidth  = InitialWidth
	MinimumHeight = InitialHeight
)

// Chrome geometry, in device-independent dp.
const (
	headerHeight    = 52
	footerHeight    = 49
	innerRadius     = 8
	controlWidth    = 30
	controlGap      = 12
	controlIconSize = 24
	closeIconSize   = 28
	controlsInset   = 12
	logoInset       = 16
	logoSize        = 22
	labelGap        = 10
	contentGap      = 14
	badgeHeight     = 20
	badgePaddingX   = 7
	badgeGap        = 12
)

// geometry holds every chrome measurement converted to pixels for the current
// frame.
type geometry struct {
	size  image.Point
	scale float32

	border         int
	inner          image.Rectangle
	headerDivider  int
	contentTop     int
	contentBottom  int
	footerDivider  int
	footerTop      int
	footerBottom   int
	stripTop       int
	cornerRadius   int
	headerClickEnd int
	headerCenterY  int

	logo     image.Rectangle
	titleX   int
	controls controlGeometry
}

type controlGeometry struct {
	minimize, maximize, close image.Rectangle
}

func (g geometry) px(v float32) int {
	return int(float32(v) * g.scale)
}

func (g geometry) leastPx(v float32) int {
	if n := g.px(v); n > 0 {
		return n
	}
	return 1
}

// contentRect returns the drawable area between the header and the footer. It
// is empty when the window is too small for the chrome.
func (g geometry) contentRect() image.Rectangle {
	if g.inner.Empty() || g.contentBottom <= g.contentTop {
		return image.Rectangle{}
	}
	return image.Rect(g.inner.Min.X, g.contentTop, g.inner.Max.X, g.contentBottom)
}

// footerRect returns the band where the active screen draws its controls.
func (g geometry) footerRect() image.Rectangle {
	if g.inner.Empty() || g.footerBottom <= g.footerTop {
		return image.Rectangle{}
	}
	return image.Rect(g.inner.Min.X, g.footerTop, g.inner.Max.X, g.footerBottom)
}

// newGeometry converts the chrome measurements for the current frame.
func newGeometry(gtx layout.Context, maximized bool) geometry {
	size := gtx.Constraints.Max
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}

	g := geometry{size: size, scale: scale}
	g.border = g.leastPx(1)
	if size.X > 2*g.border && size.Y > 2*g.border {
		g.inner = image.Rect(g.border, g.border, size.X-g.border, size.Y-g.border)
	}

	g.headerDivider = g.border + g.px(headerHeight-1)
	g.contentTop = g.headerDivider + g.leastPx(1)
	g.footerBottom = size.Y - g.border - g.leastPx(1)
	g.footerTop = g.footerBottom - g.px(footerHeight)
	g.footerDivider = g.footerTop - g.leastPx(1)
	g.contentBottom = g.footerDivider
	g.stripTop = g.footerBottom
	g.headerClickEnd = g.headerDivider
	if !maximized {
		g.cornerRadius = g.px(innerRadius)
	}
	g.headerCenterY = (g.border + g.headerDivider) / 2

	logo := g.px(logoSize)
	logoTop := g.headerCenterY - logo/2
	g.logo = rect(g.px(logoInset), logoTop, g.px(logoInset)+logo, logoTop+logo)
	g.titleX = g.logo.Max.X + g.px(labelGap)

	right := size.X - g.border - g.px(controlsInset)
	width := g.px(controlWidth)
	gap := g.px(controlGap)
	g.controls.close = rect(right-width, 0, right, g.headerClickEnd)
	g.controls.maximize = g.controls.close.Add(image.Pt(-(width + gap), 0))
	g.controls.minimize = g.controls.maximize.Add(image.Pt(-(width + gap), 0))
	return g
}

// layoutHeader paints the draggable regions, the brand and the window controls.
func (s *Shell) layoutHeader(gtx layout.Context, g geometry) {
	if g.inner.Empty() {
		return
	}
	// Drag regions come first, so the controls added later sit above them and
	// receive pointer input.
	s.layoutDragRegions(gtx, g)
	s.layoutBrand(gtx, g)

	palette := s.appearance.Palette()
	maximizeIcon := s.icon(resource.IconMaximize)
	maximizeAction := system.ActionMaximize
	if s.maximized {
		maximizeIcon = s.icon(resource.IconUnmaximize)
		maximizeAction = system.ActionUnmaximize
	}
	s.layoutControl(gtx, g, g.controls.minimize, &s.minimize,
		s.icon(resource.IconMinimize), system.ActionMinimize, controlIconSize,
		palette.TextMuted, palette.Text)
	s.layoutControl(gtx, g, g.controls.maximize, &s.maximize,
		maximizeIcon, maximizeAction, controlIconSize,
		palette.TextMuted, palette.Text)
	s.layoutControl(gtx, g, g.controls.close, &s.close,
		s.icon(resource.IconClose), system.ActionClose, closeIconSize,
		palette.TextMuted, palette.Danger)
}

// layoutBrand paints the torii mark, the active title and the version badge.
func (s *Shell) layoutBrand(gtx layout.Context, g geometry) {
	palette := s.appearance.Palette()
	typo := s.appearance.Typography()
	if typo.Shaper() == nil {
		return
	}
	drawIcon(gtx, s.icon(resource.IconTorii), g.logo, palette.Accent)

	maxWidth := max(1, g.controls.minimize.Min.X-g.titleX-contentGap)
	titleGtx := gtx
	titleGtx.Constraints = layout.Constraints{Max: image.Pt(maxWidth, g.headerClickEnd)}
	macro := op.Record(gtx.Ops)
	dims := kit.Text(titleGtx, typo, appearance.RoleSubtitle, s.title, palette.Text)
	call := macro.Stop()
	stack := op.Offset(image.Pt(g.titleX, g.headerCenterY-dims.Size.Y/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()

	if s.version != "" {
		s.layoutBadge(gtx, g, g.titleX+dims.Size.X+badgeGap)
	}
}

// layoutBadge paints the version pill after the title. The shell never formats
// the version text.
func (s *Shell) layoutBadge(gtx layout.Context, g geometry, left int) {
	palette := s.appearance.Palette()
	typo := s.appearance.Typography()
	maxWidth := max(1, g.controls.minimize.Min.X-left)
	textGtx := gtx
	textGtx.Constraints = layout.Constraints{Max: image.Pt(maxWidth, g.headerClickEnd)}
	macro := op.Record(gtx.Ops)
	dims := kit.Text(textGtx, typo, appearance.RoleCaption, s.version, palette.TextMuted)
	call := macro.Stop()

	width := dims.Size.X + 2*g.px(badgePaddingX)
	height := g.px(badgeHeight)
	pill := rect(left, g.headerCenterY-height/2, left+width, g.headerCenterY+height/2)
	if pill.Empty() || pill.Max.X > g.controls.minimize.Min.X-contentGap {
		return
	}
	fillRounded(gtx, palette.SurfaceAlt, pill, height/2)
	stack := op.Offset(image.Pt(left+g.px(badgePaddingX), g.headerCenterY-dims.Size.Y/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
}

// layoutControl registers one window-control hitbox and paints its icon.
func (s *Shell) layoutControl(gtx layout.Context, g geometry, bounds image.Rectangle,
	click *widget.Clickable, icon *widget.Icon, action system.Action,
	iconSize float32, normal, hover color.NRGBA) {
	if bounds.Empty() {
		return
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()

	stack := op.Offset(bounds.Min).Push(gtx.Ops)
	defer stack.Pop()
	controlGtx := gtx
	controlGtx.Constraints = layout.Exact(bounds.Size())
	click.Layout(controlGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		col := normal
		if hovered {
			col = hover
		}
		size := min(g.px(iconSize), min(bounds.Dx(), bounds.Dy()))
		iconRect := image.Rect(0, 0, size, size).Add(image.Pt(
			(bounds.Dx()-size)/2,
			(bounds.Dy()-size)/2,
		))
		drawIcon(gtx, icon, iconRect, col)
		return layout.Dimensions{Size: bounds.Size()}
	})
	if clicked && s.window != nil {
		s.window.Perform(action)
	}
}

// layoutDragRegions installs system.ActionMove over every header area outside
// the window controls, so dragging the empty surface moves the window.
func (s *Shell) layoutDragRegions(gtx layout.Context, g geometry) {
	minimize, maximize, close := g.controls.minimize, g.controls.maximize, g.controls.close
	regions := [...]image.Rectangle{
		rect(g.inner.Min.X, 0, minimize.Min.X, g.headerClickEnd),
		rect(minimize.Max.X, 0, maximize.Min.X, g.headerClickEnd),
		rect(maximize.Max.X, 0, close.Min.X, g.headerClickEnd),
		rect(close.Max.X, 0, g.size.X, g.headerClickEnd),
	}
	for _, region := range regions {
		if region.Empty() {
			continue
		}
		stack := clip.Rect(region).Push(gtx.Ops)
		system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
		stack.Pop()
	}
}

// icon returns the embedded icon or nil when the shell has no resources.
func (s *Shell) icon(key resource.IconKey) *widget.Icon {
	if s.resources == nil {
		return nil
	}
	return s.resources.Icon(key)
}

func fill(gtx layout.Context, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}

func fillRounded(gtx layout.Context, col color.NRGBA, r image.Rectangle, radius int) {
	if r.Empty() {
		return
	}
	radius = max(0, min(radius, min(r.Dx(), r.Dy())/2))
	paint.FillShape(gtx.Ops, col, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

func drawIcon(gtx layout.Context, icon *widget.Icon, r image.Rectangle, col color.NRGBA) {
	if icon == nil || r.Empty() {
		return
	}
	size := min(r.Dx(), r.Dy())
	if size <= 0 {
		return
	}
	target := image.Rect(0, 0, size, size).Add(r.Min)
	stack := op.Offset(target.Min).Push(gtx.Ops)
	defer stack.Pop()
	iconGtx := gtx
	iconGtx.Constraints = layout.Exact(target.Size())
	icon.Layout(iconGtx, col)
}

func rect(x0, y0, x1, y1 int) image.Rectangle {
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1, y1)
}
