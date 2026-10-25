package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/version"
)

type shell struct {
	typo *typography
	cfg  app.Config
	icon image.Image

	minimize widget.Clickable
	maximize widget.Clickable
	close    widget.Clickable
	update   widget.Clickable
	settings widget.Clickable
	party    widget.Clickable
	log      widget.Clickable

	minimizeHover hoverAnim
	maximizeHover hoverAnim
	closeHover    hoverAnim
	updateHover   hoverAnim
	settingsHover hoverAnim
	partyHover    hoverAnim
	logHover      hoverAnim

	// updateVisible is set by the application when the cached release is newer
	// than the running version. The settings control is always present on the
	// main screen; the party and update controls appear to its left.
	settingsVisible bool
	updateVisible   bool
	partyVisible    bool
	logVisible      bool
	// partyActive lights the watch-party control while a room is open.
	partyActive bool
	onUpdate    func()
	onSettings  func()
	onParty     func()
	onLog       func()

	// errorText is the user-facing failure shown in the middle of the header.
	// Technical details stay in the log; screens never draw their own errors.
	errorText string
	// logAlertUntil keeps the log button dull red for a few seconds after an
	// error, pointing the user at the details without opening the window.
	logAlertUntil time.Time

	content layout.Widget
	// hidden reports that the window is parked in the notification area, so the
	// header stops polling the cursor for hover highlights.
	hidden func() bool
	// rateLimit draws a temporary phrase in the middle of the header while
	// AnimeLIB asks the client to slow down.
	rateLimit rateLimitBanner
	// footer lets a screen own the footer area, such as the title window.
	footer layout.Widget
	// overlay is drawn after the chrome so a dragged history card can move
	// across the whole window, including over the header and footer.
	overlay func(layout.Context, image.Point)
	// pressTag observes every press without consuming it, so the main screen
	// can drop a history selection when the press lands outside the selected
	// card. onPress receives main-content coordinates.
	pressTag    struct{}
	onPress     func(image.Point)
	clientPress bool
	leftDown    bool
}

func newShell(typo *typography, icon image.Image) *shell {
	return &shell{
		typo: typo,
		cfg:  app.Config{Mode: app.Windowed},
		icon: icon,
	}
}

func (s *shell) setConfig(cfg app.Config) {
	s.cfg = cfg
}

// setError shows a friendly message in the header and highlights the log
// button for five seconds. The caller is responsible for logging the technical
// error before calling this method.
func (s *shell) setError(message string, now time.Time) {
	s.errorText = message
	if message != "" {
		s.logAlertUntil = now.Add(5 * time.Second)
	}
}

func (s *shell) clearError() {
	s.errorText = ""
}

// animationsRun reports whether the chrome may keep scheduling its own frames.
func (s *shell) animationsRun() bool {
	return s.hidden == nil || !s.hidden()
}

func (s *shell) layout(gtx layout.Context, w *app.Window) {
	g := makeGeometry(gtx, s.cfg.Mode != app.Windowed, s.settingsVisible, s.partyVisible, s.updateVisible)
	// The watcher registered last frame has already collected presses for this
	// frame. Reading it before the content lets a press outside the selected
	// history card clear the highlight before row click handlers run.
	s.clientPress = false
	s.readWindowPress(gtx, g)
	s.pollNonClientPress(g)
	s.layoutFrame(gtx, g)
	s.layoutHeader(gtx, w, g)
	s.layoutContent(gtx, g)
	s.layoutFooter(gtx, g)
	if s.overlay != nil {
		overlayGtx := gtx
		overlayGtx.Constraints = layout.Exact(g.size)
		s.overlay(overlayGtx, g.contentRect().Min)
	}
	s.addWindowPressWatcher(gtx, g)
}

// readWindowPress drains observed presses and forwards their content-local
// position to the active screen.
func (s *shell) readWindowPress(gtx layout.Context, g geometry) {
	if s.onPress == nil {
		return
	}
	origin := g.contentRect().Min
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &s.pressTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Kind != pointer.Press {
			continue
		}
		p := image.Pt(int(e.Position.X), int(e.Position.Y)).Sub(origin)
		s.clientPress = true
		s.onPress(p)
	}
}

// pollNonClientPress catches presses that never reach Gio. The empty header is
// a non-client drag area, and Windows may run its move loop before the app gets
// a frame, so both the held state and the "pressed since last poll" bit are
// used. The cursor check keeps delayed client presses from being mistaken for
// non-client ones.
func (s *shell) pollNonClientPress(g geometry) {
	down, pressed := mouseLeftPress()
	inHeader := cursorInRect(image.Rect(0, 0, g.size.X, g.headerClickEnd))
	if (pressed || (down && !s.leftDown)) && s.onPress != nil && !s.clientPress && inHeader {
		s.onPress(image.Pt(-1, -1))
	}
	s.leftDown = down
}

// addWindowPressWatcher covers the whole window with a pass-through pointer
// handler. It is added last, so it sees presses over the chrome, content and
// footer while every other control still receives its normal events.
func (s *shell) addWindowPressWatcher(gtx layout.Context, g geometry) {
	if g.size.X <= 0 || g.size.Y <= 0 {
		return
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: g.size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &s.pressTag)
	area.Pop()
	pass.Pop()
}

// layoutFrame draws the custom window chrome. The outer border is drawn square
// and the inner surface is rounded, leaving a hairline border visible along the
// curved corners. Windows 11 rounds (and clips) the outer window itself.
func (s *shell) layoutFrame(gtx layout.Context, g geometry) {
	fill(gtx, borderColor, image.Rectangle{Max: g.size})

	clipStack := clip.RRect{
		Rect: g.inner,
		NW:   g.cornerRadius,
		NE:   g.cornerRadius,
		SE:   g.cornerRadius,
		SW:   g.cornerRadius,
	}.Push(gtx.Ops)
	fill(gtx, contentColor, g.inner)

	// Header background, followed by its divider.
	fill(gtx, chromeColor, image.Rect(
		g.inner.Min.X,
		g.inner.Min.Y,
		g.inner.Max.X,
		g.headerDivider,
	))
	fill(gtx, dividerColor, image.Rect(
		g.inner.Min.X,
		g.headerDivider,
		g.inner.Max.X,
		g.contentTop,
	))

	// Footer divider, footer, and the content-colored strip above the bottom
	// border.
	fill(gtx, dividerColor, image.Rect(
		g.inner.Min.X,
		g.footerDivider,
		g.inner.Max.X,
		g.footerTop,
	))
	fill(gtx, chromeColor, image.Rect(
		g.inner.Min.X,
		g.footerTop,
		g.inner.Max.X,
		g.footerBottom,
	))
	fill(gtx, contentColor, image.Rect(
		g.inner.Min.X,
		g.stripTop,
		g.inner.Max.X,
		g.inner.Max.Y,
	))

	clipStack.Pop()
}

func (s *shell) layoutHeader(gtx layout.Context, w *app.Window, g geometry) {
	s.layoutLogo(gtx, g)

	// The title is deliberately pinned to a common baseline between the logo
	// and version badge, rather than being laid out as flowing text.
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := s.typo.measure(gtx, "Yonime", titleFace, titleSize)
	titleY := g.headerCenterY - titleInk.Dy()/2
	s.typo.draw(gtx, "Yonime", titleFace, titleSize, titleColor, image.Pt(g.titleX, titleY))

	badgeRight := s.layoutBadge(gtx, g)
	if s.errorText != "" {
		s.layoutHeaderError(gtx, g, badgeRight)
	} else {
		s.rateLimit.layout(gtx, s.typo, g, badgeRight)
	}

	if s.clickControl(gtx, g, g.controls.minimize, &s.minimize, &s.minimizeHover, controlHoverColor, func(gtx layout.Context) {
		s.drawMinimize(gtx, g)
	}) {
		w.Perform(system.ActionMinimize)
	}
	if s.clickControl(gtx, g, g.controls.maximize, &s.maximize, &s.maximizeHover, controlHoverColor, func(gtx layout.Context) {
		if s.cfg.Mode == app.Maximized {
			s.drawRestore(gtx, g)
		} else {
			s.drawMaximize(gtx, g)
		}
	}) {
		if s.cfg.Mode == app.Maximized {
			w.Perform(system.ActionUnmaximize)
		} else {
			w.Perform(system.ActionMaximize)
		}
	}
	if s.clickControl(gtx, g, g.controls.close, &s.close, &s.closeHover, closeHoverColor, func(gtx layout.Context) {
		s.drawClose(gtx, g)
	}) {
		w.Perform(system.ActionClose)
	}

	s.layoutDragRegions(gtx, g)
}

// layoutHeaderError centers the friendly error between the version badge and
// the window controls. It is intentionally a single line: the full details are
// in the log window, while the header only tells the user what went wrong.
func (s *shell) layoutHeaderError(gtx layout.Context, g geometry, badgeRight int) {
	left := badgeRight + g.px(14)
	right := g.controls.minimize.Min.X - g.px(14)
	if right <= left {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := s.typo.measure(gtx, s.errorText, face, mainClearSize)
	width := ink.Dx()
	if width > right-left {
		width = right - left
	}
	x := left + (right-left-width)/2
	y := g.headerCenterY - ink.Dy()/2
	drawTruncatedText(
		gtx,
		s.typo,
		s.errorText,
		face,
		mainClearSize,
		mainDangerColor,
		image.Pt(x, y),
		width,
	)
}

func (s *shell) layoutLogo(gtx layout.Context, g geometry) {
	drawImage(gtx, s.icon, g.logo)
}

// versionBadge is the version pill and the placement of its label.
type versionBadge struct {
	// pill is the outer rectangle, border included.
	pill image.Rectangle
	// ink is the visual bounds of the label in its own coordinates; center is
	// where the painter puts its midpoint.
	ink image.Rectangle
	// textWidth is the typographic width of the label in device pixels. It
	// sizes the pill and is the value handed to mpv, which cannot measure text
	// itself.
	textWidth float32
	// center is the fractional point the label ink is centered on.
	center f32.Point
	// cap is the distance from the top of the label ink to the baseline. The
	// label is centered by this block, not by the full ink: the dot in "v1.2"
	// must not lift the digits above the middle of the pill.
	cap int
}

// measureVersionBadge sizes the pill from the label advance so any version
// string, short or long, keeps the same padding around its text. The advance,
// not the ink, is what both renderers can agree on: mpv centres its ASS line
// by it as well.
func measureVersionBadge(gtx layout.Context, typo *typography, text string, g geometry) (versionBadge, bool) {
	glyphs, ink := typo.measure(gtx, text, font.Font{Typeface: fontFamily, Weight: font.Medium}, badgeSize)
	if ink.Empty() || len(glyphs) == 0 {
		return versionBadge{}, false
	}
	last := glyphs[len(glyphs)-1]
	advance := float32(last.X+last.Advance) / 64

	border := g.leastPx(1)
	pad := g.px(badgePaddingX)
	height := g.px(badgeHeight)
	width := int(math.Round(float64(advance))) + 2*pad + 2*border
	top := g.headerCenterY - height/2
	pill := image.Rect(g.badgeX, top, g.badgeX+width, top+height)
	inner := pill.Inset(border)

	baseline := int(glyphs[0].Y)
	cap := baseline - ink.Min.Y
	if cap <= 0 || cap > ink.Dy() {
		cap = ink.Dy()
	}

	return versionBadge{
		pill:      pill,
		ink:       ink,
		textWidth: advance,
		center: f32.Pt(
			float32(inner.Min.X)+float32(inner.Dx())/2,
			float32(inner.Min.Y)+float32(inner.Dy())/2,
		),
		cap: cap,
	}, true
}

// layoutBadge draws the version pill and returns its right edge so the
// rate-limit banner can stay clear of a long version string.
func (s *shell) layoutBadge(gtx layout.Context, g geometry) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	badgeText := version.String()
	badge, ok := measureVersionBadge(gtx, s.typo, badgeText, g)
	if !ok {
		return g.badgeX
	}

	border := g.leastPx(1)
	radius := badge.pill.Dy() / 2
	fillRounded(gtx, badgeBorderColor, badge.pill, radius)

	inner := badge.pill.Inset(border)
	innerRadius := radius - border
	if innerRadius < 0 {
		innerRadius = 0
	}
	fillRounded(gtx, badgeFillColor, inner, innerRadius)

	s.typo.paintCentered(gtx, badgeText, face, badgeSize, badgeTextColor, badge.center, badge.cap)
	return badge.pill.Max.X
}

func (s *shell) clickControl(gtx layout.Context, g geometry, rect image.Rectangle, control *widget.Clickable, hover *hoverAnim, hoverColor color.NRGBA, draw func(layout.Context)) bool {
	clicked := control.Clicked(gtx)
	hovered := control.Hovered()
	if hovered && !cursorInRect(rect) {
		hovered = false
	}
	progress := hover.update(gtx, hovered)

	// The empty parts of the header are non-client drag areas, so Windows stops
	// delivering pointer events as soon as the cursor leaves a control for the
	// header. Poll the real cursor while a highlight is visible to clear it.
	if hoverPollingSupported() && hover.target > 0 && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(hoverPollInterval)})
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(rect.Size())
	control.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		drawControlHover(gtx, g, progress, hoverColor)
		draw(gtx)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()

	return clicked
}

func (s *shell) layoutDragRegions(gtx layout.Context, g geometry) {
	minimize, maximize, close := g.controls.minimize, g.controls.maximize, g.controls.close
	regions := []image.Rectangle{
		image.Rect(0, 0, minimize.Min.X, g.headerClickEnd),
		image.Rect(minimize.Max.X, 0, maximize.Min.X, g.headerClickEnd),
		image.Rect(maximize.Max.X, 0, close.Min.X, g.headerClickEnd),
		image.Rect(close.Max.X, 0, g.size.X, g.headerClickEnd),
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

func (s *shell) layoutContent(gtx layout.Context, g geometry) {
	if s.content == nil {
		return
	}
	bounds := g.contentRect()
	if bounds.Empty() {
		return
	}

	stack := op.Offset(bounds.Min).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(bounds.Size())
	contentClip := clip.Rect(image.Rectangle{Max: bounds.Size()}).Push(gtx.Ops)
	s.content(gtx)
	contentClip.Pop()
	stack.Pop()
}

func (s *shell) layoutFooter(gtx layout.Context, g geometry) {
	if s.footer != nil {
		bounds := image.Rect(g.inner.Min.X, g.footerTop, g.inner.Max.X, g.footerBottom)
		if bounds.Empty() {
			return
		}
		stack := op.Offset(bounds.Min).Push(gtx.Ops)
		defer stack.Pop()
		fgtx := gtx
		fgtx.Constraints = layout.Exact(bounds.Size())
		clipStack := clip.Rect(image.Rectangle{Max: bounds.Size()}).Push(gtx.Ops)
		s.footer(fgtx)
		clipStack.Pop()
		return
	}
	if s.settingsVisible && !g.settingsRect.Empty() {
		opened := s.layoutFooterControl(gtx, g, g.settingsRect, &s.settings, &s.settingsHover, false, false, func(gtx layout.Context, local image.Rectangle, progress float32, _ color.NRGBA) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			drawSettingsIcon(gtx, centerOf(local), g, icon)
		})
		if opened {
			if s.onSettings != nil {
				s.onSettings()
			}
			// Opening settings replaces the main footer immediately; do not
			// paint the update control on top of the new screen.
			return
		}
	}
	if s.logVisible && !g.logRect.Empty() {
		alert := !s.logAlertUntil.IsZero() && gtx.Now.Before(s.logAlertUntil)
		if alert && s.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: s.logAlertUntil})
		}
		if s.layoutFooterControl(gtx, g, g.logRect, &s.log, &s.logHover, false, alert, func(gtx layout.Context, local image.Rectangle, progress float32, _ color.NRGBA) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			if alert {
				icon = mixColor(footerLogAlertIcon, footerControlHoverIconColor, progress)
			}
			drawLogIcon(gtx, centerOf(local), g, icon)
		}) && s.onLog != nil {
			s.onLog()
		}
	}
	if s.partyVisible && !g.partyRect.Empty() {
		active := s.partyActive
		if s.layoutFooterControl(gtx, g, g.partyRect, &s.party, &s.partyHover, active, false, func(gtx layout.Context, local image.Rectangle, progress float32, background color.NRGBA) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			if active {
				icon = mixColor(footerPartyActiveIcon, footerControlHoverIconColor, progress)
			}
			drawPeopleIcon(gtx, centerOf(local), g, background, icon)
		}) && s.onParty != nil {
			s.onParty()
		}
	}
	if s.updateVisible && !g.updateRect.Empty() {
		if s.layoutFooterControl(gtx, g, g.updateRect, &s.update, &s.updateHover, false, false, func(gtx layout.Context, local image.Rectangle, progress float32, _ color.NRGBA) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			drawDownloadIcon(
				gtx,
				image.Pt(local.Dx()/2, local.Dy()/2),
				g,
				icon,
			)
		}) && s.onUpdate != nil {
			s.onUpdate()
		}
	}
}

// layoutFooterControl draws one round footer button and reports a click.
// active tints the button for a control that is currently holding state, such
// as an open watch-party room.
func (s *shell) layoutFooterControl(gtx layout.Context, g geometry, rect image.Rectangle, control *widget.Clickable, hover *hoverAnim, active, alert bool, draw func(layout.Context, image.Rectangle, float32, color.NRGBA)) bool {
	if rect.Empty() {
		return false
	}
	clicked := control.Clicked(gtx)
	hovered := control.Hovered()
	if hovered && !cursorInRect(rect) {
		hovered = false
	}
	progress := hover.update(gtx, hovered)
	if hoverPollingSupported() && hover.target > 0 && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(hoverPollInterval)})
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	control.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		fill := mixColor(footerControlFillColor, footerControlHoverFill, progress)
		border := mixColor(footerControlBorderColor, footerControlHoverBorder, progress)
		if active {
			fill = mixColor(footerPartyActiveFill, footerControlHoverFill, progress)
			border = mixColor(footerPartyActiveBorder, footerControlHoverBorder, progress)
		}
		if alert {
			fill = mixColor(footerLogAlertFill, footerControlHoverFill, progress)
			border = mixColor(footerLogAlertBorder, footerControlHoverBorder, progress)
		}
		radius := g.px(float32(mainControlRadius))
		fillRounded(gtx, fill, local, radius)
		strokeRoundedRect(
			gtx,
			local.Inset(g.leastPx(1)),
			float32(radius),
			float32(g.leastPx(1)),
			border,
		)
		if draw != nil {
			draw(gtx, local, progress, fill)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
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
	paint.FillShape(gtx.Ops, col, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

func drawImage(gtx layout.Context, src image.Image, r image.Rectangle) {
	if src == nil || r.Empty() {
		return
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return
	}

	scale := f32.Pt(
		float32(r.Dx())/float32(bounds.Dx()),
		float32(r.Dy())/float32(bounds.Dy()),
	)

	defer clip.Rect(r).Push(gtx.Ops).Pop()
	defer op.Offset(r.Min).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), scale)).Push(gtx.Ops).Pop()

	imageOp := paint.NewImageOp(src)
	imageOp.Filter = paint.FilterLinear
	imageOp.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// drawImageCoverRounded fills a rounded rectangle with an image, cropping the
// overflow instead of stretching the artwork.
func drawImageCoverRounded(gtx layout.Context, src image.Image, r image.Rectangle, radius int) {
	if src == nil || r.Empty() {
		return
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return
	}
	scale := max(
		float32(r.Dx())/float32(bounds.Dx()),
		float32(r.Dy())/float32(bounds.Dy()),
	)
	scaledWidth := float32(bounds.Dx()) * scale
	scaledHeight := float32(bounds.Dy()) * scale
	offset := image.Pt(
		r.Min.X+int((float32(r.Dx())-scaledWidth)/2),
		r.Min.Y+int((float32(r.Dy())-scaledHeight)/2),
	)

	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	defer op.Offset(offset).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(scale, scale))).Push(gtx.Ops).Pop()

	imageOp := paint.NewImageOp(src)
	imageOp.Filter = paint.FilterLinear
	imageOp.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}
