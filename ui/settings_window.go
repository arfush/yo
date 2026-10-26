package ui

import (
	"errors"
	"image"
	"image/color"
	"math"

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
	"github.com/arfush/yonime/party"
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
	Name          string
	Provider      party.Provider
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
	provider   party.Provider

	cloudflareEditor   widget.Editor
	supabaseURLEditor  widget.Editor
	supabaseKeyEditor  widget.Editor
	nameEditor         widget.Editor
	cloudflareFocused  bool
	supabaseURLFocused bool
	supabaseKeyFocused bool
	nameFocused        bool
	relayDirty         bool
	nameDirty          bool

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

	back footerNamedButton

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
	onProvider   func(party.Provider) error
	onRelay      func(party.Config) error
	onName       func(string) error
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
		back:          newBackFooterButton(),
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
	w.nameEditor.SingleLine = true
	w.nameEditor.Submit = true
	w.nameEditor.InputHint = key.HintText
	w.nameEditor.SetText(values.Name)
	return w
}

func (w *settingsWindow) Close() {
	w.saveRelays()
	w.saveName()
}

// dismissPopup closes the provider menu when the window loses focus.
func (w *settingsWindow) dismissPopup() {
	w.providerOpen = false
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
	var nameSubmitted, nameFocusLost bool
	y, nameSubmitted, nameFocusLost = w.layoutNamePanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
	if nameSubmitted || nameFocusLost {
		w.saveName()
	}
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
	if w.provider == party.Supabase {
		relays = 2
	}
	height := gtx.Dp(settingsPadTop)
	height += title("Уведомления") + panel
	height += gtx.Dp(settingsGroupGap)
	height += title("Обновление") + 2*panel + gtx.Dp(settingsRowGap)
	height += gtx.Dp(settingsGroupGap)
	height += title("Совместный просмотр") + 3*panel
	height += (relays + 2) * gtx.Dp(settingsRowGap)
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
	panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: settingsPanelRadius}.layout(gtx, rect)
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
func (w *settingsWindow) layoutNamePanel(gtx layout.Context, rect image.Rectangle) (int, bool, bool) {
	if rect.Empty() {
		return rect.Max.Y, false, false
	}
	w.drawPanel(gtx, rect)
	fieldRect := relayFieldRect(gtx, rect)
	if fieldRect.Empty() {
		return rect.Max.Y, false, false
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, "Имя в комнате", face, settingsLabelSize)
	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	top := rect.Min.Y + (rect.Dy()-ink.Dy())/2
	w.typo.draw(gtx, "Имя в комнате", face, settingsLabelSize, mainTextColor, image.Pt(left, top))

	submitted := w.layoutSettingsEditor(gtx, fieldRect, &w.nameEditor, "например, quiet-tanuki", &w.nameDirty)
	focusLost := relayFocusLost(gtx, &w.nameEditor, &w.nameFocused)
	return rect.Max.Y, submitted, focusLost
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
func (w *settingsWindow) saveRelays() {
	if !w.relayDirty {
		return
	}
	cfg := party.Config{
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

// saveName validates and stores the room nickname. A rejected value keeps the
// dirty flag set, so the next focus change or Enter retries the same text
// instead of silently discarding what the user typed.
func (w *settingsWindow) saveName() {
	if !w.nameDirty {
		return
	}
	normalized, err := party.NormalizeName(w.nameEditor.Text())
	if err != nil {
		w.reportError(errors.New("никнейм — до 24 символов, без служебных знаков и не похож на ключ комнаты"))
		return
	}
	if w.onName != nil {
		if err := w.onName(normalized); err != nil {
			w.reportError(err)
			return
		}
	}
	w.nameDirty = false
	w.errorText = ""
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

func (w *settingsWindow) setProvider(provider party.Provider) {
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

	if w.back.layout(gtx, w.typo, backRect) {
		if w.onBack != nil {
			w.onBack()
		}
	}
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
