package ui

import (
	"image"
	"io"
	"math"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yo/party"
)

func (w *partyScreen) layoutCardText(gtx layout.Context, cardWidth int, px func(float32) int, title, description string) {
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

func (w *partyScreen) layoutChoice(gtx layout.Context, cardWidth int, px func(float32) int) {
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

func (w *partyScreen) layoutCreated(gtx layout.Context, cardWidth int, px func(float32) int, snapshot partySnapshot) {
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
func (w *partyScreen) layoutSecretChip(gtx layout.Context, cardWidth int, px func(float32) int, secret string) {
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

func (w *partyScreen) layoutSecret(gtx layout.Context, cardWidth int, px func(float32) int) {
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

func (w *partyScreen) layoutJoin(gtx layout.Context, cardWidth int, px func(float32) int) {
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
func (w *partyScreen) layoutWaiting(gtx layout.Context, snapshot partySnapshot) {
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
		w.failJoin(friendlyError(party.ErrRoomNotFound))
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
	drawCatgirl(gtx, mascot, gtx.Now, w.waitHat, mascotNormal)

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
