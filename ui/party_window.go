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

	"github.com/arfush/yonime/party"
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
	wpSecretTop    = 128
	wpSecretHeight = 28

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
	wpWaitHintSize = unit.Sp(12)
)

const (
	wpTitleChoice  = "Совместный просмотр"
	wpDescChoice   = "Создай свою или присоединись к чужой комнате"
	wpTitleCreated = "Комната создана"
	wpDescCreated  = "Комната готова — осталось поделиться ключом и можно устраивать аниме-марафон"
	wpDescLocked   = "Комната закрыта секретом — отправьте ключ и секрет гостям"
	wpTitleJoin    = "Войти в комнату"
	wpDescJoin     = "Введите полученный от друга ключ — и подключайтесь"
	wpTitleSecret  = "Комната закрыта"
	wpDescSecret   = "Введите секрет комнаты — тот, что создатель прислал вместе с ключом"
	wpWaitingText  = "Ожидаем создателя комнаты"
	wpCreateLabel  = "Создать"
	wpJoinLabel    = "Присоединиться"
)

var (
	wpLinkColor      = color.NRGBA{R: 0x7d, G: 0x77, B: 0x8c, A: 0xff}
	wpLinkHoverColor = color.NRGBA{R: 0xb5, G: 0xae, B: 0xc6, A: 0xff}
)

type partyMode int

const (
	wpChoice partyMode = iota
	wpCreated
	wpJoin
	wpWaiting
	wpSecret
)

type partyResultKind int

const (
	partyCreated partyResultKind = iota
	partyJoined
	partySecret
)

type partyResult struct {
	kind partyResultKind
	err  error
}

// partyWindow is the whole watch-party flow: the create/join choice, the
// shared-key card, the join form and the waiting screen a guest sees before the
// host starts a title.
type partyWindow struct {
	typo   *typography
	window *app.Window
	party  *partyManager

	mode partyMode

	createButton widget.Clickable
	joinButton   widget.Clickable
	copyButton   widget.Clickable
	nextButton   widget.Clickable
	deleteButton widget.Clickable
	leaveButton  widget.Clickable
	lockButton   widget.Clickable
	rotateButton widget.Clickable
	unlockButton widget.Clickable
	secretCopy   widget.Clickable
	backClick    widget.Clickable
	backHover    hoverAnim
	editor       widget.Editor
	// keyEditor displays the created room key. It is read-only but selectable,
	// so the key can be copied with Ctrl+C straight from the card.
	keyEditor widget.Editor
	keySelect bool
	// secretEditor holds the ten-symbol secret a guest types.
	secretEditor widget.Editor

	focusField  bool
	focusSecret bool
	busy        bool
	errorText   string
	errorAt     time.Time
	// join feedback disappears together with the red button; create feedback
	// stays until the next attempt because it usually means the relay is
	// misconfigured and the text is the only hint the user gets.
	joinFeedback bool

	waitSince time.Time
	waitHat   int

	results chan partyResult

	hidden  func() bool
	onBack  func()
	onError func(error)
}

func newPartyWindow(typo *typography, w *app.Window, manager *partyManager) *partyWindow {
	screen := &partyWindow{
		typo:    typo,
		window:  w,
		party:   manager,
		waitHat: mascotHatNone,
		results: make(chan partyResult, 4),
	}
	screen.editor.SingleLine = true
	screen.editor.Submit = true
	screen.editor.InputHint = key.HintText
	screen.keyEditor.SingleLine = true
	screen.keyEditor.ReadOnly = true
	screen.secretEditor.SingleLine = true
	screen.secretEditor.Submit = true
	screen.secretEditor.InputHint = key.HintText
	snapshot := manager.Snapshot()
	switch {
	case !snapshot.Active:
		screen.mode = wpChoice
	case snapshot.Role == party.Host:
		screen.mode = wpCreated
	case snapshot.NeedsSecret:
		screen.mode = wpSecret
		screen.focusSecret = true
	default:
		screen.enterWaiting()
	}
	// The guest may have been on the main screen when the host left; the
	// reason waits in the manager until the card is opened again.
	if text := manager.takeClosedNotice(); text != "" {
		screen.roomClosed(text)
	}
	return screen
}

// Close only detaches the screen; the room stays open so the main-page control
// can reopen the card with the same key.
func (w *partyWindow) Close() {}

func (w *partyWindow) reportText(message string) {
	w.errorText = message
	if w.onError != nil {
		w.onError(userMessage(message))
	}
}

func (w *partyWindow) reportError(err error) {
	if err == nil {
		return
	}
	w.errorText = err.Error()
	if w.onError != nil {
		w.onError(err)
	}
}

func (w *partyWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *partyWindow) layout(gtx layout.Context) {
	w.drain()
	if text := w.party.takeClosedNotice(); text != "" {
		w.roomClosed(text)
	}
	w.handleEscape(gtx)
	w.expireFeedback(gtx)

	snapshot := w.party.Snapshot()
	if snapshot.Active && snapshot.Role == party.Guest {
		switch {
		case snapshot.NeedsSecret && w.mode != wpSecret:
			w.openSecret()
		case !snapshot.NeedsSecret && w.mode == wpSecret:
			w.enterWaiting()
		}
	} else if !snapshot.Active && (w.mode == wpWaiting || w.mode == wpSecret) {
		// The room vanished without a close notice (for example, another
		// window deleted it). Do not leave a guest typing a secret into a room
		// that no longer exists.
		w.mode = wpJoin
		w.waitSince = time.Time{}
	}
	if w.mode == wpWaiting && snapshot.Active && snapshot.Role == party.Guest && !snapshot.NeedsSecret {
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
		w.layoutCreated(gtx, cardWidth, px, snapshot)
	case wpSecret:
		w.layoutSecret(gtx, cardWidth, px)
	default:
		w.layoutJoin(gtx, cardWidth, px)
	}
}

// layoutCardText draws the shared title and description block.
func (w *partyWindow) layoutCardText(gtx layout.Context, cardWidth int, px func(float32) int, title, description string) {
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

func (w *partyWindow) layoutChoice(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleChoice, wpDescChoice)

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

	if shake != 0 {
		// Only the shake needs continuous frames; the red tint can wait for the
		// frame that clears it.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

func (w *partyWindow) layoutCreated(gtx layout.Context, cardWidth int, px func(float32) int, snapshot partySnapshot) {
	description := wpDescCreated
	if snapshot.Locked {
		description = wpDescLocked
	}
	w.layoutCardText(gtx, cardWidth, px, wpTitleCreated, description)

	field := image.Rect(px(wpCardPadding), px(wpActionTop), cardWidth-px(wpCardPadding), px(wpActionTop)+px(wpFieldHeight))
	if !field.Empty() {
		w.layoutKeyField(gtx, field, snapshot.Key, false, false, 0)
	}

	actions := []wpLinkAction{
		{label: "Закрыть комнату", click: &w.lockButton, onClick: w.lockRoom},
	}
	if snapshot.Locked {
		w.layoutSecretChip(gtx, cardWidth, px, snapshot.Secret)
		actions = []wpLinkAction{
			{label: "Сменить секрет", click: &w.rotateButton, onClick: w.lockRoom},
			{label: "Открыть комнату", click: &w.unlockButton, onClick: w.unlockRoom},
		}
	}
	actions = append(actions, wpLinkAction{label: "Удалить комнату", click: &w.deleteButton, onClick: w.deleteRoom})
	w.layoutLinkRow(gtx, cardWidth, px, actions)
}

// layoutSecretChip shows the host secret in a read-only chip. Clicking it
// copies the formatted value, which is what people paste into a chat.
func (w *partyWindow) layoutSecretChip(gtx layout.Context, cardWidth int, px func(float32) int, secret string) {
	if secret == "" {
		return
	}
	label := party.FormatSecret(secret)
	face := font.Font{Typeface: fontFamily, Weight: font.SemiBold}
	_, ink := w.typo.measure(gtx, label, face, wpKeySize)
	padX := px(14)
	icon := px(14)
	gap := px(8)
	width := ink.Dx() + 2*padX + icon + gap
	height := px(wpSecretHeight)
	if width > cardWidth-2*px(wpCardPadding) {
		return
	}
	rect := image.Rect((cardWidth-width)/2, px(wpSecretTop), (cardWidth+width)/2, px(wpSecretTop)+height)
	if rect.Empty() {
		return
	}
	radius := px(8)
	fillRounded(gtx, mainInputBorder, rect, radius)
	fillRounded(gtx, mainInputColor, rect.Inset(px(1)), radius-px(1))

	clicked := w.secretCopy.Clicked(gtx)
	col := mainTextColor
	if w.secretCopy.Hovered() {
		col = wpLinkHoverColor
	}
	centerY := rect.Min.Y + rect.Dy()/2
	w.typo.draw(gtx, label, face, wpKeySize, col, image.Pt(rect.Min.X+padX, centerY-ink.Dy()/2))
	iconGeometry := makeGeometry(gtx, false, false, false, false)
	drawCopyIcon(gtx, image.Pt(rect.Max.X-padX-icon/2, centerY), iconGeometry, col)

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(rect.Size())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	w.secretCopy.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()
	if clicked {
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(label)),
		})
	}
}

type wpLinkAction struct {
	label   string
	click   *widget.Clickable
	onClick func()
}

// layoutLinkRow centers a row of small links under the card content.
func (w *partyWindow) layoutLinkRow(gtx layout.Context, cardWidth int, px func(float32) int, actions []wpLinkAction) {
	if len(actions) == 0 {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	padX := px(8)
	padY := px(6)
	gap := px(6)
	widths := make([]int, len(actions))
	total := 0
	height := 0
	for index, action := range actions {
		_, ink := w.typo.measure(gtx, action.label, face, wpLinkSize)
		widths[index] = ink.Dx() + 2*padX
		if actionHeight := ink.Dy() + 2*padY; actionHeight > height {
			height = actionHeight
		}
		total += widths[index]
	}
	if len(actions) > 1 {
		total += gap * (len(actions) - 1)
	}
	if total <= 0 || height <= 0 {
		return
	}
	top := px(wpLinkTop)
	x := (cardWidth - total) / 2
	for index, action := range actions {
		rect := image.Rect(x, top, x+widths[index], top+height)
		if !rect.Empty() {
			clicked := action.click.Clicked(gtx)
			col := wpLinkColor
			if action.click.Hovered() {
				col = wpLinkHoverColor
			}
			w.typo.drawCentered(gtx, action.label, face, wpLinkSize, col, centerOf(rect))
			btnGtx := gtx
			btnGtx.Constraints = layout.Exact(rect.Size())
			stack := op.Offset(rect.Min).Push(gtx.Ops)
			action.click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: rect.Size()}
			})
			stack.Pop()
			if clicked && action.onClick != nil {
				action.onClick()
			}
		}
		x += widths[index] + gap
	}
}

func (w *partyWindow) deleteRoom() {
	w.party.Delete()
	w.mode = wpChoice
	w.errorText = ""
	w.errorAt = time.Time{}
	if w.window != nil {
		w.window.Invalidate()
	}
}

func (w *partyWindow) layoutSecret(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleSecret, wpDescSecret)

	if w.focusSecret {
		w.focusSecret = false
		gtx.Execute(key.FocusCmd{Tag: &w.secretEditor})
	}

	field := image.Rect(px(wpCardPadding), px(wpActionTop), cardWidth-px(wpCardPadding), px(wpActionTop)+px(wpFieldHeight))
	errored, shake := buttonErrorFeedback(gtx.Now, w.errorAt, true)
	if !field.Empty() {
		if w.layoutSecretField(gtx, field, errored, shake) {
			w.submitSecret()
		}
	}

	if w.animationsRun() && (w.busy || w.shakeRunning(gtx.Now)) {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
	w.layoutLinkRow(gtx, cardWidth, px, []wpLinkAction{
		{label: "Выйти из комнаты", click: &w.leaveButton, onClick: w.deleteRoom},
	})
}

func (w *partyWindow) lockRoom() {
	if w.busy {
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	w.joinFeedback = false
	w.window.Invalidate()
	go func() {
		_, err := w.party.LockRoom()
		w.results <- partyResult{kind: partyCreated, err: err}
		w.window.Invalidate()
	}()
}

func (w *partyWindow) unlockRoom() {
	if w.busy {
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	w.joinFeedback = false
	w.window.Invalidate()
	go func() {
		err := w.party.UnlockRoom()
		w.results <- partyResult{kind: partyCreated, err: err}
		w.window.Invalidate()
	}()
}

func (w *partyWindow) submitSecret() {
	if w.busy {
		return
	}
	raw := strings.TrimSpace(w.secretEditor.Text())
	if raw == "" {
		w.failSecret("Введите секрет комнаты")
		return
	}
	secret, err := party.NormalizeSecret(raw)
	if err != nil || secret == "" {
		w.failSecret("Секрет — 10 символов из букв и цифр")
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	w.window.Invalidate()
	go func() {
		err := w.party.SubmitSecret(secret)
		w.results <- partyResult{kind: partySecret, err: err}
		w.window.Invalidate()
	}()
}

func (w *partyWindow) failSecret(text string) {
	w.reportText(text)
	w.errorAt = time.Now()
	w.joinFeedback = true
	w.focusSecret = true
	w.secretEditor.SetText("")
	if w.window != nil {
		w.window.Invalidate()
	}
}

// openSecret switches to the secret form after the host locked the room under
// an already joined guest.
func (w *partyWindow) openSecret() {
	w.mode = wpSecret
	w.waitSince = time.Time{}
	w.focusSecret = true
	w.secretEditor.SetText("")
	w.errorText = ""
	w.errorAt = time.Time{}
	w.joinFeedback = true
}

func (w *partyWindow) layoutJoin(gtx layout.Context, cardWidth int, px func(float32) int) {
	w.layoutCardText(gtx, cardWidth, px, wpTitleJoin, wpDescJoin)

	if w.focusField {
		w.focusField = false
		gtx.Execute(key.FocusCmd{Tag: &w.editor})
	}

	field := image.Rect(px(wpCardPadding), px(wpActionTop), cardWidth-px(wpCardPadding), px(wpActionTop)+px(wpFieldHeight))
	errored, shake := buttonErrorFeedback(gtx.Now, w.errorAt, true)
	if !field.Empty() {
		if w.layoutKeyField(gtx, field, w.editor.Text(), true, errored, shake) {
			w.join()
		}
	}

	if w.animationsRun() && (w.busy || w.shakeRunning(gtx.Now)) {
		// The spinner and the shaking chevron both need continuous frames.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

// shakeRunning keeps animation frames coming for the whole shake, including
// its very first frame when the sine offset is still zero.
func (w *partyWindow) shakeRunning(now time.Time) bool {
	if w.errorAt.IsZero() {
		return false
	}
	elapsed := now.Sub(w.errorAt)
	return elapsed >= 0 && elapsed < wpShakeDuration
}

// layoutKeyField draws the room-key field. The value is editable on the join
// form and read-only on the created card.
func (w *partyWindow) layoutKeyField(gtx layout.Context, rect image.Rectangle, value string, editable, errored bool, shake float32) bool {
	return w.layoutRoomField(gtx, rect, &w.editor, value, editable, errored, shake, w.busy && w.mode == wpJoin)
}

// layoutSecretField draws the secret field on the locked-room screen.
func (w *partyWindow) layoutSecretField(gtx layout.Context, rect image.Rectangle, errored bool, shake float32) bool {
	return w.layoutRoomField(gtx, rect, &w.secretEditor, w.secretEditor.Text(), true, errored, shake, w.busy && w.mode == wpSecret)
}

// layoutRoomField draws the rounded field with either a copy or a submit button
// on its right. editor hosts the editable text; view is the text shown and
// copied in the read-only case. spinner replaces the submit chevron while a
// background check runs.
func (w *partyWindow) layoutRoomField(gtx layout.Context, rect image.Rectangle, editor *widget.Editor, view string, editable, errored bool, shake float32, spinner bool) bool {
	radius := gtx.Dp(mainInputRadius)
	border := mainInputBorder
	if editable && gtx.Focused(editor) {
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

	submitted := false
	if textRect.Dx() > 0 {
		if editable {
			submitted = w.layoutRoomEditor(gtx, textRect, editor)
		} else {
			w.layoutKeyViewer(gtx, textRect, party.NormalizeDisplay(view))
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
			if spinner {
				drawSpinner(gtx, center, gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(1.7)), 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
				return
			}
			if shake != 0 {
				center.X += int(math.Round(float64(shake) * float64(gtx.Dp(unit.Dp(wpShakeAmpDp)))))
			}
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

	if editable {
		return clicked || submitted
	}
	if !clicked {
		return false
	}
	key := party.NormalizeDisplay(view)
	if key == "" {
		return false
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(key)),
	})
	return false
}

// layoutRoomEditor draws an editable field and reports whether Enter was
// pressed. Editor.Layout drains its events internally, so SubmitEvent has to be
// collected with a separate Update pass before the layout.
func (w *partyWindow) layoutRoomEditor(gtx layout.Context, rect image.Rectangle, editor *widget.Editor) bool {
	w.syncEditor(editor)
	submitted := false
	for {
		event, ok := editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(rect.Size())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	editor.Layout(
		editorGtx,
		w.typo.shaper,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		wpKeySize,
		colorMaterial(gtx, mainTextColor),
		colorMaterial(gtx, mainSelectionColor),
	)
	stack.Pop()
	w.syncEditor(editor)
	return submitted
}

// syncEditor keeps an input field in its canonical form: the join field holds a
// room key, the secret field a ten-symbol secret. The caret is restored so
// typing feels normal.
func (w *partyWindow) syncEditor(editor *widget.Editor) {
	text := editor.Text()
	cleaned := party.SanitizeKeyInput(text)
	if editor == &w.secretEditor {
		cleaned = party.SanitizeSecretInput(text)
	}
	if cleaned == text {
		return
	}
	start, end := editor.Selection()
	editor.SetText(cleaned)
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
	editor.SetCaret(start, end)
	w.window.Invalidate()
}

// layoutKeyViewer shows the created room key in a read-only editor. The key is
// selected as soon as the card opens and the editor keeps keyboard focus, so
// Ctrl+C copies it immediately; clicking the field also works for mouse users.
func (w *partyWindow) layoutKeyViewer(gtx layout.Context, rect image.Rectangle, roomKey string) {
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

func (w *partyWindow) drawRoundButton(gtx layout.Context, size image.Point, hovered, errored bool, draw func(layout.Context, image.Point)) {
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

func (w *partyWindow) layoutActionButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, label string, errored bool, shake float32) bool {
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

func (w *partyWindow) drawActionButton(gtx layout.Context, size image.Point, hovered bool, label string, errored bool, shake float32) {
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

func (w *partyWindow) layoutWaiting(gtx layout.Context, snapshot partySnapshot) {
	if w.waitSince.IsZero() {
		w.waitSince = gtx.Now
	}
	if w.waitHat < 0 || w.waitHat >= mascotHatCount {
		w.waitHat = nextMascotHat()
	}
	if waitingTimedOut(gtx.Now, w.waitSince, snapshot) {
		w.party.Delete()
		w.mode = wpJoin
		w.waitSince = time.Time{}
		w.failJoin(party.ErrRoomNotFound.Error())
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
	drawCatgirl(gtx, mascot, gtx.Now, w.waitHat, false)

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	waitText := wpWaitingText
	if snapshot.HostName != "" {
		waitText = "Создатель: " + snapshot.HostName
	}
	w.typo.drawCentered(gtx, waitText, face, wpWaitSize, firstRunSubtitleColor, image.Pt(centerX, bottom+gtx.Dp(unit.Dp(26))))

	notice := snapshot.Notice != ""
	if notice {
		w.typo.drawCentered(gtx, upperFirst(snapshot.Notice), face, wpWaitHintSize, tokenErrorColor1, image.Pt(centerX, bottom+gtx.Dp(unit.Dp(48))))
	}
	w.layoutLeaveLink(gtx, centerX, bottom, notice, face)

	if w.animationsRun() {
		// The mascot animates at display rate on this screen; 30 fps made the
		// bob look chopped compared to the main window.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

// layoutLeaveLink paints a small link under the waiting text. It closes the
// guest's room instead of merely hiding the card, which is what "Вернуться"
// does, so a host that never starts a title cannot keep the guest trapped.
func (w *partyWindow) layoutLeaveLink(gtx layout.Context, centerX, bottom int, notice bool, face font.Font) {
	const label = "Выйти"
	_, ink := w.typo.measure(gtx, label, face, wpWaitHintSize)
	linkWidth := ink.Dx() + gtx.Dp(unit.Dp(20))
	linkHeight := ink.Dy() + gtx.Dp(unit.Dp(10))
	if linkWidth <= 0 || linkHeight <= 0 {
		return
	}
	centerY := bottom + gtx.Dp(unit.Dp(50))
	if notice {
		centerY = bottom + gtx.Dp(unit.Dp(74))
	}
	rect := image.Rect(
		centerX-linkWidth/2,
		centerY-linkHeight/2,
		centerX-linkWidth/2+linkWidth,
		centerY+linkHeight/2,
	)
	if rect.Empty() {
		return
	}

	clicked := w.leaveButton.Clicked(gtx)
	col := wpLinkColor
	if w.leaveButton.Hovered() {
		col = wpLinkHoverColor
	}
	w.typo.drawCentered(gtx, label, face, wpWaitHintSize, col, centerOf(rect))
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(rect.Size())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	w.leaveButton.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()

	if !clicked {
		return
	}
	w.party.Delete()
	w.mode = wpChoice
	w.waitSince = time.Time{}
	w.errorText = ""
	w.errorAt = time.Time{}
	w.editor.SetText("")
	w.secretEditor.SetText("")
	w.window.Invalidate()
}

func (w *partyWindow) layoutFooter(gtx layout.Context) {
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

func (w *partyWindow) handleEscape(gtx layout.Context) {
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

func (w *partyWindow) expireFeedback(gtx layout.Context) {
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

func (w *partyWindow) drain() {
	for {
		select {
		case result := <-w.results:
			w.handleResult(result)
			if w.window != nil {
				w.window.Invalidate()
			}
		default:
			return
		}
	}
}

func (w *partyWindow) handleResult(result partyResult) {
	w.busy = false
	if result.err != nil {
		switch result.kind {
		case partyJoined:
			// A failed join is corrected by retyping the key, so the form clears
			// it and uses the same red, shaking feedback for every reason.
			w.failJoin(result.err.Error())
		case partySecret:
			w.failSecret(result.err.Error())
		default:
			w.reportError(result.err)
			w.errorAt = time.Now()
			w.joinFeedback = false
		}
		return
	}
	w.errorText = ""
	w.errorAt = time.Time{}
	switch result.kind {
	case partyCreated:
		w.mode = wpCreated
	case partyJoined:
		w.enterWaiting()
		w.editor.SetText("")
	case partySecret:
		w.secretEditor.SetText("")
		w.enterWaiting()
	}
}

func (w *partyWindow) create() {
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

// enterWaiting starts a fresh wait, either after a join or when the card is
// reopened for a room that is still active. Every opening gets a random hat.
func (w *partyWindow) enterWaiting() {
	w.mode = wpWaiting
	w.waitSince = time.Now()
	w.waitHat = nextMascotHat()
}

func (w *partyWindow) openJoin() {
	w.mode = wpJoin
	w.waitSince = time.Time{}
	w.errorText = ""
	w.errorAt = time.Time{}
	w.focusField = true
	w.focusSecret = false
	w.editor.SetText("")
	w.secretEditor.SetText("")
	w.window.Invalidate()
}

func (w *partyWindow) join() {
	if w.busy {
		return
	}
	raw := strings.TrimSpace(w.editor.Text())
	if raw == "" {
		w.failJoin("Введите ключ комнаты")
		return
	}
	key, err := party.NormalizeKey(raw)
	if err != nil {
		w.failJoin("Ключ — 9 символов из букв и цифр, начинается с YO")
		return
	}
	w.busy = true
	w.errorText = ""
	w.errorAt = time.Time{}
	w.window.Invalidate()
	go func() {
		err := w.party.Join(key)
		w.results <- partyResult{kind: partyJoined, err: err}
		w.window.Invalidate()
	}()
}

func (w *partyWindow) failJoin(text string) {
	w.reportText(text)
	w.errorAt = time.Now()
	w.joinFeedback = true
	w.focusField = true
	w.editor.SetText("")
	if w.window != nil {
		w.window.Invalidate()
	}
}

// roomClosed returns the screen to the join form with an explanation. The room
// is already gone by the time this runs, so the waiting layout would otherwise
// fall through to a silent join form.
func (w *partyWindow) roomClosed(text string) {
	w.mode = wpJoin
	w.waitSince = time.Time{}
	w.busy = false
	w.focusField = false
	w.focusSecret = false
	w.editor.SetText("")
	w.secretEditor.SetText("")
	w.reportText(text)
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
