package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
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
	wpFeedbackDelay = 5 * time.Second
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
	back         footerNamedButton
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
		back:    newBackFooterButton(),
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

	drawDownloadCard(gtx, card.Size(), px)

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
func (w *partyWindow) deleteRoom() {
	w.party.Delete()
	w.mode = wpChoice
	w.errorText = ""
	w.errorAt = time.Time{}
	if w.window != nil {
		w.window.Invalidate()
	}
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

func (w *partyWindow) shakeRunning(now time.Time) bool {
	if w.errorAt.IsZero() {
		return false
	}
	elapsed := now.Sub(w.errorAt)
	return elapsed >= 0 && elapsed < wpShakeDuration
}

// layoutKeyField draws the room-key field. The value is editable on the join
// form and read-only on the created card.
func waitingTimedOut(now, since time.Time, snapshot partySnapshot) bool {
	if snapshot.HostSeen || snapshot.HostEverSeen {
		return false
	}
	return now.Sub(since) > wpWaitTimeout
}

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

	if w.back.layout(gtx, w.typo, backRect) && w.onBack != nil {
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

// buttonErrorFeedback reports the five-second red state and, when requested,
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
