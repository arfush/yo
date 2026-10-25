package ui

import (
	"image"
	"image/color"
	"io"
	"math"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/watchparty"
)

const (
	wpCardWidth    = 448
	wpCardHeight   = 300
	wpCardPadding  = 32
	wpTitleTop     = 34
	wpDescTop      = 92
	wpActionTop    = 166
	wpButtonHeight = 48
	wpButtonGap    = 12
	wpFieldHeight  = 60
	wpErrorTop     = 232
	wpLinkTop      = 232

	wpChipHeight = 22
	wpChipPadX   = 8

	wpRoundSize     = 40
	wpRoundInset    = 10
	wpFieldPadX     = 18
	wpWaitTimeout   = 12 * time.Second
	wpFeedbackDelay = 3 * time.Second
	wpShakeDuration = 520 * time.Millisecond
	wpShakeAmpDp    = 2.4
)

const (
	wpTitleSize    = unit.Sp(24)
	wpDescSize     = unit.Sp(13)
	wpButtonSize   = unit.Sp(13.5)
	wpKeySize      = unit.Sp(16)
	wpErrorSize    = unit.Sp(12.5)
	wpLinkSize     = unit.Sp(12)
	wpWaitSize     = unit.Sp(13.5)
	wpChipSize     = unit.Sp(11)
	wpWaitHintSize = unit.Sp(12)
)

const (
	wpTitleChoice  = "Совместный просмотр"
	wpDescChoice   = "Создай свою или присоединись к чужой комнате"
	wpTitleCreated = "Комната создана"
	wpDescCreated  = "Комната готова — осталось поделиться ключом и можно устраивать аниме-марафон"
	wpTitleJoin    = "Войти в комнату"
	wpDescJoin     = "Введите полученный от друга ключ — и подключайтесь"
	wpWaitingText  = "Ожидаем создателя комнаты"
	wpCreateLabel  = "Создать"
	wpJoinLabel    = "Присоединиться"
)

var (
	wpLinkColor      = color.NRGBA{R: 0x7d, G: 0x77, B: 0x8c, A: 0xff}
	wpLinkHoverColor = color.NRGBA{R: 0xb5, G: 0xae, B: 0xc6, A: 0xff}
)

type watchPartyMode int

const (
	wpChoice watchPartyMode = iota
	wpCreated
	wpJoin
	wpWaiting
)

type partyResultKind int

const (
	partyCreated partyResultKind = iota
	partyJoined
)

type partyResult struct {
	kind partyResultKind
	err  error
}

// watchPartyWindow is the whole watch-party flow: the create/join choice, the
// shared-key card, the join form and the waiting screen a guest sees before the
// host starts a title.
type watchPartyWindow struct {
	typo   *typography
	window *app.Window
	party  *partyManager

	mode watchPartyMode

	providerButton widget.Clickable
	createButton   widget.Clickable
	joinButton     widget.Clickable
	copyButton     widget.Clickable
	nextButton     widget.Clickable
	deleteButton   widget.Clickable
	backClick      widget.Clickable
	backHover      hoverAnim
	editor         widget.Editor
	// keyEditor displays the created room key. It is read-only but selectable,
	// so the key can be copied with Ctrl+C straight from the card.
	keyEditor widget.Editor
	keySelect bool

	focusField bool
	busy       bool
	errorText  string
	errorAt    time.Time
	// join feedback disappears together with the red button; create feedback
	// stays until the next attempt because it usually means the relay is
	// misconfigured and the text is the only hint the user gets.
	joinFeedback bool

	waitSince time.Time

	results chan partyResult

	hidden func() bool
	onBack func()
}

func newWatchPartyWindow(typo *typography, w *app.Window, party *partyManager) *watchPartyWindow {
	screen := &watchPartyWindow{
		typo:    typo,
		window:  w,
		party:   party,
		results: make(chan partyResult, 4),
	}
	screen.editor.SingleLine = true
	screen.editor.InputHint = key.HintText
	screen.keyEditor.SingleLine = true
	screen.keyEditor.ReadOnly = true
	snapshot := party.Snapshot()
	switch {
	case !snapshot.Active:
		screen.mode = wpChoice
	case snapshot.Role == watchparty.Host:
		screen.mode = wpCreated
	default:
		screen.mode = wpWaiting
		screen.waitSince = time.Now()
	}
	// The guest may have been on the main screen when the host left; the
	// reason waits in the manager until the card is opened again.
	if text := party.takeClosedNotice(); text != "" {
		screen.roomClosed(text)
	}
	return screen
}

// Close only detaches the screen; the room stays open so the main-page control
// can reopen the card with the same key.
func (w *watchPartyWindow) Close() {}

func (w *watchPartyWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *watchPartyWindow) layout(gtx layout.Context) {
	w.drain()
	if text := w.party.takeClosedNotice(); text != "" {
		w.roomClosed(text)
	}
	w.handleEscape(gtx)
	w.expireFeedback(gtx)

	snapshot := w.party.Snapshot()
	if w.mode == wpWaiting && snapshot.Active && snapshot.Role == watchparty.Guest {
		w.layoutWaiting(gtx, snapshot)
		return
	}

	scale := gtx.Metric.PxPerDp
	if scale == 0 {
		scale = 1
	}
	px := func(v float32) int {
		return int(math.Round(float64(v) * float64(scale)))
	}

	contentSize := gtx.Constraints.Max
	cardWidth := px(wpCardWidth)
	cardHeight := px(wpCardHeight)
	if maxWidth := contentSize.X - px(32); cardWidth > maxWidth {
		cardWidth = maxWidth
	}
	if maxHeight := contentSize.Y - px(32); cardHeight > maxHeight {
		cardHeight = maxHeight
	}
	if cardWidth <= 0 || cardHeight <= 0 {
		return
	}

	card := image.Rect(
		(contentSize.X-cardWidth)/2,
		(contentSize.Y-cardHeight)/2,
		(contentSize.X-cardWidth)/2+cardWidth,
		(contentSize.Y-cardHeight)/2+cardHeight,
	)

	stack := op.Offset(card.Min).Push(gtx.Ops)
	defer stack.Pop()

	drawFirstRunCard(gtx, card.Size(), px)

	switch w.mode {
	case wpChoice:
		w.layoutChoice(gtx, cardWidth, px)
	case wpCreated:
		w.layoutCreated(gtx, cardWidth, px)
	default:
		w.layoutJoin(gtx, cardWidth, px)
	}
}

// layoutCardText draws the shared title and description block.
func (w *watchPartyWindow) layoutCardText(gtx layout.Context, cardWidth int, px func(float32) int, title, description string) {
	w.typo.draw(gtx, title, font.Font{Typeface: fontFamily, Weight: font.Bold}, wpTitleSize, titleColor, image.Pt(px(wpCardPadding), px(wpTitleTop)))

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	width := cardWidth - 2*px(wpCardPadding)
	lines := wrapFirstRunText(gtx, w.typo, description, face, wpDescSize, width)
	advance := int(math.Round(float64(gtx.Sp(wpDescSize)) * 1.25))
	if advance < 1 {
		advance = 1
	}
	for index, line := range lines {
		w.typo.draw(gtx, line, face, wpDescSize, firstRunSubtitleColor, image.Pt(px(wpCardPadding), px(wpDescTop)+index*advance))
	}
}

func (w *watchPartyWindow) layoutChoice(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleChoice, wpDescChoice)
	w.layoutProviderChip(gtx, cardWidth, px)

	gap := px(wpButtonGap)
	left := px(wpCardPadding)
	right := cardWidth - px(wpCardPadding)
	half := (right - left - gap) / 2
	top := px(wpActionTop)
	bottom := top + px(wpButtonHeight)
	if half <= 0 {
		return
	}

	create := image.Rect(left, top, left+half, bottom)
	join := image.Rect(left+half+gap, top, right, bottom)

	errored, shake := buttonErrorFeedback(gtx.Now, w.errorAt, !w.joinFeedback)
	// Both actions open a room, so they share the primary accent instead of
	// competing for attention.
	if w.layoutActionButton(gtx, create, &w.createButton, wpCreateLabel, errored, shake) {
		w.create()
	}
	if w.layoutActionButton(gtx, join, &w.joinButton, wpJoinLabel, false, 0) {
		w.openJoin()
	}

	drawCenteredError(gtx, w.typo, w.errorText, cardWidth, px)
	if shake != 0 {
		// Only the shake needs continuous frames; the red tint can wait for the
		// frame that clears it.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

func (w *watchPartyWindow) layoutCreated(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleCreated, wpDescCreated)

	field := image.Rect(px(wpCardPadding), px(wpActionTop), cardWidth-px(wpCardPadding), px(wpActionTop)+px(wpFieldHeight))
	key := w.party.Snapshot().Key
	if !field.Empty() {
		w.layoutKeyField(gtx, field, key, false, false)
	}

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, ink := w.typo.measure(gtx, "Удалить комнату", face, wpLinkSize)
	linkWidth := ink.Dx() + px(16)
	linkHeight := ink.Dy() + px(8)
	centerX := cardWidth / 2
	link := image.Rect(
		centerX-linkWidth/2,
		px(wpLinkTop),
		centerX-linkWidth/2+linkWidth,
		px(wpLinkTop)+linkHeight,
	)
	if link.Empty() {
		return
	}
	clicked := w.deleteButton.Clicked(gtx)
	col := wpLinkColor
	if w.deleteButton.Hovered() {
		col = wpLinkHoverColor
	}
	w.typo.drawCentered(gtx, "Удалить комнату", face, wpLinkSize, col, centerOf(link))
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(link.Size())
	stack := op.Offset(link.Min).Push(gtx.Ops)
	w.deleteButton.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: link.Size()}
	})
	stack.Pop()
	if clicked {
		w.party.Delete()
		w.mode = wpChoice
		w.errorText = ""
		w.window.Invalidate()
	}
}

func (w *watchPartyWindow) layoutJoin(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleJoin, wpDescJoin)

	if w.focusField {
		w.focusField = false
		gtx.Execute(key.FocusCmd{Tag: &w.editor})
	}

	field := image.Rect(px(wpCardPadding), px(wpActionTop), cardWidth-px(wpCardPadding), px(wpActionTop)+px(wpFieldHeight))
	if !field.Empty() {
		errored, _ := buttonErrorFeedback(gtx.Now, w.errorAt, false)
		if w.layoutKeyField(gtx, field, w.editor.Text(), true, errored) {
			w.join()
		}
	}

	drawCenteredError(gtx, w.typo, w.errorText, cardWidth, px)
}

// drawCenteredError paints the feedback line in the middle of the card, between
// the two action buttons. Long relay errors are shortened to the card width.
func drawCenteredError(gtx layout.Context, typo *typography, text string, cardWidth int, px func(float32) int) {
	if text == "" || typo == nil {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	text = upperFirst(truncateText(gtx, typo, text, face, wpErrorSize, cardWidth-2*px(wpCardPadding)))
	if text == "" {
		return
	}
	_, ink := typo.measure(gtx, text, face, wpErrorSize)
	typo.draw(gtx, text, face, wpErrorSize, tokenErrorColor1, image.Pt((cardWidth-ink.Dx())/2, px(wpErrorTop)))
}

// layoutProviderChip draws the provider name next to the title. It toggles on
// click only when a second relay is configured for this build.
func (w *watchPartyWindow) layoutProviderChip(gtx layout.Context, cardWidth int, px func(float32) int) {
	label := w.party.Provider().Label()
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, wpTitleChoice, titleFace, wpTitleSize)
	chipHeight := px(wpChipHeight)
	if chipHeight <= 0 {
		return
	}
	chipTop := px(wpTitleTop) + (titleInk.Dy()-chipHeight)/2

	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	// Size the chip from the widest relay name rather than the current one so
	// switching Cloudflare/Supabase does not resize neighbouring elements.
	labels := []string{label}
	if w.party.HasProviderChoice() {
		labels = nil
		for _, provider := range watchparty.Providers() {
			labels = append(labels, provider.Label())
		}
	}
	chipInk := 0
	for _, name := range labels {
		if _, ink := w.typo.measure(gtx, name, face, wpChipSize); ink.Dx() > chipInk {
			chipInk = ink.Dx()
		}
	}
	if chipInk == 0 {
		return
	}
	chipWidth := chipInk + 2*px(wpChipPadX)
	maxWidth := cardWidth - 2*px(wpCardPadding) - titleInk.Dx() - px(12)
	if maxWidth <= 0 || chipWidth > maxWidth {
		return
	}
	chip := image.Rect(cardWidth-px(wpCardPadding)-chipWidth, chipTop, cardWidth-px(wpCardPadding), chipTop+chipHeight)
	if chip.Empty() {
		return
	}

	clickable := w.party.HasProviderChoice()
	var clicked, hovered bool
	if clickable {
		clicked = w.providerButton.Clicked(gtx)
		hovered = w.providerButton.Hovered()
	}

	stack := op.Offset(chip.Min).Push(gtx.Ops)
	rect := image.Rectangle{Max: chip.Size()}
	radius := chipHeight / 2
	fillRounded(gtx, tokenDomainBorder, rect, radius)
	fill := tokenDomainFill
	if hovered {
		fill = tokenDomainFillHover
	}
	fillRounded(gtx, fill, rect.Inset(px(1)), max(1, radius-px(1)))
	w.typo.drawCentered(gtx, label, face, wpChipSize, firstRunProgressText, image.Pt(chipWidth/2, chipHeight/2))
	stack.Pop()

	if !clickable {
		return
	}

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(chip.Size())
	stack = op.Offset(chip.Min).Push(gtx.Ops)
	w.providerButton.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: chip.Size()}
	})
	stack.Pop()

	if clicked {
		w.party.SetProvider(w.party.Provider().Next())
		w.window.Invalidate()
	}
}

// layoutKeyField draws the rounded field with either a copy or a submit button
// on its right. editable fields host the key editor.
func (w *watchPartyWindow) layoutKeyField(gtx layout.Context, rect image.Rectangle, value string, editable, errored bool) bool {
	radius := gtx.Dp(mainInputRadius)
	border := mainInputBorder
	if editable && gtx.Focused(&w.editor) {
		border = mainInputFocusBorder
	}
	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	if inner.Empty() {
		return false
	}
	fillRounded(gtx, mainInputColor, inner, max(1, radius-gtx.Dp(unit.Dp(1))))

	roundSize := gtx.Dp(unit.Dp(wpRoundSize))
	roundInset := gtx.Dp(unit.Dp(wpRoundInset))
	round := image.Rect(
		inner.Max.X-roundInset-roundSize,
		inner.Min.Y+(inner.Dy()-roundSize)/2,
		inner.Max.X-roundInset,
		inner.Min.Y+(inner.Dy()-roundSize)/2+roundSize,
	)

	lineHeight := gtx.Dp(unit.Dp(22))
	textRect := image.Rect(
		inner.Min.X+gtx.Dp(unit.Dp(wpFieldPadX)),
		inner.Min.Y+(inner.Dy()-lineHeight)/2,
		round.Min.X-gtx.Dp(unit.Dp(10)),
		inner.Min.Y+(inner.Dy()-lineHeight)/2+lineHeight,
	)

	if textRect.Dx() > 0 {
		if editable {
			w.layoutKeyEditor(gtx, textRect)
		} else {
			w.layoutKeyViewer(gtx, textRect, watchparty.NormalizeDisplay(value))
		}
	}

	if round.Empty() {
		return false
	}
	click := &w.copyButton
	if editable {
		click = &w.nextButton
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	stack := op.Offset(round.Min).Push(gtx.Ops)
	w.drawRoundButton(gtx, round.Size(), hovered, errored, func(gtx layout.Context, center image.Point) {
		if editable {
			drawChevronRight(
				gtx,
				center,
				roundSize/3,
				gtx.Dp(unit.Dp(2)),
				color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			)
			return
		}
		// The key icon sits in a wide field, so it can be drawn larger than a
		// footer glyph without touching the circle.
		iconGeometry := makeGeometry(gtx, false, false, false, false)
		iconGeometry.scale *= 1.5
		drawCopyIcon(gtx, center, iconGeometry, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	})
	stack.Pop()

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(round.Size())
	stack = op.Offset(round.Min).Push(gtx.Ops)
	click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: round.Size()}
	})
	stack.Pop()

	if !clicked {
		return false
	}
	if editable {
		return true
	}
	key := watchparty.NormalizeDisplay(value)
	if key == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(key)),
	})
	return false
}

func (w *watchPartyWindow) layoutKeyEditor(gtx layout.Context, rect image.Rectangle) {
	w.syncEditor()
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(rect.Size())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	w.editor.Layout(
		editorGtx,
		w.typo.shaper,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		wpKeySize,
		colorMaterial(gtx, mainTextColor),
		colorMaterial(gtx, mainSelectionColor),
	)
	stack.Pop()
	w.syncEditor()
}

// syncEditor keeps the join field in the canonical room-key form: upper case,
// letters only, at most nine characters. The caret is restored so typing feels
// normal.
func (w *watchPartyWindow) syncEditor() {
	text := w.editor.Text()
	cleaned := watchparty.SanitizeKeyInput(text)
	if cleaned == text {
		return
	}
	start, end := w.editor.Selection()
	w.editor.SetText(cleaned)
	limit := len(cleaned)
	if start > limit {
		start = limit
	}
	if end < start {
		end = start
	}
	if end > limit {
		end = limit
	}
	w.editor.SetCaret(start, end)
	w.window.Invalidate()
}

// layoutKeyViewer shows the created room key in a read-only editor. The key is
// selected as soon as the card opens and the editor keeps keyboard focus, so
// Ctrl+C copies it immediately; clicking the field also works for mouse users.
func (w *watchPartyWindow) layoutKeyViewer(gtx layout.Context, rect image.Rectangle, roomKey string) {
	if roomKey == "" {
		return
	}
	if w.keyEditor.Text() != roomKey {
		w.keyEditor.SetText(roomKey)
		w.keyEditor.SetCaret(0, len(roomKey))
		w.keySelect = true
	}
	if w.keySelect {
		w.keySelect = false
		gtx.Execute(key.FocusCmd{Tag: &w.keyEditor})
	}
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(rect.Size())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	w.keyEditor.Layout(
		editorGtx,
		w.typo.shaper,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		wpKeySize,
		colorMaterial(gtx, mainTextColor),
		colorMaterial(gtx, mainSelectionColor),
	)
	stack.Pop()
}

func (w *watchPartyWindow) drawRoundButton(gtx layout.Context, size image.Point, hovered, errored bool, draw func(layout.Context, image.Point)) {
	rect := image.Rectangle{Max: size}
	if rect.Empty() {
		return
	}
	if errored {
		fillRoundedGradient(gtx, rect, rect, rect.Dy()/2, tokenErrorColor1, tokenErrorColor2)
	} else {
		c1, c2 := mainAccentColor1, mainAccentColor2
		if hovered {
			c1 = lighten(c1, 0x08)
			c2 = lighten(c2, 0x08)
		}
		fillRoundedGradient(gtx, rect, rect, rect.Dy()/2, c1, c2)
	}
	if draw != nil {
		draw(gtx, centerOf(rect))
	}
}

func (w *watchPartyWindow) layoutActionButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, label string, errored bool, shake float32) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	w.drawActionButton(gtx, rect.Size(), click.Hovered(), label, errored, shake)
	stack.Pop()

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(rect.Size())
	stack = op.Offset(rect.Min).Push(gtx.Ops)
	click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()
	return clicked
}

func (w *watchPartyWindow) drawActionButton(gtx layout.Context, size image.Point, hovered bool, label string, errored bool, shake float32) {
	rect := image.Rectangle{Max: size}
	radius := size.Y / 2
	face := font.Font{Typeface: fontFamily, Weight: font.SemiBold}
	labelColor := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	if errored {
		drawGlow(gtx, rect, radius, tokenErrorGlow)
		fillRoundedGradient(gtx, rect, rect, radius, tokenErrorColor1, tokenErrorColor2)
	} else {
		drawFirstRunGlow(gtx, rect, radius)
		c1, c2 := mainAccentColor1, mainAccentColor2
		if hovered {
			c1 = lighten(c1, 0x08)
			c2 = lighten(c2, 0x08)
		}
		fillRoundedGradient(gtx, rect, rect, radius, c1, c2)
	}

	center := image.Pt(size.X/2, size.Y/2)
	if shake != 0 {
		center.X += int(math.Round(float64(shake) * float64(gtx.Dp(unit.Dp(wpShakeAmpDp)))))
	}
	w.typo.drawCentered(gtx, label, face, wpButtonSize, labelColor, center)
}

// waitingTimedOut reports whether the joined key never reached a host. A room
// whose host was seen at least once is governed by the reconnect grace window
// in the session, not by the join form's "room not found" timeout.
func waitingTimedOut(now, since time.Time, snapshot partySnapshot) bool {
	if snapshot.HostSeen || snapshot.HostEverSeen {
		return false
	}
	return now.Sub(since) > wpWaitTimeout
}

func (w *watchPartyWindow) layoutWaiting(gtx layout.Context, snapshot partySnapshot) {
	if w.waitSince.IsZero() {
		w.waitSince = gtx.Now
	}
	if waitingTimedOut(gtx.Now, w.waitSince, snapshot) {
		w.party.Delete()
		w.mode = wpJoin
		w.waitSince = time.Time{}
		w.editor.SetText("")
		w.failJoin("Комната не найдена — проверьте ключ")
		return
	}

	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	mascotSize := int(math.Min(float64(size.Y)*0.46, float64(size.X)*0.34))
	centerX := size.X / 2
	bottom := size.Y/2 + mascotSize/2 - gtx.Dp(unit.Dp(10))
	mascot := image.Rect(
		centerX-mascotSize/2,
		bottom-mascotSize,
		centerX+mascotSize/2,
		bottom,
	)
	drawCatgirl(gtx, mascot, gtx.Now, time.Time{})

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	w.typo.drawCentered(gtx, wpWaitingText, face, wpWaitSize, firstRunSubtitleColor, image.Pt(centerX, bottom+gtx.Dp(unit.Dp(26))))

	if snapshot.Notice != "" {
		w.typo.drawCentered(gtx, upperFirst(snapshot.Notice), face, wpWaitHintSize, tokenErrorColor1, image.Pt(centerX, bottom+gtx.Dp(unit.Dp(48))))
	}

	if w.animationsRun() {
		// The mascot animates at display rate on this screen; 30 fps made the
		// bob look chopped compared to the main window.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

func (w *watchPartyWindow) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(settingsButtonHeight)
	top := (size.Y - height) / 2
	backRect := image.Rect(padX, top, padX+gtx.Dp(unit.Dp(120)), top+height)
	if backRect.Empty() {
		return
	}

	buttonFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, capInk := w.typo.measure(gtx, "Н", buttonFace, settingsButtonSize)
	centerY := float32(height) / 2

	clicked := w.backClick.Clicked(gtx)
	progress := w.backHover.update(gtx, w.backClick.Hovered())
	stack := op.Offset(backRect.Min).Push(gtx.Ops)
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(backRect.Size())
	w.backClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, backRect.Dx(), backRect.Dy())
		radius := gtx.Dp(unit.Dp(8))
		fill := mixColor(titleButtonFill, titleButtonHover, progress)
		border := mixColor(titleButtonBorder, titleButtonHoverBorder, progress)
		fillRounded(gtx, border, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
		strokeRoundedRect(gtx, local.Inset(gtx.Dp(unit.Dp(1))), float32(radius), float32(gtx.Dp(unit.Dp(1))), border)

		_, ink := w.typo.measure(gtx, "Вернуться", buttonFace, settingsButtonSize)
		iconSize := gtx.Dp(unit.Dp(11))
		gap := gtx.Dp(unit.Dp(8))
		groupWidth := float32(iconSize + gap + ink.Dx())
		start := (float32(local.Dx()) - groupWidth) / 2
		drawChevronLeft(
			gtx,
			image.Pt(int(math.Round(float64(start+float32(iconSize)/2))), int(math.Round(float64(centerY)))),
			iconSize,
			gtx.Dp(unit.Dp(1.5)),
			titleButtonText,
		)
		w.typo.paintCentered(
			gtx,
			"Вернуться",
			buttonFace,
			settingsButtonSize,
			titleButtonText,
			f32.Pt(start+float32(iconSize+gap)+float32(ink.Dx())/2, centerY),
			capInk.Dy(),
		)
		return layout.Dimensions{Size: backRect.Size()}
	})
	stack.Pop()

	if clicked && w.onBack != nil {
		w.onBack()
	}
}

func (w *watchPartyWindow) handleEscape(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		event, ok := ev.(key.Event)
		if !ok || event.State != key.Press {
			continue
		}
		if w.onBack != nil {
			w.onBack()
			return
		}
	}
}

func (w *watchPartyWindow) expireFeedback(gtx layout.Context) {
	if w.errorAt.IsZero() {
		return
	}
	if gtx.Now.Sub(w.errorAt) < wpFeedbackDelay {
		gtx.Execute(op.InvalidateCmd{At: w.errorAt.Add(wpFeedbackDelay)})
		return
	}
	w.errorAt = time.Time{}
	if w.joinFeedback {
		w.errorText = ""
	}
}

func (w *watchPartyWindow) drain() {
	for {
		select {
		case result := <-w.results:
			w.busy = false
			if result.err != nil {
				w.errorText = result.err.Error()
				w.errorAt = time.Now()
				w.joinFeedback = result.kind == partyJoined
				w.window.Invalidate()
				continue
			}
			w.errorText = ""
			w.errorAt = time.Time{}
			switch result.kind {
			case partyCreated:
				w.mode = wpCreated
			case partyJoined:
				w.mode = wpWaiting
				w.waitSince = time.Now()
				w.editor.SetText("")
			}
			w.window.Invalidate()
		default:
			return
		}
	}
}

func (w *watchPartyWindow) create() {
	if w.busy {
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	// A create failure is reported on the choice form, not on the join form, so
	// it must not inherit the auto-clear behaviour of join errors.
	w.joinFeedback = false
	provider := w.party.Provider()
	w.window.Invalidate()
	go func() {
		_, err := w.party.Create(provider)
		w.results <- partyResult{kind: partyCreated, err: err}
		w.window.Invalidate()
	}()
}

func (w *watchPartyWindow) openJoin() {
	w.mode = wpJoin
	w.waitSince = time.Time{}
	w.errorText = ""
	w.errorAt = time.Time{}
	w.focusField = true
	w.editor.SetText("")
	w.window.Invalidate()
}

func (w *watchPartyWindow) join() {
	if w.busy {
		return
	}
	raw := strings.TrimSpace(w.editor.Text())
	if raw == "" {
		w.failJoin("Введите ключ комнаты")
		return
	}
	key, err := watchparty.NormalizeKey(raw)
	if err != nil {
		w.failJoin("Ключ — 9 латинских букв, начинается с YO")
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	provider := w.party.Provider()
	w.window.Invalidate()
	go func() {
		err := w.party.Join(provider, key)
		w.results <- partyResult{kind: partyJoined, err: err}
		w.window.Invalidate()
	}()
}

func (w *watchPartyWindow) failJoin(text string) {
	w.errorText = text
	w.errorAt = time.Now()
	w.joinFeedback = true
	w.window.Invalidate()
}

// roomClosed returns the screen to the join form with an explanation. The room
// is already gone by the time this runs, so the waiting layout would otherwise
// fall through to a silent join form.
func (w *watchPartyWindow) roomClosed(text string) {
	w.mode = wpJoin
	w.waitSince = time.Time{}
	w.busy = false
	w.focusField = false
	w.editor.SetText("")
	w.errorText = text
	w.errorAt = time.Time{}
	// Unlike a mistyped key, this reason must stay until the next attempt.
	w.joinFeedback = false
	if w.window != nil {
		w.window.Invalidate()
	}
}

// buttonErrorFeedback reports the three-second red state and, when requested,
// the label shake that goes with it.
func buttonErrorFeedback(now, errorAt time.Time, allowShake bool) (bool, float32) {
	if errorAt.IsZero() {
		return false, 0
	}
	elapsed := now.Sub(errorAt)
	if elapsed < 0 || elapsed >= wpFeedbackDelay {
		return false, 0
	}
	if !allowShake {
		return true, 0
	}
	return true, labelShake(now, errorAt)
}

func labelShake(now, start time.Time) float32 {
	elapsed := now.Sub(start)
	if elapsed < 0 || elapsed >= wpShakeDuration {
		return 0
	}
	p := float32(elapsed) / float32(wpShakeDuration)
	return (1 - p) * float32(math.Sin(6*math.Pi*float64(p)))
}
