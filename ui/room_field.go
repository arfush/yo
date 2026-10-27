package ui

import (
	"image"
	"image/color"
	"io"
	"math"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yoanime/party"
)

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
	box := fieldBoxStyle{
		fill:        mainInputColor,
		border:      mainInputBorder,
		focusBorder: mainInputFocusBorder,
		errorBorder: mainDangerColor,
		radius:      mainInputRadius,
	}
	if !editable {
		key := party.NormalizeDisplay(view)
		clicked := copyField{
			typo:  w.typo,
			value: key,
			click: &w.copyButton,
			box:   box,
			size:  wpKeySize,
			viewer: func(gtx layout.Context, rect image.Rectangle) {
				w.layoutKeyViewer(gtx, rect, key)
			},
		}.layout(gtx, rect)
		if !clicked || key == "" {
			return false
		}
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(key)),
		})
		return false
	}

	inner := drawFieldBox(gtx, rect, gtx.Focused(editor), errored, box)
	if inner.Empty() {
		return false
	}
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
		submitted = w.layoutRoomEditor(gtx, textRect, editor)
	}
	if round.Empty() {
		return submitted
	}

	click := &w.nextButton
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	style := iconButtonStyle{
		Shape:       iconButtonCircle,
		Fill:        mainAccentColor1,
		FillHover:   lighten(mainAccentColor1, 0x08),
		Border:      mainAccentColor2,
		BorderHover: lighten(mainAccentColor2, 0x08),
		Gradient:    true,
	}
	if errored {
		style.Fill, style.FillHover = tokenErrorColor1, tokenErrorColor1
		style.Border, style.BorderHover = tokenErrorColor2, tokenErrorColor2
	}
	progress := float32(0)
	if hovered {
		progress = 1
	}
	stack := op.Offset(round.Min).Push(gtx.Ops)
	drawIconButtonSurface(gtx, image.Rectangle{Max: round.Size()}, style, progress)
	center := centerOf(image.Rectangle{Max: round.Size()})
	switch {
	case spinner:
		drawSpinner(gtx, center, gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(1.7)), 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	case shake != 0:
		center.X += int(math.Round(float64(shake) * float64(gtx.Dp(unit.Dp(wpShakeAmpDp)))))
		fallthrough
	default:
		drawChevronRight(gtx, center, roundSize/3, gtx.Dp(unit.Dp(2)), color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	}
	stack.Pop()
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(round.Size())
	stack = op.Offset(round.Min).Push(gtx.Ops)
	click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: round.Size()}
	})
	stack.Pop()
	return clicked || submitted
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
