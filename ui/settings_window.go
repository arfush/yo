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
	settingsTitleGap     = unit.Dp(16)
	settingsGroupGap     = unit.Dp(20)
	settingsRowGap       = unit.Dp(10)
	settingsPillWidth    = unit.Dp(94)
	settingsPillHeight   = unit.Dp(28)
	settingsPillGap      = unit.Dp(8)
)

var (
	settingsToggleOnColor = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	settingsToggleOffFill = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	settingsToggleOffEdge = color.NRGBA{R: 0x3a, G: 0x34, B: 0x4a, A: 0xff}
	settingsToggleKnob    = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	settingsToggleKnobOff = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	settingsErrorText     = color.NRGBA{R: 0xf0, G: 0x8a, B: 0x8a, A: 0xff}
	settingsNoteColor     = color.NRGBA{R: 0x8a, G: 0x84, B: 0x9c, A: 0xff}
)

// settingsValues is the state the settings screen opens with. It is separate
// from the window so the screen itself has no storage dependency.
type settingsValues struct {
	Notifications  bool
	SharedControl  bool
	Provider       watchparty.Provider
	ProviderChoice bool
}

type settingsWindow struct {
	typo *typography

	enabled  bool
	control  bool
	provider watchparty.Provider
	// providerChoice hides the relay row when the build ships one relay: a
	// choice between one option is not a setting.
	providerChoice bool

	toggleClick  widget.Clickable
	toggleHover  hoverAnim
	controlClick widget.Clickable
	controlHover hoverAnim
	providerPill [2]widget.Clickable

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
}

func newSettingsWindow(typo *typography, values settingsValues) *settingsWindow {
	return &settingsWindow{
		typo:           typo,
		enabled:        values.Notifications,
		control:        values.SharedControl,
		provider:       values.Provider,
		providerChoice: values.ProviderChoice,
	}
}

func (w *settingsWindow) Close() {}

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
	if w.providerChoice {
		y += gtx.Dp(settingsRowGap)
		y = w.layoutProviderPanel(gtx, image.Rect(left, y, right, y+gtx.Dp(settingsPanelHeight)))
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
		"Совместное управление",
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
		blockHeight += gtx.Dp(unit.Dp(2)) + noteInk.Dy()
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
		noteTop := top + labelInk.Dy() + gtx.Dp(unit.Dp(2))
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
	labelTop := rect.Min.Y + (rect.Dy()-labelInk.Dy())/2
	w.typo.draw(gtx, "Провайдер", labelFace, settingsLabelSize, mainTextColor, image.Pt(left, labelTop))

	providers := watchparty.Providers()
	pillHeight := gtx.Dp(settingsPillHeight)
	pillWidth := gtx.Dp(settingsPillWidth)
	gap := gtx.Dp(settingsPillGap)
	right := rect.Max.X - gtx.Dp(settingsPanelPadX)
	top := rect.Min.Y + (rect.Dy()-pillHeight)/2
	for index := len(providers) - 1; index >= 0; index-- {
		provider := providers[index]
		pill := image.Rect(right-pillWidth, top, right, top+pillHeight)
		right = pill.Min.X - gap
		if pill.Empty() {
			continue
		}
		if w.layoutProviderPill(gtx, pill, provider, &w.providerPill[index]) {
			w.setProvider(provider)
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	return rect.Max.Y
}

func (w *settingsWindow) layoutProviderPill(gtx layout.Context, rect image.Rectangle, provider watchparty.Provider, click *widget.Clickable) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	hovered := click.Hovered()
	selected := provider == w.provider

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())

	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		radius := gtx.Dp(unit.Dp(7))
		switch {
		case selected:
			fillRoundedGradient(gtx, local, local, radius, mainAccentColor1, mainAccentColor2)
		case hovered:
			fillRounded(gtx, tokenDomainFillHover, local, radius)
		default:
			fillRounded(gtx, tokenDomainFill, local, radius)
		}
		if !selected {
			strokeRoundedRect(gtx, local, float32(radius), float32(gtx.Dp(unit.Dp(1))), tokenDomainBorder)
		}
		textColor := settingsToggleKnobOff
		if selected {
			textColor = titleButtonText
		}
		w.typo.drawCentered(gtx, provider.Label(), font.Font{Typeface: fontFamily, Weight: font.Medium}, settingsButtonSize, textColor, centerOf(local))
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
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
