package shell

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/kit"
	"github.com/arfush/yo/ui/resource"
)

// Window sizes. The designed size is also the smallest usable size: the chrome
// and the screens are verified at exactly this width and height.
const (
	InitialWidth  unit.Dp = 833
	InitialHeight unit.Dp = 545
	MinimumWidth          = InitialWidth
	MinimumHeight         = InitialHeight
)

// Chrome geometry, in device-independent dp.
const (
	headerHeight      unit.Dp = 48
	footerHeight      unit.Dp = 42
	controlWidth      unit.Dp = 30
	controlGap        unit.Dp = 12
	controlIconSize   unit.Dp = 24
	closeIconSize     unit.Dp = 28
	controlsInset     unit.Dp = 12
	logoInset         unit.Dp = 16
	logoSize          unit.Dp = 25
	labelGap          unit.Dp = 10
	contentGap        unit.Dp = 14
	versionGap        unit.Dp = 14
	badgeHeight       unit.Dp = 20
	badgePaddingX     unit.Dp = 7
	markRadius        unit.Dp = 6
	markInset         unit.Dp = 3
	markIconDrop      unit.Dp = 1
	siteChevronSize   unit.Dp = 8
	siteChevronGap    unit.Dp = 10
	siteChevronStroke unit.Dp = 1.7
	siteMenuOverhang  unit.Dp = 4
)

// accentTintAlpha dims AccentMuted behind the brand mark.
const accentTintAlpha uint8 = 0x40

// geometry holds every chrome measurement converted to pixels for the current
// frame.
type geometry struct {
	size image.Point

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

// windowControl groups one window-control hitbox with its icon and look.
type windowControl struct {
	bounds   image.Rectangle
	click    *widget.Clickable
	icon     *widget.Icon
	action   system.Action
	iconSize unit.Dp
	normal   color.NRGBA
	hover    color.NRGBA
	label    resource.TextKey
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
func (s *Shell) newGeometry(gtx layout.Context) geometry {
	size := gtx.Constraints.Max
	line := max(1, gtx.Dp(appearance.Hairline))

	g := geometry{size: size}
	g.border = line
	if size.X > 2*g.border && size.Y > 2*g.border {
		g.inner = image.Rect(g.border, g.border, size.X-g.border, size.Y-g.border)
	}

	g.headerDivider = g.border + gtx.Dp(headerHeight-1)
	g.contentTop = g.headerDivider + line
	g.footerBottom = size.Y - g.border - line
	g.footerTop = g.footerBottom - gtx.Dp(footerHeight)
	g.footerDivider = g.footerTop - line
	g.contentBottom = g.footerDivider
	g.stripTop = g.footerBottom
	g.headerClickEnd = g.headerDivider
	if !s.maximized {
		g.cornerRadius = gtx.Dp(appearance.RadiusS)
	}
	g.headerCenterY = (g.border + g.headerDivider) / 2

	logo := gtx.Dp(logoSize)
	logoTop := g.headerCenterY - logo/2
	g.logo = rect(gtx.Dp(logoInset), logoTop, gtx.Dp(logoInset)+logo, logoTop+logo)
	g.titleX = g.logo.Max.X + gtx.Dp(labelGap)

	right := size.X - g.border - gtx.Dp(controlsInset)
	width := gtx.Dp(controlWidth)
	gap := gtx.Dp(controlGap)
	g.controls.close = rect(right-width, 0, right, g.headerClickEnd)
	g.controls.maximize = g.controls.close.Add(image.Pt(-(width + gap), 0))
	g.controls.minimize = g.controls.maximize.Add(image.Pt(-(width + gap), 0))
	return g
}

// brandLayout is the measured site block. It is built before the drag regions
// so the site trigger can be excluded from them. The chevron and the version
// badge sit at anchors reserved for the widest site name, so switching sites
// never moves them.
type brandLayout struct {
	label     op.CallOp
	labelSize image.Point
	chevron   image.Rectangle
	trigger   image.Rectangle
	versionX  int
}

// layoutHeader paints the draggable regions, the brand and the window controls.
func (s *Shell) layoutHeader(gtx layout.Context, g geometry) {
	if g.inner.Empty() {
		return
	}
	brand := s.brandLayout(gtx, g)
	// Drag regions come first, so the trigger and the controls added later sit
	// above them and receive pointer input.
	s.layoutDragRegions(gtx, g, brand.trigger)
	s.layoutBrand(gtx, g, brand)

	palette := s.palette()
	maximizeIcon := s.icon(resource.IconMaximize)
	maximizeAction := system.ActionMaximize
	maximizeLabel := resource.TextWindowMaximize
	if s.maximized {
		maximizeIcon = s.icon(resource.IconUnmaximize)
		maximizeAction = system.ActionUnmaximize
		maximizeLabel = resource.TextWindowUnmaximize
	}
	common := windowControl{
		iconSize: controlIconSize,
		normal:   palette.ControlIcon,
		hover:    palette.ControlIconHover,
	}
	minimize := common
	minimize.bounds = g.controls.minimize
	minimize.click = &s.minimize
	minimize.icon = s.icon(resource.IconMinimize)
	minimize.action = system.ActionMinimize
	minimize.label = resource.TextWindowMinimize
	s.layoutControl(gtx, minimize)

	maximize := common
	maximize.bounds = g.controls.maximize
	maximize.click = &s.maximize
	maximize.icon = maximizeIcon
	maximize.action = maximizeAction
	maximize.label = maximizeLabel
	s.layoutControl(gtx, maximize)

	closeControl := common
	closeControl.bounds = g.controls.close
	closeControl.click = &s.close
	closeControl.icon = s.icon(resource.IconClose)
	closeControl.action = system.ActionClose
	closeControl.iconSize = closeIconSize
	closeControl.hover = palette.Danger
	closeControl.label = resource.TextWindowClose
	s.layoutControl(gtx, closeControl)
}

// brandLayout measures the site block and records its label. The chevron is
// anchored after the widest catalog name, not after the active one, so every
// site keeps the same chevron and version position.
func (s *Shell) brandLayout(gtx layout.Context, g geometry) brandLayout {
	palette := s.palette()
	typo := s.typography()
	if typo.Shaper() == nil {
		return brandLayout{}
	}
	chevronSize := gtx.Dp(siteChevronSize)
	chevronGap := gtx.Dp(siteChevronGap)
	reserve := gtx.Dp(contentGap)
	if s.siteSwitchable() {
		reserve += chevronGap + chevronSize
	}
	maxWidth := max(1, g.controls.minimize.Min.X-g.titleX-reserve)
	textGtx := gtx
	textGtx.Constraints = layout.Constraints{Max: image.Pt(maxWidth, g.headerClickEnd)}

	var b brandLayout
	macro := op.Record(gtx.Ops)
	dims := kit.Text(textGtx, typo, appearance.RoleLabel, s.site.Value().String(), palette.Text)
	b.label = macro.Stop()
	b.labelSize = dims.Size
	if b.labelSize.X <= 0 {
		return b
	}

	anchorWidth := b.labelSize.X
	if s.siteSwitchable() {
		for _, site := range catalog.Sites() {
			if width := measureLabel(gtx, textGtx, typo, site.String(), palette.Text); width > anchorWidth {
				anchorWidth = width
			}
		}
	}
	anchorWidth = min(anchorWidth, maxWidth)
	b.versionX = g.titleX + anchorWidth + gtx.Dp(versionGap)
	if s.siteSwitchable() {
		left := g.titleX + anchorWidth + chevronGap
		top := g.headerCenterY - chevronSize/2
		b.chevron = rect(left, top, left+chevronSize, top+chevronSize)
		right := b.chevron.Max.X + gtx.Dp(siteMenuOverhang)
		b.trigger = rect(g.logo.Min.X, 0, right, g.headerClickEnd)
		b.versionX = b.chevron.Max.X + gtx.Dp(versionGap)
	} else {
		b.trigger = rect(g.logo.Min.X, 0, g.titleX+b.labelSize.X, g.headerClickEnd)
	}
	return b
}

// measureLabel returns the width kit.Text would use for label at the site
// name role without leaving the measurement in the frame.
func measureLabel(gtx, textGtx layout.Context, typo appearance.Typography, label string, col color.NRGBA) int {
	macro := op.Record(gtx.Ops)
	dims := kit.Text(textGtx, typo, appearance.RoleLabel, label, col)
	macro.Stop()
	return dims.Size.X
}

// layoutBrand paints the site mark, the active site name, the switcher chevron
// and the version.
func (s *Shell) layoutBrand(gtx layout.Context, g geometry, b brandLayout) {
	palette := s.palette()
	s.layoutSiteMark(gtx, g.logo, palette)
	if b.labelSize.X <= 0 {
		return
	}
	if s.siteSwitchable() && !b.trigger.Empty() && s.layoutSiteTrigger(gtx, b.trigger) {
		if s.menu.open {
			s.dismissSiteMenu()
		} else {
			s.menu.toggle(b.trigger)
		}
		gtx.Execute(op.InvalidateCmd{})
	}

	stack := op.Offset(image.Pt(g.titleX, g.headerCenterY-b.labelSize.Y/2)).Push(gtx.Ops)
	b.label.Add(gtx.Ops)
	stack.Pop()

	if !b.chevron.Empty() {
		drawChevron(gtx, b.chevron, palette.ControlIcon, s.menu.open)
	}
	if s.version != "" {
		s.layoutVersion(gtx, g, b.versionX)
	}
}

// layoutSiteMark paints the vector site tile: a dim rounded edge, the
// alternative surface and the torii in the site accent.
func (s *Shell) layoutSiteMark(gtx layout.Context, r image.Rectangle, palette appearance.Palette) {
	if r.Empty() {
		return
	}
	line := max(1, gtx.Dp(appearance.Hairline))
	radius := gtx.Dp(markRadius)
	fillRounded(gtx, withAlpha(palette.AccentMuted, accentTintAlpha), r, radius)
	inner := r.Inset(line)
	fillRounded(gtx, palette.SurfaceAlt, inner, max(0, radius-line))
	iconRect := inner.Inset(gtx.Dp(markInset)).Add(image.Pt(0, gtx.Dp(markIconDrop)))
	drawIcon(gtx, s.icon(resource.IconTorii), iconRect, palette.Accent)
}

// layoutVersion paints the version pill after the site block. Its fill is
// tinted by the active palette. The shell never formats the version text.
func (s *Shell) layoutVersion(gtx layout.Context, g geometry, left int) {
	palette := s.palette()
	typo := s.typography()
	line := max(1, gtx.Dp(appearance.Hairline))
	padding := gtx.Dp(badgePaddingX)
	maxWidth := max(1, g.controls.minimize.Min.X-left-gtx.Dp(contentGap)-2*(padding+line))
	textGtx := gtx
	textGtx.Constraints = layout.Constraints{Max: image.Pt(maxWidth, g.headerClickEnd)}
	macro := op.Record(gtx.Ops)
	dims := kit.Text(textGtx, typo, appearance.RoleCaption, s.version, palette.Ink)
	call := macro.Stop()
	if dims.Size.X <= 0 {
		return
	}
	height := gtx.Dp(badgeHeight)
	radius := height / 2
	pill := rect(left, g.headerCenterY-height/2,
		left+dims.Size.X+2*(padding+line), g.headerCenterY+(height+1)/2)
	if pill.Max.X > g.controls.minimize.Min.X-gtx.Dp(contentGap) {
		return
	}
	fillRounded(gtx, palette.Edge, pill, radius)
	inner := pill.Inset(line)
	fillRounded(gtx, palette.Badge, inner, max(0, radius-line))
	text := image.Pt(left+line+padding, g.headerCenterY-dims.Size.Y/2)
	stack := op.Offset(text).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
}

// layoutControl registers one window-control hitbox, its accessibility label
// and paints its icon.
func (s *Shell) layoutControl(gtx layout.Context, c windowControl) {
	if c.bounds.Empty() {
		return
	}
	clicked := c.click.Clicked(gtx)
	hovered := c.click.Hovered()

	stack := op.Offset(c.bounds.Min).Push(gtx.Ops)
	defer stack.Pop()
	controlGtx := gtx
	controlGtx.Constraints = layout.Exact(c.bounds.Size())
	clipStack := clip.Rect(image.Rectangle{Max: c.bounds.Size()}).Push(gtx.Ops)
	defer clipStack.Pop()
	label := s.locale().Text(c.label)
	c.click.Layout(controlGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		semantic.Button.Add(gtx.Ops)
		semantic.DescriptionOp(label).Add(gtx.Ops)
		col := c.normal
		if hovered {
			col = c.hover
		}
		size := min(gtx.Dp(c.iconSize), min(c.bounds.Dx(), c.bounds.Dy()))
		iconRect := image.Rect(0, 0, size, size).Add(image.Pt(
			(c.bounds.Dx()-size)/2,
			(c.bounds.Dy()-size)/2,
		))
		drawIcon(gtx, c.icon, iconRect, col)
		return layout.Dimensions{Size: c.bounds.Size()}
	})
	if clicked && s.window != nil {
		s.window.Perform(c.action)
	}
}

// layoutDragRegions installs system.ActionMove over every header area outside
// the site trigger and the window controls, so dragging the empty surface moves
// the window.
func (s *Shell) layoutDragRegions(gtx layout.Context, g geometry, trigger image.Rectangle) {
	minimize, maximize, close := g.controls.minimize, g.controls.maximize, g.controls.close
	regions := make([]image.Rectangle, 0, 5)
	if trigger.Empty() {
		regions = append(regions, rect(g.inner.Min.X, 0, minimize.Min.X, g.headerClickEnd))
	} else {
		regions = append(regions,
			rect(g.inner.Min.X, 0, trigger.Min.X, g.headerClickEnd),
			rect(trigger.Max.X, 0, minimize.Min.X, g.headerClickEnd),
		)
	}
	regions = append(regions,
		rect(minimize.Max.X, 0, maximize.Min.X, g.headerClickEnd),
		rect(maximize.Max.X, 0, close.Min.X, g.headerClickEnd),
		rect(close.Max.X, 0, g.size.X, g.headerClickEnd),
	)
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
	if s.resource == nil {
		return nil
	}
	return s.resource.Icon(key)
}

func fill(gtx layout.Context, col color.NRGBA, r image.Rectangle) {
	if r.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}

// withAlpha returns col with its alpha replaced, so a token can be used as a
// dim tint over the surface below.
func withAlpha(col color.NRGBA, alpha uint8) color.NRGBA {
	col.A = alpha
	return col
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

// drawChevron paints the switcher arrow as a stroked vector, so the chrome does
// not depend on an embedded chevron asset. It points down when closed and up
// when open.
func drawChevron(gtx layout.Context, r image.Rectangle, col color.NRGBA, open bool) {
	if r.Empty() {
		return
	}
	x, y := float32(r.Min.X), float32(r.Min.Y)
	w, h := float32(r.Dx()), float32(r.Dy())
	var path clip.Path
	path.Begin(gtx.Ops)
	if open {
		path.MoveTo(f32.Pt(x+w*0.12, y+h*0.64))
		path.LineTo(f32.Pt(x+w*0.5, y+h*0.3))
		path.LineTo(f32.Pt(x+w*0.88, y+h*0.64))
	} else {
		path.MoveTo(f32.Pt(x+w*0.12, y+h*0.36))
		path.LineTo(f32.Pt(x+w*0.5, y+h*0.7))
		path.LineTo(f32.Pt(x+w*0.88, y+h*0.36))
	}
	width := max(1, float32(gtx.Metric.PxPerDp)*float32(siteChevronStroke))
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

func rect(x0, y0, x1, y1 int) image.Rectangle {
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1, y1)
}
