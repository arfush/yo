package ui

import (
	"image"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/version"
)

type shell struct {
	typo *typography
	cfg  app.Config
	icon image.Image

	// section is the active library mode (anime/manga/ranobe). The header name,
	// the logo tile and the history filter follow it; the choice is persisted by
	// the application.
	section      librarySection
	sectionIcons map[librarySection]image.Image
	// sectionLogo is the clickable brand block (logo + app name) that opens the
	// section switcher menu.
	sectionLogo   widget.Clickable
	sectionOpen   bool
	sectionScrim  widget.Clickable
	sectionItems  [2]widget.Clickable
	onSection     func(librarySection)
	sectionAnchor image.Rectangle

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
	// errorUntil expires the header sentence. logAlertUntil keeps the log
	// button dull red for the same window, pointing the user at the details
	// without opening the window.
	errorUntil    time.Time
	logAlertUntil time.Time

	content layout.Widget
	// hidden reports that the window is parked in the notification area, so the
	// header stops polling the cursor for hover highlights.
	hidden func() bool
	// rateLimit draws a temporary phrase in the middle of the header while
	// AnimeLIB asks the client to slow down.
	rateLimit rateLimitBanner
	// notice lets the active screen provide a transient header line. The shell
	// still owns the priority and the shared layout.
	notice func(time.Time) headerNotice
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

func newShell(typo *typography, icon image.Image, sectionIcons map[librarySection]image.Image) *shell {
	if sectionIcons == nil {
		sectionIcons = map[librarySection]image.Image{sectionAnime: icon}
	}
	return &shell{
		typo:         typo,
		cfg:          app.Config{Mode: app.Windowed},
		icon:         icon,
		sectionIcons: sectionIcons,
	}
}

// sectionIcon returns the header logo tile for the active section.
func (s *shell) sectionIcon() image.Image {
	if img := s.sectionIcons[s.section]; img != nil {
		return img
	}
	return s.icon
}

// selectSection switches the header branding and closes the menu. The
// application is the only caller that knows persistence, so this method never
// fires onSection by itself.
func (s *shell) selectSection(section librarySection) {
	s.section = section
	s.sectionOpen = false
}

// otherSections lists the section switcher options: everything except the
// active one, in a stable order.
func otherSections(current librarySection) [2]librarySection {
	var out [2]librarySection
	index := 0
	for _, candidate := range []librarySection{sectionAnime, sectionManga, sectionRanobe} {
		if candidate == current {
			continue
		}
		out[index] = candidate
		index++
	}
	return out
}

// layoutSectionMenu paints the brand dropdown below the logo. Only the two
// sections other than the active one are listed; selecting one switches the
// whole library through the application callback.
func (s *shell) layoutSectionMenu(gtx layout.Context, g geometry) {
	bounds := image.Rectangle{Max: g.size}
	if bounds.Empty() {
		s.sectionOpen = false
		return
	}
	options := otherSections(s.section)
	pad := gtx.Dp(settingsProviderMenuPad)
	itemHeight := gtx.Dp(unit.Dp(42))
	itemGap := gtx.Dp(unit.Dp(2))
	height := len(options)*itemHeight + (len(options)-1)*itemGap + 2*pad
	anchor := s.sectionAnchor
	if minimum := gtx.Dp(unit.Dp(190)); anchor.Dx() < minimum {
		anchor.Max.X = anchor.Min.X + minimum
	}
	menu := placePopupList(anchor, height, gtx.Dp(unit.Dp(6)), bounds)

	if layoutOverlayScrim(gtx, bounds, &s.sectionScrim) {
		s.sectionOpen = false
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	drawPopupMenu(gtx, menu)

	for index, section := range options {
		item := image.Rect(
			menu.Min.X+pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap),
			menu.Max.X-pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap)+itemHeight,
		)
		if s.layoutSectionItem(gtx, item, section, &s.sectionItems[index]) {
			s.sectionOpen = false
			if s.onSection != nil {
				s.onSection(section)
			} else {
				s.selectSection(section)
			}
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (s *shell) layoutSectionItem(gtx layout.Context, rect image.Rectangle, section librarySection, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	clicked := layoutClickableRect(gtx, rect, click, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(6))
		if click.Hovered() {
			fillRounded(gtx, withAlpha(section.palette().accent1, 0x24), local, radius)
		}
		// Menu options use the same icon and label size as the selected
		// section in the header, so the dropdown never looks shrunken.
		face := font.Font{Typeface: fontFamily, Weight: font.Bold}
		label := section.appName()
		size := gtx.Dp(unit.Dp(24))
		padX := gtx.Dp(unit.Dp(12))
		tile := image.Rect(padX, (local.Dy()-size)/2, padX+size, (local.Dy()-size)/2+size)
		drawImage(gtx, s.sectionIconFor(section), tile)
		_, ink := s.typo.measure(gtx, label, face, titleSize)
		s.typo.draw(gtx, label, face, titleSize, mainTextColor, image.Pt(padX+size+gtx.Dp(unit.Dp(10)), (local.Dy()-ink.Dy())/2))
	})
	return clicked
}

func (s *shell) sectionIconFor(section librarySection) image.Image {
	if img := s.sectionIcons[section]; img != nil {
		return img
	}
	return s.icon
}

func (s *shell) setConfig(cfg app.Config) {
	s.cfg = cfg
}

// setError shows a friendly message in the header and highlights the log
// button for five seconds. The caller is responsible for logging the technical
// error before calling this method.
func (s *shell) setError(message string, now time.Time) {
	s.errorText = message
	if message == "" {
		s.errorUntil = time.Time{}
		s.logAlertUntil = time.Time{}
		return
	}
	s.errorUntil = now.Add(5 * time.Second)
	s.logAlertUntil = s.errorUntil
}

func (s *shell) clearError() {
	s.errorText = ""
	s.errorUntil = time.Time{}
}

// expireError clears the header sentence once its five seconds are up. It is
// separate from layout so the rule can be tested without an event loop.
func (s *shell) expireError(now time.Time) bool {
	if s.errorText == "" || s.errorUntil.IsZero() || now.Before(s.errorUntil) {
		return false
	}
	s.clearError()
	return true
}

// updateError runs before the header is drawn. Scheduling the frame at the
// exact expiry is important: otherwise the sentence would stay forever when
// the window is otherwise idle.
func (s *shell) updateError(gtx layout.Context) {
	if s.expireError(gtx.Now) {
		return
	}
	if s.errorText != "" && !s.errorUntil.IsZero() && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: s.errorUntil})
	}
}

// animationsRun reports whether the chrome may keep scheduling its own frames.
func (s *shell) animationsRun() bool {
	return s.hidden == nil || !s.hidden()
}

func (s *shell) layout(gtx layout.Context, w *app.Window) {
	// Applying the palette every frame is cheap and makes the accents
	// self-healing: a screen or a fast auto-switch that changed the section
	// without running the menu callback cannot leave the mascot, controls or
	// history filter on the previous color.
	applySectionTheme(s.section)
	s.updateError(gtx)
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
	if s.sectionOpen {
		s.layoutSectionMenu(gtx, g)
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
	brand := s.brandLayout(gtx, g)
	g.titleX = brand.titleX
	g.badgeX = brand.badgeX
	s.layoutLogo(gtx, g, brand)

	// The title is deliberately pinned to a common baseline between the logo
	// and version badge, rather than being centered by its ink box. Centering
	// by ink height made "yomanga" jump up because its descender (g) is taller
	// than the descenders of the other two section names.
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	appName := s.section.appName()
	titleY := headerTitleTop(gtx, s.typo, appName, g.headerCenterY, titleFace, titleSize)
	s.typo.draw(gtx, appName, titleFace, titleSize, titleColor, image.Pt(g.titleX, titleY))

	// The chevron sits after a reserved width for the longest section name, so
	// the badge never jumps when the active name changes.
	chevron := gtx.Dp(unit.Dp(8))
	if s.sectionOpen {
		drawChevronUp(gtx, brand.chevron, chevron, gtx.Dp(unit.Dp(1.7)), mainMutedColor)
	} else {
		drawChevronDown(gtx, brand.chevron, chevron, gtx.Dp(unit.Dp(1.7)), mainMutedColor)
	}

	badgeRight := s.layoutBadge(gtx, g)
	notice := headerNotice{}
	if s.notice != nil {
		notice = s.notice(gtx.Now)
	}
	if notice.empty() && s.errorText != "" {
		notice = headerNotice{text: s.errorText, tone: headerNoticeError}
	}
	if notice.empty() {
		if rate, ok := s.rateLimit.notice(gtx, g); ok {
			notice = rate
		}
	}
	layoutHeaderNotice(gtx, s.typo, g, badgeRight, notice)

	if s.clickControl(gtx, g, g.controls.minimize, &s.minimize, &s.minimizeHover, controlHoverColor, func(gtx layout.Context, progress float32) {
		s.drawMinimize(gtx, g, progress)
	}) {
		w.Perform(system.ActionMinimize)
	}
	if s.clickControl(gtx, g, g.controls.maximize, &s.maximize, &s.maximizeHover, controlHoverColor, func(gtx layout.Context, progress float32) {
		if s.cfg.Mode == app.Maximized {
			s.drawRestore(gtx, g, progress)
		} else {
			s.drawMaximize(gtx, g, progress)
		}
	}) {
		if s.cfg.Mode == app.Maximized {
			w.Perform(system.ActionUnmaximize)
		} else {
			w.Perform(system.ActionMaximize)
		}
	}
	if s.clickControl(gtx, g, g.controls.close, &s.close, &s.closeHover, controlHoverColor, func(gtx layout.Context, progress float32) {
		s.drawClose(gtx, g, progress)
	}) {
		w.Perform(system.ActionClose)
	}

	s.layoutDragRegions(gtx, g)
}

// headerTitleTop returns the ink-top offset that puts every section name on
// the same baseline. Descenders differ between "yoanime", "yomanga" and
// "yoranobe", so centering each ink box individually makes one name look
// higher than the others.
func headerTitleTop(gtx layout.Context, typo *typography, text string, centerY int, face font.Font, size unit.Sp) int {
	glyphs, ink := typo.measure(gtx, text, face, size)
	if len(glyphs) == 0 || ink.Empty() {
		return centerY
	}
	reference, referenceInk := typo.measure(gtx, sectionAnime.appName(), face, size)
	capHeight := ink.Dy()
	if len(reference) > 0 && !referenceInk.Empty() {
		capHeight = int(reference[0].Y) - referenceInk.Min.Y
	}
	baseline := int(glyphs[0].Y)
	return centerY + capHeight/2 - baseline + ink.Min.Y
}

// brandLayout reserves the same title width for every section. The icon and
// badge therefore stay at fixed positions while the label changes, and the
// chevron never overlaps either neighbour.
type brandLayout struct {
	titleX  int
	chevron image.Point
	right   int
	badgeX  int
}

func (s *shell) brandLayout(gtx layout.Context, g geometry) brandLayout {
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	titleWidth := 0
	for _, section := range []librarySection{sectionAnime, sectionManga, sectionRanobe} {
		if _, ink := s.typo.measure(gtx, section.appName(), titleFace, titleSize); ink.Dx() > titleWidth {
			titleWidth = ink.Dx()
		}
	}
	// The menu item has 4 dp outer padding plus 12 dp icon inset, so its label
	// starts at menu.Min + 4 + 24 + 10. The header uses the same 10 dp gap to
	// keep the two icon/label groups on the same vertical axes.
	titleX := g.logo.Max.X + gtx.Dp(unit.Dp(10))
	chevronSize := gtx.Dp(unit.Dp(8))
	chevronGap := gtx.Dp(unit.Dp(10))
	chevronCenter := image.Pt(titleX+titleWidth+chevronGap+chevronSize/2, g.headerCenterY)
	right := titleX + titleWidth + chevronGap + chevronSize
	badgeX := right + gtx.Dp(unit.Dp(14))
	return brandLayout{
		titleX:  titleX,
		chevron: chevronCenter,
		right:   right,
		badgeX:  badgeX,
	}
}

func (s *shell) layoutLogo(gtx layout.Context, g geometry, brand brandLayout) {
	// The trigger covers the logo, the app name and the trailing chevron, so
	// the whole brand block opens the menu. It deliberately paints no hover
	// surface: only the dropdown below the header should appear when opened.
	// Keep the menu anchored at its former left edge (13 dp). The visible logo
	// moved right by 10 dp, so the trigger starts 16 dp before it instead of
	// moving the whole dropdown with the logo.
	trigger := image.Rect(
		g.logo.Min.X-gtx.Dp(unit.Dp(16)),
		0,
		brand.right+gtx.Dp(unit.Dp(4)),
		g.headerClickEnd,
	)
	s.sectionAnchor = trigger

	if layoutClickableRect(gtx, trigger, &s.sectionLogo, nil) {
		s.sectionOpen = !s.sectionOpen
		gtx.Execute(op.InvalidateCmd{})
	}
	drawImage(gtx, s.sectionIcon(), g.logo)
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

func (s *shell) layoutDragRegions(gtx layout.Context, g geometry) {
	minimize, maximize, close := g.controls.minimize, g.controls.maximize, g.controls.close
	regions := []image.Rectangle{
		image.Rect(0, 0, s.sectionAnchor.Min.X, g.headerClickEnd),
		image.Rect(s.sectionAnchor.Max.X, 0, minimize.Min.X, g.headerClickEnd),
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
		opened := s.layoutFooterControl(gtx, g, g.settingsRect, &s.settings, &s.settingsHover, func(gtx layout.Context, local image.Rectangle, progress float32) {
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
		if s.layoutFooterControl(gtx, g, g.logRect, &s.log, &s.logHover, func(gtx layout.Context, local image.Rectangle, progress float32) {
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
		if s.layoutFooterControl(gtx, g, g.partyRect, &s.party, &s.partyHover, func(gtx layout.Context, local image.Rectangle, progress float32) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			if active {
				icon = mixColor(footerPartyActiveIcon, footerControlHoverIconColor, progress)
			}
			drawPeopleIcon(gtx, centerOf(local), g, icon)
		}) && s.onParty != nil {
			s.onParty()
		}
	}
	if s.updateVisible && !g.updateRect.Empty() {
		if s.layoutFooterControl(gtx, g, g.updateRect, &s.update, &s.updateHover, func(gtx layout.Context, local image.Rectangle, progress float32) {
			icon := mixColor(footerControlIconColor, footerControlHoverIconColor, progress)
			drawUpdateIcon(gtx, centerOf(local), g, icon)
		}) && s.onUpdate != nil {
			s.onUpdate()
		}
	}
}

// layoutFooterControl draws one round footer button and reports a click. The
// surface is shared by every footer action; active and alert states only
// recolour the glyph, never the button itself.
func (s *shell) layoutFooterControl(gtx layout.Context, g geometry, rect image.Rectangle, control *widget.Clickable, hover *hoverAnim, draw func(layout.Context, image.Rectangle, float32)) bool {
	if rect.Empty() {
		return false
	}
	clicked := control.Clicked(gtx)
	hovered := control.Hovered()
	if hovered && !cursorInRect(rect) {
		hovered = false
	}
	progress := hover.update(gtx, hovered)
	if hoverPollingSupported() && hover.Target > 0 && s.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(hoverPollInterval)})
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	control.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		style := iconButtonStyle{
			Shape:       iconButtonRounded,
			Fill:        footerControlFillColor,
			FillHover:   footerControlHoverFill,
			Border:      footerControlBorderColor,
			BorderHover: footerControlHoverBorder,
			Radius:      mainControlRadius,
		}
		drawIconButtonSurface(gtx, local, style, progress)
		if draw != nil {
			draw(gtx, local, progress)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}
