package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yoanime/party"
)

func (w *settingsWindow) layoutToggleRow(
	gtx layout.Context,
	rect image.Rectangle,
	label, note string,
	on bool,
	hover float32,
	click *widget.Clickable,
) {
	if rect.Empty() {
		return
	}
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := w.typo.measure(gtx, label, labelFace, settingsLabelSize)
	noteFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, noteInk := w.typo.measure(gtx, note, noteFace, settingsNoteSize)
	blockHeight := labelInk.Dy()
	if note != "" {
		blockHeight += gtx.Dp(settingsNoteGap) + noteInk.Dy()
	}
	top := rect.Min.Y + (rect.Dy()-blockHeight)/2

	toggleHeight := gtx.Dp(settingsToggleHeight)
	toggle := image.Rect(
		rect.Max.X-gtx.Dp(settingsPanelPadX)-gtx.Dp(settingsToggleWidth),
		rect.Min.Y+(rect.Dy()-toggleHeight)/2,
		rect.Max.X-gtx.Dp(settingsPanelPadX),
		rect.Min.Y+(rect.Dy()+toggleHeight)/2,
	)
	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	textRight := toggle.Min.X - gtx.Dp(unit.Dp(18))

	w.typo.draw(gtx, label, labelFace, settingsLabelSize, mainTextColor, image.Pt(left, top))
	if note != "" {
		noteTop := top + labelInk.Dy() + gtx.Dp(settingsNoteGap)
		drawTruncatedText(gtx, w.typo, note, noteFace, settingsNoteSize, settingsNoteColor, image.Pt(left, noteTop), textRight-left)
	}

	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		toggleSwitch(gtx, toggle.Sub(rect.Min), on, hover)
	})
}

// layoutNamePanel edits the nickname shown to the other side of a room. An
// empty name would leave the other person without a hint about who is hosting,
// so the field is validated on commit.
func (w *settingsWindow) layoutRelayPanels(gtx layout.Context, left, right, top int) (int, bool, bool) {
	height := gtx.Dp(settingsPanelHeight)
	gap := gtx.Dp(settingsRowGap)
	submitted := false
	focusLost := false

	if w.provider == party.Supabase {
		w.cloudflareFocused = false
		var panel image.Rectangle
		var ok, lost bool

		top += gap
		panel = image.Rect(left, top, right, top+height)
		ok, lost = w.layoutRelayPanel(gtx, panel, "Supabase Endpoint", &w.supabaseURLEditor, "https://...supabase.co", &w.supabaseURLFocused)
		submitted = submitted || ok
		focusLost = focusLost || lost
		top = panel.Max.Y

		top += gap
		panel = image.Rect(left, top, right, top+height)
		ok, lost = w.layoutRelayPanel(gtx, panel, "Supabase Publishable Key", &w.supabaseKeyEditor, "sb_publishable_...", &w.supabaseKeyFocused)
		submitted = submitted || ok
		focusLost = focusLost || lost
		return panel.Max.Y, submitted, focusLost
	}

	w.supabaseURLFocused = false
	w.supabaseKeyFocused = false
	top += gap
	panel := image.Rect(left, top, right, top+height)
	ok, lost := w.layoutRelayPanel(gtx, panel, "Cloudflare Endpoint", &w.cloudflareEditor, "https://...workers.dev", &w.cloudflareFocused)
	return panel.Max.Y, ok, lost
}

// layoutRelayPanel paints one endpoint setting: the name on the left, the
// editor on the right, both vertically centered like the other setting rows.
func (w *settingsWindow) layoutRelayPanel(
	gtx layout.Context,
	rect image.Rectangle,
	label string,
	editor *widget.Editor,
	hint string,
	wasFocused *bool,
) (bool, bool) {
	if rect.Empty() {
		return false, false
	}
	w.drawPanel(gtx, rect)

	panelPadX := gtx.Dp(settingsPanelPadX)
	fieldRect := relayFieldRect(gtx, rect)
	if fieldRect.Empty() {
		return false, false
	}

	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, label, face, settingsLabelSize)
	labelLeft := rect.Min.X + panelPadX
	labelTop := rect.Min.Y + (rect.Dy()-ink.Dy())/2
	drawTruncatedText(
		gtx,
		w.typo,
		label,
		face,
		settingsLabelSize,
		mainTextColor,
		image.Pt(labelLeft, labelTop),
		fieldRect.Min.X-gtx.Dp(settingsLabelGap)-labelLeft,
	)

	submitted := w.layoutSettingsEditor(gtx, fieldRect, editor, hint, &w.relayDirty)
	focusLost := relayFocusLost(gtx, editor, wasFocused)
	return submitted, focusLost
}

// relayFieldRect returns the editor rectangle for one endpoint row: a fixed
// width on the right, vertically centered in the panel.
func relayFieldRect(gtx layout.Context, rect image.Rectangle) image.Rectangle {
	panelPadX := gtx.Dp(settingsPanelPadX)
	fieldHeight := gtx.Dp(settingsFieldHeight)
	fieldWidth := gtx.Dp(settingsFieldWidth)
	if max := (rect.Dx() - 2*panelPadX) / 2; fieldWidth > max {
		fieldWidth = max
	}
	right := rect.Max.X - panelPadX
	return image.Rect(
		right-fieldWidth,
		rect.Min.Y+(rect.Dy()-fieldHeight)/2,
		right,
		rect.Min.Y+(rect.Dy()+fieldHeight)/2,
	)
}

// layoutSettingsEditor paints one editable settings value. Enter and leaving
// the field both commit the block that owns it; typing only marks it dirty.
func (w *settingsWindow) layoutSettingsEditor(gtx layout.Context, rect image.Rectangle, editor *widget.Editor, hint string, dirty *bool) bool {
	if rect.Empty() {
		return false
	}
	submitted := false
	for {
		event, ok := editor.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.ChangeEvent:
			*dirty = true
		case widget.SubmitEvent:
			*dirty = true
			submitted = true
		}
	}

	focused := gtx.Focused(editor)
	inner := drawFieldBox(gtx, rect, focused, false, fieldBoxStyle{
		fill:        tokenDomainFill,
		border:      tokenDomainBorder,
		focusBorder: settingsToggleOnColor,
		radius:      unit.Dp(7),
	})
	if inner.Empty() {
		return submitted
	}
	padX := gtx.Dp(settingsFieldPadX)
	if editor.Len() == 0 && hint != "" {
		face := font.Font{Typeface: fontFamily, Weight: font.Normal}
		_, ink := w.typo.measure(gtx, hint, face, settingsFieldSize)
		drawTruncatedText(
			gtx,
			w.typo,
			hint,
			face,
			settingsFieldSize,
			mainPlaceholderColor,
			image.Pt(inner.Min.X+padX, inner.Min.Y+(inner.Dy()-ink.Dy())/2),
			inner.Dx()-2*padX,
		)
	}

	lineHeight := gtx.Dp(unit.Dp(18))
	textRect := image.Rect(
		inner.Min.X+padX,
		inner.Min.Y+(inner.Dy()-lineHeight)/2,
		inner.Max.X-padX,
		inner.Min.Y+(inner.Dy()-lineHeight)/2+lineHeight,
	)
	if textRect.Empty() {
		return submitted
	}
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(textRect.Size())
	stack := op.Offset(textRect.Min).Push(gtx.Ops)
	editor.Layout(
		editorGtx,
		w.typo.shaper,
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		settingsFieldSize,
		colorMaterial(gtx, mainTextColor),
		colorMaterial(gtx, mainSelectionColor),
	)
	stack.Pop()
	return submitted
}

func relayFocusLost(gtx layout.Context, editor *widget.Editor, wasFocused *bool) bool {
	focused := gtx.Focused(editor)
	lost := *wasFocused && !focused
	*wasFocused = focused
	return lost
}

// saveRelays stores the endpoint block. It runs on focus loss, Enter, provider
// change and when the screen closes; a failed save leaves the dirty flag set so
// the next attempt retries the same text.
