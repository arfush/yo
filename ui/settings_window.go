package ui

import (
	"fmt"
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
)

const (
	settingsPadX            = unit.Dp(28)
	settingsPadTop          = unit.Dp(30)
	settingsTitleSize       = unit.Sp(21)
	settingsLabelSize       = unit.Sp(14)
	settingsNoteSize        = unit.Sp(11.5)
	settingsButtonSize      = unit.Sp(12)
	settingsButtonHeight    = unit.Dp(34)
	settingsPanelRadius     = unit.Dp(10)
	settingsPanelHeight     = unit.Dp(84)
	settingsPanelPadX       = unit.Dp(18)
	settingsPanelPadTop     = unit.Dp(15)
	settingsToggleWidth     = unit.Dp(42)
	settingsToggleHeight    = unit.Dp(24)
	settingsTestPanelGap    = unit.Dp(18)
	settingsTestPanelHeight = unit.Dp(132)
)

var (
	settingsToggleOnColor  = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	settingsToggleOffFill  = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	settingsToggleOffEdge  = color.NRGBA{R: 0x3a, G: 0x34, B: 0x4a, A: 0xff}
	settingsToggleKnob     = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	settingsToggleKnobOff  = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	settingsErrorText      = color.NRGBA{R: 0xf0, G: 0x8a, B: 0x8a, A: 0xff}
	settingsSuccessText    = color.NRGBA{R: 0x8d, G: 0xd6, B: 0xa8, A: 0xff}
	settingsTestNoteColor  = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	settingsTestLabelColor = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
)

type settingsWindow struct {
	typo *typography

	enabled bool

	toggleClick widget.Clickable
	toggleHover hoverAnim
	backClick   widget.Clickable
	backHover   hoverAnim
	testClick   widget.Clickable
	testHover   hoverAnim

	errorText   string
	testMessage string
	testError   bool

	// hidden reports that the window is parked in the notification area, so
	// hover transitions stop requesting frames.
	hidden func() bool

	onBack   func()
	onToggle func(bool) error
	onTest   func() (int, error)
}

func newSettingsWindow(typo *typography, enabled bool) *settingsWindow {
	return &settingsWindow{typo: typo, enabled: enabled}
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
	padTop := gtx.Dp(settingsPadTop)

	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, "Уведомления", titleFace, settingsTitleSize)
	w.typo.draw(gtx, "Уведомления", titleFace, settingsTitleSize, mainTextColor, image.Pt(padX, padTop))

	panelTop := padTop + titleInk.Dy() + gtx.Dp(unit.Dp(22))
	panel := image.Rect(padX, panelTop, size.X-padX, panelTop+gtx.Dp(settingsPanelHeight))
	if !panel.Empty() {
		w.layoutNotificationPanel(gtx, panel)
	}

	testTop := panel.Max.Y + gtx.Dp(settingsTestPanelGap)
	testPanel := image.Rect(padX, testTop, size.X-padX, testTop+gtx.Dp(settingsTestPanelHeight))
	if !testPanel.Empty() && testPanel.Max.Y+gtx.Dp(settingsButtonHeight) < size.Y {
		w.layoutTestPanel(gtx, testPanel)
	}
}

func (w *settingsWindow) layoutNotificationPanel(gtx layout.Context, rect image.Rectangle) {
	radius := gtx.Dp(settingsPanelRadius)
	fillRounded(gtx, titlePanelBorder, rect, radius)
	fillRounded(gtx, titlePanelFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	clicked := w.toggleClick.Clicked(gtx)
	hovered := w.toggleClick.Hovered()
	hover := w.toggleHover.update(gtx, hovered && w.animationsRun())
	if clicked {
		w.setEnabled(!w.enabled)
		gtx.Execute(op.InvalidateCmd{})
	}

	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	toggleWidth := gtx.Dp(settingsToggleWidth)
	toggleHeight := gtx.Dp(settingsToggleHeight)
	toggle := image.Rect(
		rect.Max.X-gtx.Dp(settingsPanelPadX)-toggleWidth,
		rect.Min.Y+(rect.Dy()-toggleHeight)/2,
		rect.Max.X-gtx.Dp(settingsPanelPadX),
		rect.Min.Y+(rect.Dy()+toggleHeight)/2,
	)
	textRight := toggle.Min.X - gtx.Dp(unit.Dp(18))

	labelText := "Сообщать о новых сериях/релизе"
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := w.typo.measure(gtx, labelText, labelFace, settingsLabelSize)
	labelTop := rect.Min.Y + (rect.Dy()-labelInk.Dy())/2
	w.typo.draw(gtx, labelText, labelFace, settingsLabelSize, mainTextColor, image.Pt(left, labelTop))

	if w.errorText != "" {
		drawTruncatedText(
			gtx,
			w.typo,
			w.errorText,
			font.Font{Typeface: fontFamily, Weight: font.Normal},
			settingsNoteSize,
			settingsErrorText,
			image.Pt(left, labelTop+labelInk.Dy()+gtx.Dp(unit.Dp(2))),
			textRight-left,
		)
	}

	w.layoutClickableDraw(gtx, rect, &w.toggleClick, func(gtx layout.Context) {
		w.drawToggle(gtx, toggle.Sub(rect.Min), hover)
	})
}

func (w *settingsWindow) layoutTestPanel(gtx layout.Context, rect image.Rectangle) {
	radius := gtx.Dp(settingsPanelRadius)
	fillRounded(gtx, titlePanelBorder, rect, radius)
	fillRounded(gtx, titlePanelFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	left := rect.Min.X + gtx.Dp(settingsPanelPadX)
	top := rect.Min.Y + gtx.Dp(settingsPanelPadTop)
	sectionFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	w.typo.draw(
		gtx,
		"ТЕСТ УВЕДОМЛЕНИЙ",
		sectionFace,
		settingsNoteSize,
		settingsTestLabelColor,
		image.Pt(left, top),
	)
	note := "Отправит тестовые уведомления для тайтлов из истории"
	drawTruncatedText(
		gtx,
		w.typo,
		note,
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		settingsNoteSize,
		settingsTestNoteColor,
		image.Pt(left, top+gtx.Dp(unit.Dp(21))),
		rect.Max.X-gtx.Dp(settingsPanelPadX)-left,
	)
	drawTruncatedText(
		gtx,
		w.typo,
		"в статусе онгоинг или анонс.",
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		settingsNoteSize,
		settingsTestNoteColor,
		image.Pt(left, top+gtx.Dp(unit.Dp(37))),
		rect.Max.X-gtx.Dp(settingsPanelPadX)-left,
	)

	buttonWidth := gtx.Dp(unit.Dp(245))
	buttonHeight := gtx.Dp(settingsButtonHeight)
	button := image.Rect(left, rect.Max.Y-gtx.Dp(settingsPanelPadTop)-buttonHeight, left+buttonWidth, rect.Max.Y-gtx.Dp(settingsPanelPadTop))
	if button.Max.X > rect.Max.X-gtx.Dp(settingsPanelPadX) {
		button.Max.X = rect.Max.X - gtx.Dp(settingsPanelPadX)
	}
	clicked := w.layoutSettingsButton(gtx, button, &w.testClick, &w.testHover, func(gtx layout.Context) {
		w.typo.paintCentered(
			gtx,
			"Отправить тестовые уведомления",
			font.Font{Typeface: fontFamily, Weight: font.Medium},
			settingsButtonSize,
			titleButtonText,
			f32.Pt(float32(button.Dx())/2, float32(button.Dy())/2),
			gtx.Dp(unit.Dp(9)),
		)
	})
	if clicked {
		w.runTestNotification()
	}

	if w.testMessage != "" {
		col := settingsSuccessText
		if w.testError {
			col = settingsErrorText
		}
		drawTruncatedText(
			gtx,
			w.typo,
			w.testMessage,
			font.Font{Typeface: fontFamily, Weight: font.Normal},
			settingsNoteSize,
			col,
			image.Pt(button.Max.X+gtx.Dp(unit.Dp(14)), button.Min.Y+(button.Dy()-gtx.Dp(unit.Dp(15)))/2),
			rect.Max.X-gtx.Dp(settingsPanelPadX)-button.Max.X-gtx.Dp(unit.Dp(14)),
		)
	}
}

func (w *settingsWindow) runTestNotification() {
	w.testMessage = ""
	w.testError = false
	if w.onTest == nil {
		return
	}
	sent, err := w.onTest()
	if err != nil {
		w.testMessage = err.Error()
		w.testError = true
		return
	}
	if sent == 0 {
		w.testMessage = "Нет подходящих тайтлов"
		return
	}
	w.testMessage = fmt.Sprintf("Отправлено: %d", sent)
}

func (w *settingsWindow) setEnabled(enabled bool) {
	if w.onToggle != nil {
		if err := w.onToggle(enabled); err != nil {
			w.errorText = err.Error()
			return
		}
	}
	w.enabled = enabled
	w.errorText = ""
}

func (w *settingsWindow) drawToggle(gtx layout.Context, rect image.Rectangle, hover float32) {
	if rect.Empty() {
		return
	}
	radius := rect.Dy() / 2
	border := settingsToggleOffEdge
	if w.enabled {
		border = settingsToggleOnColor
	}
	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	innerRadius := radius - gtx.Dp(unit.Dp(1))
	if innerRadius < 0 {
		innerRadius = 0
	}
	if w.enabled {
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
	if w.enabled {
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
