package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/watchparty"
)

const (
	settingsPadX         = unit.Dp(28)
	settingsPadTop       = unit.Dp(30)
	settingsTitleSize    = unit.Sp(21)
	settingsLabelSize    = unit.Sp(14)
	settingsNoteSize     = unit.Sp(11.5)
	settingsButtonSize   = unit.Sp(12)
	settingsButtonHeight = unit.Dp(34)
	settingsPanelRadius  = unit.Dp(10)
	settingsPanelHeight  = unit.Dp(84)
	settingsPanelPadX    = unit.Dp(18)
	settingsPanelPadTop  = unit.Dp(15)
	settingsToggleWidth  = unit.Dp(42)
	settingsToggleHeight = unit.Dp(24)
	settingsNoteGap      = unit.Dp(6)
	settingsTitleGap     = unit.Dp(16)
	settingsGroupGap     = unit.Dp(20)
	settingsRowGap       = unit.Dp(10)

	settingsProviderWidth      = unit.Dp(124)
	settingsProviderHeight     = unit.Dp(30)
	settingsProviderItemHeight = unit.Dp(32)
	settingsProviderItemGap    = unit.Dp(2)
	settingsProviderMenuPad    = unit.Dp(4)
	settingsProviderMenuGap    = unit.Dp(6)

	settingsRelayPanelHeight = unit.Dp(108)
	settingsFieldHeight      = unit.Dp(30)
	settingsFieldGap         = unit.Dp(12)
	settingsFieldPadX        = unit.Dp(10)
	settingsFieldLabelGap    = unit.Dp(4)
	settingsFieldSize        = unit.Sp(12)
)

var (
	settingsToggleOnColor = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	settingsToggleOffFill = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	settingsToggleOffEdge = color.NRGBA{R: 0x3a, G: 0x34, B: 0x4a, A: 0xff}
	settingsToggleKnob    = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	settingsToggleKnobOff = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	settingsErrorText     = color.NRGBA{R: 0xf0, G: 0x8a, B: 0x8a, A: 0xff}
	settingsNoteColor     = color.NRGBA{R: 0x8a, G: 0x84, B: 0x9c, A: 0xff}
	settingsMenuFill      = color.NRGBA{R: 0x1d, G: 0x1a, B: 0x27, A: 0xff}
	settingsMenuSelected  = color.NRGBA{R: 0x2b, G: 0x22, B: 0x45, A: 0xff}
)

// settingsValues is the state the settings screen opens with. It is separate
// from the window so the screen itself has no storage dependency.
type settingsValues struct {
	Notifications bool
	SharedControl bool
	Provider      watchparty.Provider
	CloudflareURL string
	SupabaseURL   string
	SupabaseKey   string
}

type settingsWindow struct {
	typo *typography

	enabled  bool
	control  bool
	provider watchparty.Provider

	cloudflareEditor   widget.Editor
	supabaseURLEditor  widget.Editor
	supabaseKeyEditor  widget.Editor
	cloudflareFocused  bool
	supabaseURLFocused bool
	supabaseKeyFocused bool
	relayDirty         bool

	toggleClick  widget.Clickable
	toggleHover  hoverAnim
	controlClick widget.Clickable
	controlHover hoverAnim

	// providerClick is the select itself, providerItems are its options and
	// providerScrim catches clicks outside the open menu.
	providerClick  widget.Clickable
	providerItems  [2]widget.Clickable
	providerScrim  widget.Clickable
	providerOpen   bool
	providerAnchor image.Rectangle

	backClick widget.Clickable
	backHover hoverAnim

	errorText string

	// hidden reports that the window is parked in the notification area, so
	// hover transitions stop requesting frames.
	hidden func() bool

	onBack     func()
	onToggle   func(bool) error
	onControl  func(bool) error
	onProvider func(watchparty.Provider) error
	onRelay    func(watchparty.Config) error
}

func newSettingsWindow(typo *typography, values settingsValues) *settingsWindow {
	w := &settingsWindow{
		typo:     typo,
		enabled:  values.Notifications,
		control:  values.SharedControl,
		provider: values.Provider,
	}
	w.cloudflareEditor.SingleLine = true
	w.cloudflareEditor.Submit = true
	w.cloudflareEditor.InputHint = key.HintURL
	w.cloudflareEditor.SetText(values.CloudflareURL)
	w.supabaseURLEditor.SingleLine = true
	w.supabaseURLEditor.Submit = true
	w.supabaseURLEditor.InputHint = key.HintURL
	w.supabaseURLEditor.SetText(values.SupabaseURL)
	w.supabaseKeyEditor.SingleLine = true
	w.supabaseKeyEditor.Submit = true
	w.supabaseKeyEditor.InputHint = key.HintText
	w.supabaseKeyEditor.SetText(values.SupabaseKey)
	return w
}

func (w *settingsWindow) Close() {
	w.saveRelays()
}

func (w *settingsWindow) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		// Escape dismisses the open provider menu before it leaves the screen.
		if w.providerOpen {
			w.providerOpen = false
			gtx.Execute(op.InvalidateCmd{})
			continue
		}
		if w.onBack != nil {
			w.onBack()
		}
	}
}

func (w *settingsWindow) layout(gtx layout.Context) {
	w.handleKeys(gtx)
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(settingsPadX)
	left := padX
	right := size.X - padX
	if right <= left {
		return
	}

	y := gtx.Dp(settingsPadTop)
	y = w.layoutSectionTitle(gtx, left, y, "Уведомления")
	y = w.layoutNotificationPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsGroupGap)

	y = w.layoutSectionTitle(gtx, left, y, "Совместный просмотр")
	y = w.layoutControlPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsRowGap)
	var relaySubmitted, relayFocusLost bool
	y, relaySubmitted, relayFocusLost = w.layoutProviderPanel(
		gtx,
		image.Rect(left, y, right, y+gtx.Dp(settingsRelayPanelHeight)),
	)
	if relaySubmitted || relayFocusLost {
		w.saveRelays()
	}

	if w.errorText != "" {
		drawTruncatedText(
			gtx,
			w.typo,
			upperFirst(w.errorText),
			font.Font{Typeface: fontFamily, Weight: font.Normal},
			settingsNoteSize,
			settingsErrorText,
			image.Pt(left, y+gtx.Dp(unit.Dp(8))),
			right-left,
		)
	}

	// The menu floats over the panels above, so it is painted last.
	if w.providerOpen {
		w.layoutProviderMenu(gtx, size)
	}
}

// layoutSectionTitle paints one group heading and returns the top of the panel
// that follows it.
func (w *settingsWindow) layoutSectionTitle(gtx layout.Context, left, top int, title string) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, ink := w.typo.measure(gtx, title, face, settingsTitleSize)
	w.typo.draw(gtx, title, face, settingsTitleSize, mainTextColor, image.Pt(left, top))
	return top + ink.Dy() + gtx.Dp(settingsTitleGap)
}

func (w *settingsWindow) drawPanel(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(settingsPanelRadius)
	fillRounded(gtx, titlePanelBorder, rect, radius)
	fillRounded(gtx, titlePanelFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
}

func (w *settingsWindow) layoutNotificationPanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	clicked := w.toggleClick.Clicked(gtx)
	hovered := w.toggleClick.Hovered()
	hover := w.toggleHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setToggle(!w.enabled)
		gtx.Execute(op.InvalidateCmd{})
	}

	w.layoutToggleRow(
		gtx,
		rect,
		"Сообщать о новых сериях/релизе",
		"",
		w.enabled,
		hover,
		&w.toggleClick,
	)
	return rect.Max.Y
}

func (w *settingsWindow) layoutControlPanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	clicked := w.controlClick.Clicked(gtx)
	hovered := w.controlClick.Hovered()
	hover := w.controlHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setControl(!w.control)
		gtx.Execute(op.InvalidateCmd{})
	}

	w.layoutToggleRow(
		gtx,
		rect,
		"Гостевое управление",
		"Гости могут ставить на паузу и перематывать",
		w.control,
		hover,
		&w.controlClick,
	)
	return rect.Max.Y
}

// layoutToggleRow paints a label with an optional note and a toggle on the
// right. clickable covers the whole panel, like the original notification row.
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
		w.drawToggle(gtx, toggle.Sub(rect.Min), on, hover)
	})
}

func (w *settingsWindow) layoutProviderPanel(gtx layout.Context, rect image.Rectangle) (int, bool, bool) {
	w.drawPanel(gtx, rect)
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := w.typo.measure(gtx, "Провайдер", labelFace, settingsLabelSize)
	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	right := rect.Max.X - gtx.Dp(settingsPanelPadX)
	rowTop := rect.Min.Y + gtx.Dp(unit.Dp(12))
	rowHeight := gtx.Dp(settingsProviderHeight)
	labelTop := rowTop + (rowHeight-labelInk.Dy())/2
	w.typo.draw(gtx, "Провайдер", labelFace, settingsLabelSize, mainTextColor, image.Pt(left, labelTop))

	trigger := image.Rect(
		right-gtx.Dp(settingsProviderWidth),
		rowTop,
		right,
		rowTop+rowHeight,
	)
	w.providerAnchor = trigger
	if w.layoutProviderTrigger(gtx, trigger) {
		w.providerOpen = !w.providerOpen
		if w.providerOpen {
			// Typing must not fall through to an endpoint hidden under the menu.
			gtx.Execute(key.FocusCmd{})
		}
		gtx.Execute(op.InvalidateCmd{})
	}

	fieldLabelFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, fieldLabelInk := w.typo.measure(gtx, "Supabase Publishable Key", fieldLabelFace, settingsNoteSize)
	fieldLabelTop := rowTop + rowHeight + gtx.Dp(unit.Dp(8))
	fieldTop := fieldLabelTop + fieldLabelInk.Dy() + gtx.Dp(settingsFieldLabelGap)
	fieldHeight := gtx.Dp(settingsFieldHeight)
	fieldBottom := fieldTop + fieldHeight
	if fieldBottom > rect.Max.Y {
		fieldBottom = rect.Max.Y
	}

	submitted := false
	focusLost := false
	if w.provider == watchparty.Supabase {
		gap := gtx.Dp(settingsFieldGap)
		width := (right - left - gap) / 2
		if width <= 0 {
			return rect.Max.Y, false, false
		}
		w.cloudflareFocused = false

		urlRect := image.Rect(left, fieldTop, left+width, fieldBottom)
		keyRect := image.Rect(left+width+gap, fieldTop, right, fieldBottom)
		w.typo.draw(gtx, "Supabase Endpoint", fieldLabelFace, settingsNoteSize, settingsNoteColor, image.Pt(urlRect.Min.X, fieldLabelTop))
		w.typo.draw(gtx, "Supabase Publishable Key", fieldLabelFace, settingsNoteSize, settingsNoteColor, image.Pt(keyRect.Min.X, fieldLabelTop))
		if w.layoutRelayField(gtx, urlRect, &w.supabaseURLEditor, "https://...supabase.co") {
			submitted = true
		}
		if w.layoutRelayField(gtx, keyRect, &w.supabaseKeyEditor, "sb_publishable_...") {
			submitted = true
		}
		if relayFocusLost(gtx, &w.supabaseURLEditor, &w.supabaseURLFocused) {
			focusLost = true
		}
		if relayFocusLost(gtx, &w.supabaseKeyEditor, &w.supabaseKeyFocused) {
			focusLost = true
		}
		return rect.Max.Y, submitted, focusLost
	}

	w.supabaseURLFocused = false
	w.supabaseKeyFocused = false
	fieldRect := image.Rect(left, fieldTop, right, fieldBottom)
	w.typo.draw(gtx, "Cloudflare Endpoint", fieldLabelFace, settingsNoteSize, settingsNoteColor, image.Pt(fieldRect.Min.X, fieldLabelTop))
	if w.layoutRelayField(gtx, fieldRect, &w.cloudflareEditor, "https://...workers.dev") {
		submitted = true
	}
	if relayFocusLost(gtx, &w.cloudflareEditor, &w.cloudflareFocused) {
		focusLost = true
	}
	return rect.Max.Y, submitted, focusLost
}

// layoutRelayField paints one editable endpoint. Enter and leaving the field
// both commit the whole relay block; typing only marks it dirty.
func (w *settingsWindow) layoutRelayField(gtx layout.Context, rect image.Rectangle, editor *widget.Editor, hint string) bool {
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
			w.relayDirty = true
		case widget.SubmitEvent:
			w.relayDirty = true
			submitted = true
		}
	}

	focused := gtx.Focused(editor)
	radius := gtx.Dp(unit.Dp(7))
	border := tokenDomainBorder
	fill := tokenDomainFill
	if focused {
		border = settingsToggleOnColor
		fill = tokenDomainFillHover
	}
	fillRounded(gtx, border, rect, radius)
	fillRounded(gtx, fill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
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
func (w *settingsWindow) saveRelays() {
	if !w.relayDirty {
		return
	}
	cfg := watchparty.Config{
		CloudflareURL: w.cloudflareEditor.Text(),
		SupabaseURL:   w.supabaseURLEditor.Text(),
		SupabaseKey:   w.supabaseKeyEditor.Text(),
	}
	if w.onRelay != nil {
		if err := w.onRelay(cfg); err != nil {
			w.errorText = err.Error()
			return
		}
	}
	w.relayDirty = false
	w.errorText = ""
}

// layoutProviderTrigger paints the closed select: the current provider and a
// chevron pointing at the menu, which opens on click.
func (w *settingsWindow) layoutProviderTrigger(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	clicked := w.providerClick.Clicked(gtx)
	active := w.providerClick.Hovered() || w.providerOpen

	w.layoutClickableDraw(gtx, rect, &w.providerClick, func(gtx layout.Context) {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(7))
		fill := tokenDomainFill
		if active {
			fill = tokenDomainFillHover
		}
		fillRounded(gtx, tokenDomainBorder, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		label := w.provider.Label()
		_, ink := w.typo.measure(gtx, label, face, settingsButtonSize)
		padX := gtx.Dp(unit.Dp(12))
		w.typo.draw(gtx, label, face, settingsButtonSize, mainTextColor, image.Pt(padX, (rect.Dy()-ink.Dy())/2))

		size := gtx.Dp(unit.Dp(9))
		center := image.Pt(rect.Dx()-padX-size/2, rect.Dy()/2)
		width := gtx.Dp(unit.Dp(1.6))
		if w.providerOpen {
			drawChevronUp(gtx, center, size, width, settingsNoteColor)
			return
		}
		drawChevronDown(gtx, center, size, width, settingsNoteColor)
	})
	return clicked
}

// layoutProviderMenu paints the open select. It is a plain overlay instead of
// an inline list so the panel row keeps its single-line height.
func (w *settingsWindow) layoutProviderMenu(gtx layout.Context, size image.Point) {
	if w.providerAnchor.Empty() {
		return
	}
	providers := watchparty.Providers()
	pad := gtx.Dp(settingsProviderMenuPad)
	itemHeight := gtx.Dp(settingsProviderItemHeight)
	itemGap := gtx.Dp(settingsProviderItemGap)
	menu := placeProviderMenu(
		w.providerAnchor,
		providerMenuHeight(gtx, len(providers)),
		gtx.Dp(settingsProviderMenuGap),
		image.Rectangle{Max: size},
	)

	// The scrim goes first so the options registered after it stay on top and
	// receive their own clicks; anything else closes the menu.
	if w.layoutScrim(gtx, image.Rectangle{Max: size}, &w.providerScrim) {
		w.providerOpen = false
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	radius := gtx.Dp(unit.Dp(8))
	fillRounded(gtx, titlePanelBorder, menu, radius)
	fillRounded(gtx, settingsMenuFill, menu.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	for index, provider := range providers {
		item := image.Rect(
			menu.Min.X+pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap),
			menu.Max.X-pad,
			menu.Min.Y+pad+index*(itemHeight+itemGap)+itemHeight,
		)
		if w.layoutProviderItem(gtx, item, provider, &w.providerItems[index]) {
			w.setProvider(provider)
			w.saveRelays()
			w.providerOpen = false
			gtx.Execute(key.FocusCmd{})
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

// providerMenuHeight sums the options plus the gaps and the menu padding.
func providerMenuHeight(gtx layout.Context, count int) int {
	if count <= 0 {
		return 0
	}
	height := count*gtx.Dp(settingsProviderItemHeight) + 2*gtx.Dp(settingsProviderMenuPad)
	if count > 1 {
		height += (count - 1) * gtx.Dp(settingsProviderItemGap)
	}
	return height
}

func (w *settingsWindow) layoutProviderItem(gtx layout.Context, rect image.Rectangle, provider watchparty.Provider, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	selected := provider == w.provider

	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(6))
		switch {
		case selected:
			fillRounded(gtx, settingsMenuSelected, local, radius)
		case hovered:
			fillRounded(gtx, tokenDomainFillHover, local, radius)
		}

		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		label := provider.Label()
		_, ink := w.typo.measure(gtx, label, face, settingsButtonSize)
		textColor := mainTextColor
		if selected {
			textColor = badgeTextColor
		}
		padX := gtx.Dp(unit.Dp(10))
		w.typo.draw(gtx, label, face, settingsButtonSize, textColor, image.Pt(padX, (rect.Dy()-ink.Dy())/2))
	})
	return clicked
}

// placeProviderMenu returns the menu rectangle for anchor. It prefers to open
// below the trigger and flips above it when the content area would clip it.
func placeProviderMenu(anchor image.Rectangle, height, gap int, bounds image.Rectangle) image.Rectangle {
	below := image.Rect(anchor.Min.X, anchor.Max.Y+gap, anchor.Max.X, anchor.Max.Y+gap+height)
	if below.Max.Y <= bounds.Max.Y {
		return below
	}
	above := image.Rect(anchor.Min.X, anchor.Min.Y-gap-height, anchor.Max.X, anchor.Min.Y-gap)
	if above.Min.Y >= bounds.Min.Y {
		return above
	}
	return below
}

// layoutScrim registers an invisible click target and reports a click on it.
func (w *settingsWindow) layoutScrim(gtx layout.Context, rect image.Rectangle, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: rect.Size()}
	})
	return click.Clicked(gtx)
}

func drawChevronDown(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawChevronVertical(gtx, center, size, width, col, 1)
}

func drawChevronUp(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	drawChevronVertical(gtx, center, size, width, col, -1)
}

func drawChevronVertical(gtx layout.Context, center image.Point, size, width int, col color.NRGBA, direction float32) {
	if size <= 0 {
		return
	}
	half := float32(size) / 2
	tip := f32.Pt(float32(center.X), float32(center.Y)+half*0.55*direction)
	strokeLine(gtx,
		f32.Pt(float32(center.X)-half*0.8, float32(center.Y)-half*0.35*direction),
		tip,
		float32(width),
		col,
	)
	strokeLine(gtx,
		tip,
		f32.Pt(float32(center.X)+half*0.8, float32(center.Y)-half*0.35*direction),
		float32(width),
		col,
	)
}

func (w *settingsWindow) setToggle(enabled bool) {
	if w.onToggle != nil {
		if err := w.onToggle(enabled); err != nil {
			w.errorText = err.Error()
			return
		}
	}
	w.enabled = enabled
	w.errorText = ""
}

func (w *settingsWindow) setControl(enabled bool) {
	if w.onControl != nil {
		if err := w.onControl(enabled); err != nil {
			w.errorText = err.Error()
			return
		}
	}
	w.control = enabled
	w.errorText = ""
}

func (w *settingsWindow) setProvider(provider watchparty.Provider) {
	if provider == w.provider {
		return
	}
	if w.onProvider != nil {
		if err := w.onProvider(provider); err != nil {
			w.errorText = err.Error()
			return
		}
	}
	w.provider = provider
	w.errorText = ""
}

func (w *settingsWindow) drawToggle(gtx layout.Context, rect image.Rectangle, on bool, hover float32) {
	if rect.Empty() {
		return
	}
	radius := rect.Dy() / 2
	border := settingsToggleOffEdge
	if on {
		border = settingsToggleOnColor
	}
	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	innerRadius := radius - gtx.Dp(unit.Dp(1))
	if innerRadius < 0 {
		innerRadius = 0
	}
	if on {
		c1 := mixColor(settingsToggleOnColor, color.NRGBA{R: 0xae, G: 0x8b, B: 0xff, A: 0xff}, hover)
		c2 := mixColor(mainAccentColor2, color.NRGBA{R: 0x82, G: 0x3e, B: 0xee, A: 0xff}, hover)
		fillRoundedGradient(gtx, inner, inner, innerRadius, c1, c2)
	} else {
		fill := mixColor(settingsToggleOffFill, color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}, hover)
		fillRounded(gtx, fill, inner, innerRadius)
	}

	knobRadius := rect.Dy()/2 - gtx.Dp(unit.Dp(3))
	if knobRadius < 1 {
		knobRadius = 1
	}
	knobX := rect.Min.X + rect.Dy()/2
	knobColor := settingsToggleKnobOff
	if on {
		knobX = rect.Max.X - rect.Dy()/2
		knobColor = settingsToggleKnob
	}
	fillCircle(gtx, image.Pt(knobX, rect.Min.Y+rect.Dy()/2), knobRadius, knobColor)
}

func (w *settingsWindow) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(settingsButtonHeight)
	top := (size.Y - height) / 2
	backWidth := gtx.Dp(unit.Dp(120))
	backRect := image.Rect(padX, top, padX+backWidth, top+height)

	buttonFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, capInk := w.typo.measure(gtx, "Н", buttonFace, settingsButtonSize)
	capHeight := capInk.Dy()
	centerY := float32(height) / 2

	if w.layoutSettingsButton(gtx, backRect, &w.backClick, &w.backHover, func(gtx layout.Context) {
		local := image.Rect(0, 0, backRect.Dx(), backRect.Dy())
		_, ink := w.typo.measure(gtx, "Вернуться", buttonFace, settingsButtonSize)
		iconSize := gtx.Dp(unit.Dp(11))
		gap := gtx.Dp(unit.Dp(8))
		groupWidth := float32(iconSize + gap + ink.Dx())
		start := (float32(local.Dx()) - groupWidth) / 2
		drawChevronLeft(
			gtx,
			image.Pt(
				int(math.Round(float64(start+float32(iconSize)/2))),
				int(math.Round(float64(centerY))),
			),
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
			capHeight,
		)
	}) {
		if w.onBack != nil {
			w.onBack()
		}
	}
}

func (w *settingsWindow) layoutSettingsButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, hover *hoverAnim, draw func(layout.Context)) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	progress := hover.update(gtx, click.Hovered())
	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(8))
		fill := mixColor(titleButtonFill, titleButtonHover, progress)
		border := mixColor(titleButtonBorder, titleButtonHoverBorder, progress)
		fillRounded(gtx, border, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
		strokeRoundedRect(
			gtx,
			local.Inset(gtx.Dp(unit.Dp(1))),
			float32(radius),
			float32(gtx.Dp(unit.Dp(1))),
			border,
		)
		if draw != nil {
			draw(gtx)
		}
	})
	return clicked
}

func (w *settingsWindow) layoutClickableDraw(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context)) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if draw != nil {
			draw(gtx)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
}

// animationsRun prevents hover animations from asking for frames while the
// window is hidden in the notification area.
func (w *settingsWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}
