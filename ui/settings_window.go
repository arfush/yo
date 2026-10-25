package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/metadata"
	"github.com/arfush/yonime/internal/watchparty"
)

const (
	settingsPadX         = unit.Dp(28)
	settingsPadTop       = unit.Dp(20)
	settingsTitleSize    = unit.Sp(21)
	settingsLabelSize    = unit.Sp(14)
	settingsNoteSize     = unit.Sp(11.5)
	settingsButtonSize   = unit.Sp(12)
	settingsButtonHeight = unit.Dp(34)
	settingsPanelRadius  = unit.Dp(10)
	settingsPanelHeight  = unit.Dp(72)
	settingsPanelPadX    = unit.Dp(18)
	settingsToggleWidth  = unit.Dp(42)
	settingsToggleHeight = unit.Dp(24)
	settingsNoteGap      = unit.Dp(6)
	settingsTitleGap     = unit.Dp(12)
	settingsGroupGap     = unit.Dp(28)
	settingsRowGap       = unit.Dp(6)

	settingsProviderWidth      = unit.Dp(124)
	settingsProviderHeight     = unit.Dp(30)
	settingsProviderItemHeight = unit.Dp(32)
	settingsProviderItemGap    = unit.Dp(2)
	settingsProviderMenuPad    = unit.Dp(4)
	settingsProviderMenuGap    = unit.Dp(6)

	settingsFieldHeight = unit.Dp(30)
	settingsFieldWidth  = unit.Dp(360)
	settingsFieldPadX   = unit.Dp(10)
	settingsFieldSize   = unit.Sp(12)
	settingsLabelGap    = unit.Dp(18)
	settingsPadBottom   = unit.Dp(16)
)

var (
	settingsToggleOnColor = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	settingsToggleOffFill = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	settingsToggleOffEdge = color.NRGBA{R: 0x3a, G: 0x34, B: 0x4a, A: 0xff}
	settingsToggleKnob    = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	settingsToggleKnobOff = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	settingsNoteColor     = color.NRGBA{R: 0x8a, G: 0x84, B: 0x9c, A: 0xff}
	settingsMenuFill      = color.NRGBA{R: 0x1d, G: 0x1a, B: 0x27, A: 0xff}
	settingsMenuSelected  = color.NRGBA{R: 0x2b, G: 0x22, B: 0x45, A: 0xff}
)

// settingsValues is the state the settings screen opens with. It is separate
// from the window so the screen itself has no storage dependency.
type settingsValues struct {
	Notifications bool
	AutoUpdate    bool
	Beta          bool
	SharedControl bool
	Discord       bool
	AutoSkip      map[metadata.Segment]bool
	Provider      watchparty.Provider
	CloudflareURL string
	SupabaseURL   string
	SupabaseKey   string
}

type settingsWindow struct {
	typo *typography

	enabled    bool
	autoUpdate bool
	beta       bool
	control    bool
	discord    bool
	provider   watchparty.Provider

	cloudflareEditor   widget.Editor
	supabaseURLEditor  widget.Editor
	supabaseKeyEditor  widget.Editor
	cloudflareFocused  bool
	supabaseURLFocused bool
	supabaseKeyFocused bool
	relayDirty         bool

	toggleClick     widget.Clickable
	toggleHover     hoverAnim
	autoUpdateClick widget.Clickable
	autoUpdateHover hoverAnim
	betaClick       widget.Clickable
	betaHover       hoverAnim
	controlClick    widget.Clickable
	controlHover    hoverAnim
	discordClick    widget.Clickable
	discordHover    hoverAnim

	autoSkip      map[metadata.Segment]bool
	autoSkipClick map[metadata.Segment]*widget.Clickable
	autoSkipHover map[metadata.Segment]*hoverAnim

	// providerClick is the select itself, providerItems are its options and
	// providerScrim catches clicks outside the open menu.
	providerClick  widget.Clickable
	providerItems  [2]widget.Clickable
	providerScrim  widget.Clickable
	providerOpen   bool
	providerAnchor image.Rectangle

	backClick widget.Clickable
	backHover hoverAnim

	// onError sends technical failures to the shell header. errorText is kept
	// for tests and for the retry logic, but it is no longer drawn inline.
	onError func(error)

	// scroll is the wheel offset of the settings content, and scrollTag carries
	// the wheel events for the whole screen.
	scroll    int
	scrollTag struct{}

	errorText string

	// hidden reports that the window is parked in the notification area, so
	// hover transitions stop requesting frames.
	hidden func() bool

	onBack       func()
	onToggle     func(bool) error
	onAutoUpdate func(bool) error
	onBeta       func(bool) error
	onControl    func(bool) error
	onDiscord    func(bool) error
	onAutoSkip   func(metadata.Segment, bool) error
	onProvider   func(watchparty.Provider) error
	onRelay      func(watchparty.Config) error
}

func newSettingsWindow(typo *typography, values settingsValues) *settingsWindow {
	w := &settingsWindow{
		typo:          typo,
		enabled:       values.Notifications,
		autoUpdate:    values.AutoUpdate,
		beta:          values.Beta,
		control:       values.SharedControl,
		discord:       values.Discord,
		provider:      values.Provider,
		autoSkip:      make(map[metadata.Segment]bool),
		autoSkipClick: make(map[metadata.Segment]*widget.Clickable),
		autoSkipHover: make(map[metadata.Segment]*hoverAnim),
	}
	for _, segment := range metadata.Segments() {
		w.autoSkip[segment] = values.AutoSkip[segment]
		w.autoSkipClick[segment] = &widget.Clickable{}
		w.autoSkipHover[segment] = &hoverAnim{}
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

func (w *settingsWindow) reportError(err error) {
	if err == nil {
		return
	}
	w.errorText = err.Error()
	if w.onError != nil {
		w.onError(err)
	}
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

	viewport := image.Rectangle{Max: size}
	clipStack := clip.Rect(viewport).Push(gtx.Ops)
	defer clipStack.Pop()

	// The blocks are taller than the content area, so the whole screen scrolls
	// under the fixed header and footer.
	w.scroll = clampScroll(w.scroll+w.scrollDelta(gtx), w.contentHeight(gtx), size.Y)

	content := op.Offset(image.Pt(0, -w.scroll)).Push(gtx.Ops)

	y := gtx.Dp(settingsPadTop)
	y = w.layoutSectionTitle(gtx, left, y, "Уведомления")
	y = w.layoutNotificationPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsGroupGap)

	y = w.layoutSectionTitle(gtx, left, y, "Обновление")
	y = w.layoutAutoUpdatePanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsRowGap)
	y = w.layoutBetaPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsGroupGap)

	y = w.layoutSectionTitle(gtx, left, y, "Совместный просмотр")
	y = w.layoutControlPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	y += gtx.Dp(settingsRowGap)
	y = w.layoutProviderPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	var relaySubmitted, relayFocusLost bool
	y, relaySubmitted, relayFocusLost = w.layoutRelayPanels(gtx, left, right, y)
	if relaySubmitted || relayFocusLost {
		w.saveRelays()
	}
	y += gtx.Dp(settingsGroupGap)

	y = w.layoutSectionTitle(gtx, left, y, "Автопропуск")
	segments := metadata.Segments()
	for index, segment := range segments {
		y = w.layoutAutoSkipPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)), segment)
		if index < len(segments)-1 {
			y += gtx.Dp(settingsRowGap)
		}
	}
	y += gtx.Dp(settingsGroupGap)

	y = w.layoutSectionTitle(gtx, left, y, "Интеграции")
	y = w.layoutDiscordPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))

	content.Pop()

	// The menu floats over the panels above, so it is painted last. Its anchor
	// is stored in content coordinates and has to follow the scroll offset.
	if w.providerOpen {
		w.layoutProviderMenu(gtx, size, w.providerAnchor.Sub(image.Pt(0, w.scroll)))
	}
}

// scrollDelta feeds wheel input into the settings offset. The wheel works
// anywhere over the screen and closes the provider menu, whose position would
// otherwise drift away from its trigger.
func (w *settingsWindow) scrollDelta(gtx layout.Context) int {
	event.Op(gtx.Ops, &w.scrollTag)
	delta := 0
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  &w.scrollTag,
			Kinds:   pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: math.MinInt, Max: math.MaxInt},
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Kind != pointer.Scroll {
			continue
		}
		delta += int(e.Scroll.Y)
	}
	if delta != 0 {
		w.providerOpen = false
	}
	return delta
}

// clampScroll keeps the offset between the top of the content and its last
// scrollable pixel.
func clampScroll(scroll, content, viewport int) int {
	max := content - viewport
	if max < 0 {
		max = 0
	}
	if scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	return scroll
}

// contentHeight mirrors layout and reports the full height of the settings,
// including the part hidden below the viewport.
func (w *settingsWindow) contentHeight(gtx layout.Context) int {
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	title := func(text string) int {
		_, ink := w.typo.measure(gtx, text, titleFace, settingsTitleSize)
		return ink.Dy() + gtx.Dp(settingsTitleGap)
	}
	panel := gtx.Dp(settingsPanelHeight)
	relays := 1
	if w.provider == watchparty.Supabase {
		relays = 2
	}
	height := gtx.Dp(settingsPadTop)
	height += title("Уведомления") + panel
	height += gtx.Dp(settingsGroupGap)
	height += title("Обновление") + 2*panel + gtx.Dp(settingsRowGap)
	height += gtx.Dp(settingsGroupGap)
	height += title("Совместный просмотр") + 2*panel
	height += (relays + 1) * gtx.Dp(settingsRowGap)
	height += relays * panel
	height += gtx.Dp(settingsGroupGap)
	segments := len(metadata.Segments())
	height += title("Автопропуск") + segments*panel
	if segments > 1 {
		height += (segments - 1) * gtx.Dp(settingsRowGap)
	}
	height += gtx.Dp(settingsGroupGap)
	height += title("Интеграции") + panel
	height += gtx.Dp(settingsPadBottom)
	return height
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

func (w *settingsWindow) layoutAutoUpdatePanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	clicked := w.autoUpdateClick.Clicked(gtx)
	hovered := w.autoUpdateClick.Hovered()
	hover := w.autoUpdateHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setAutoUpdate(!w.autoUpdate)
		gtx.Execute(op.InvalidateCmd{})
	}

	w.layoutToggleRow(
		gtx,
		rect,
		"Автообновление",
		"Автоматически обновляет Yonime при запуске или пока оно неактивно - просмотру не повредит",
		w.autoUpdate,
		hover,
		&w.autoUpdateClick,
	)
	return rect.Max.Y
}

func (w *settingsWindow) layoutBetaPanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	clicked := w.betaClick.Clicked(gtx)
	hovered := w.betaClick.Hovered()
	hover := w.betaHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setBeta(!w.beta)
		gtx.Execute(op.InvalidateCmd{})
	}

	w.layoutToggleRow(
		gtx,
		rect,
		"Beta-тестирование",
		"Скачивать патч-версии. Могут быть ошибки",
		w.beta,
		hover,
		&w.betaClick,
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

func (w *settingsWindow) layoutDiscordPanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	clicked := w.discordClick.Clicked(gtx)
	hovered := w.discordClick.Hovered()
	hover := w.discordHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setDiscord(!w.discord)
		gtx.Execute(op.InvalidateCmd{})
	}

	w.layoutToggleRow(
		gtx,
		rect,
		"Показывать активность в Discord",
		"Название тайтла, серия и обложка видны друзьям в профиле",
		w.discord,
		hover,
		&w.discordClick,
	)
	return rect.Max.Y
}

func (w *settingsWindow) layoutAutoSkipPanel(gtx layout.Context, rect image.Rectangle, segment metadata.Segment) int {
	w.drawPanel(gtx, rect)
	click := w.autoSkipClick[segment]
	hover := w.autoSkipHover[segment]
	clicked := click.Clicked(gtx)
	progress := hover.update(gtx, click.Hovered() && w.animationsRun())
	if clicked {
		w.setAutoSkip(segment, !w.autoSkip[segment])
		gtx.Execute(op.InvalidateCmd{})
	}

	label, note := autoSkipText(segment)
	w.layoutToggleRow(gtx, rect, label, note, w.autoSkip[segment], progress, click)
	return rect.Max.Y
}

// autoSkipText is the settings wording for one introdb segment.
func autoSkipText(segment metadata.Segment) (string, string) {
	switch segment {
	case metadata.SegmentRecap:
		return "Рекап", "Повтор событий прошлых серий"
	case metadata.SegmentIntro:
		return "Опенинг", "Вступительная заставка"
	case metadata.SegmentOutro:
		return "Эндинг", "Завершающая заставка"
	case metadata.SegmentPreview:
		return "Превью", "Анонс следующей серии"
	case metadata.SegmentCredits:
		return "Титры", "Финальные титры серии"
	case metadata.SegmentPostCredits:
		return "Сцена после титров", "Фрагмент после финальных титров"
	}
	return string(segment), ""
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

func (w *settingsWindow) layoutProviderPanel(gtx layout.Context, rect image.Rectangle) int {
	w.drawPanel(gtx, rect)
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := w.typo.measure(gtx, "Провайдер", labelFace, settingsLabelSize)
	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	right := rect.Max.X - gtx.Dp(settingsPanelPadX)
	rowHeight := gtx.Dp(settingsProviderHeight)
	rowTop := rect.Min.Y + (rect.Dy()-rowHeight)/2
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
	return rect.Max.Y
}

// layoutRelayPanels draws one panel per editable endpoint, so each field reads
// as its own setting instead of a row inside the provider card. It returns the
// bottom of the last panel.
func (w *settingsWindow) layoutRelayPanels(gtx layout.Context, left, right, top int) (int, bool, bool) {
	height := gtx.Dp(settingsPanelHeight)
	gap := gtx.Dp(settingsRowGap)
	submitted := false
	focusLost := false

	if w.provider == watchparty.Supabase {
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

	submitted := w.layoutRelayField(gtx, fieldRect, editor, hint)
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
			w.reportError(err)
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
func (w *settingsWindow) layoutProviderMenu(gtx layout.Context, size image.Point, anchor image.Rectangle) {
	if anchor.Empty() {
		return
	}
	providers := watchparty.Providers()
	pad := gtx.Dp(settingsProviderMenuPad)
	itemHeight := gtx.Dp(settingsProviderItemHeight)
	itemGap := gtx.Dp(settingsProviderItemGap)
	menu := placeProviderMenu(
		anchor,
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
			w.reportError(err)
			return
		}
	}
	w.enabled = enabled
	w.errorText = ""
}

func (w *settingsWindow) setAutoUpdate(enabled bool) {
	if w.onAutoUpdate != nil {
		if err := w.onAutoUpdate(enabled); err != nil {
			w.reportError(err)
			return
		}
	}
	w.autoUpdate = enabled
	w.errorText = ""
}

func (w *settingsWindow) setBeta(enabled bool) {
	if w.onBeta != nil {
		if err := w.onBeta(enabled); err != nil {
			w.reportError(err)
			return
		}
	}
	w.beta = enabled
	w.errorText = ""
}

func (w *settingsWindow) setControl(enabled bool) {
	if w.onControl != nil {
		if err := w.onControl(enabled); err != nil {
			w.reportError(err)
			return
		}
	}
	w.control = enabled
	w.errorText = ""
}

func (w *settingsWindow) setDiscord(enabled bool) {
	if w.onDiscord != nil {
		if err := w.onDiscord(enabled); err != nil {
			w.reportError(err)
			return
		}
	}
	w.discord = enabled
	w.errorText = ""
}

func (w *settingsWindow) setAutoSkip(segment metadata.Segment, enabled bool) {
	if w.onAutoSkip != nil {
		if err := w.onAutoSkip(segment, enabled); err != nil {
			w.reportError(err)
			return
		}
	}
	w.autoSkip[segment] = enabled
	w.errorText = ""
}

func (w *settingsWindow) setProvider(provider watchparty.Provider) {
	if provider == w.provider {
		return
	}
	if w.onProvider != nil {
		if err := w.onProvider(provider); err != nil {
			w.reportError(err)
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
