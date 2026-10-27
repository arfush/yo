package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// activeFieldErrorDuration is the shared red-feedback window for inputs that
// submit an action. It is deliberately longer than a tooltip: the user must
// have time to notice the border, the shaking submit icon and, where present,
// the mascot.
const activeFieldErrorDuration = 5 * time.Second

type fieldBoxStyle struct {
	fill        color.NRGBA
	border      color.NRGBA
	focusBorder color.NRGBA
	errorBorder color.NRGBA
	radius      unit.Dp
}

// drawFieldBox paints the outer border and inner fill and returns the content
// rectangle. The editor and the leading/trailing widgets stay with the screen;
// the box only owns the shared focus and error outline.
func drawFieldBox(gtx layout.Context, rect image.Rectangle, focused, invalid bool, style fieldBoxStyle) image.Rectangle {
	if rect.Empty() {
		return image.Rectangle{}
	}
	border := style.border
	if focused {
		border = style.focusBorder
	}
	if invalid {
		border = style.errorBorder
	}
	radius := gtx.Dp(style.radius)
	if radius <= 0 {
		radius = gtx.Dp(mainInputRadius)
	}
	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	if inner.Empty() {
		return image.Rectangle{}
	}
	fillRounded(gtx, style.fill, inner, max(0, radius-gtx.Dp(unit.Dp(1))))
	return inner
}

type actionSubmitStyle struct {
	accent1   color.NRGBA
	accent2   color.NRGBA
	hover1    color.NRGBA
	hover2    color.NRGBA
	disabled1 color.NRGBA
	disabled2 color.NRGBA
	error1    color.NRGBA
	error2    color.NRGBA
	icon      color.NRGBA
}

func defaultActionSubmitStyle() actionSubmitStyle {
	return actionSubmitStyle{
		accent1: mainAccentColor1,
		accent2: mainAccentColor2,
		// Hover is derived from the live section accents. Hard-coded violet
		// made the URL "next" button stay purple in the manga and ranobe
		// sections after the rest of the UI had already changed.
		hover1:    lighten(mainAccentColor1, 0x08),
		hover2:    lighten(mainAccentColor2, 0x08),
		disabled1: mainDisabledColor1,
		disabled2: mainDisabledColor2,
		error1:    mainDangerColor,
		error2:    mainDangerColor2,
		icon:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	}
}

func drawActionSubmit(gtx layout.Context, rect image.Rectangle, hover float32, disabled, invalid bool, shake float32) {
	if rect.Empty() {
		return
	}
	style := defaultActionSubmitStyle()
	c1, c2 := style.accent1, style.accent2
	switch {
	case disabled:
		c1, c2 = style.disabled1, style.disabled2
	case invalid:
		c1, c2 = style.error1, style.error2
	case hover > 0:
		c1 = mixColor(c1, style.hover1, hover)
		c2 = mixColor(c2, style.hover2, hover)
	}
	fillRoundedGradient(gtx, rect, rect, rect.Dy()/2, c1, c2)
	if disabled {
		return
	}
	center := centerOf(rect)
	center.X += errorShakePixels(gtx, shake, unit.Dp(0.7))
	drawChevronRight(gtx, center, rect.Dx()/3, gtx.Dp(unit.Dp(2)), style.icon)
}

// fieldVisual is handed to a leading slot so the mascot can react to the same
// state as the field without the field knowing what a mascot is.
type fieldVisual struct {
	now     time.Time
	focused bool
	invalid bool
	loading bool
}

// actionField is the shared editable field with a trailing submit button. It
// deliberately has no leading artwork: the main URL/slug field composes it
// with the mascot slot, while token and party fields stay mascot-free.
type actionField struct {
	typo        *typography
	editor      *widget.Editor
	submit      *widget.Clickable
	submitHover *hoverAnim

	box      fieldBoxStyle
	textSize unit.Sp
	paddingX unit.Dp
	gap      unit.Dp

	leading      func(layout.Context, image.Rectangle, fieldVisual)
	leadingSize  unit.Dp
	leadingInset unit.Dp
	trailing     func(layout.Context, int) (int, func(image.Rectangle))
	placeholder  func(layout.Context, image.Rectangle)

	loading       bool
	errorAt       time.Time
	animationsRun func() bool
	onSubmit      func(layout.Context)
}

func (f *actionField) invalid(now time.Time) bool {
	return errorFeedbackActive(now, f.errorAt)
}

func (f *actionField) layout(gtx layout.Context, rect image.Rectangle, focused bool) {
	if rect.Empty() || f.editor == nil {
		return
	}
	invalid := f.invalid(gtx.Now)
	inner := drawFieldBox(gtx, rect, focused, invalid, f.box)
	if inner.Empty() {
		return
	}

	nextSize := gtx.Dp(mainNextSize)
	nextInset := gtx.Dp(mainNextInset)
	nextRect := image.Rect(
		inner.Max.X-nextInset-nextSize,
		inner.Min.Y+(inner.Dy()-nextSize)/2,
		inner.Max.X-nextInset,
		inner.Min.Y+(inner.Dy()-nextSize)/2+nextSize,
	)

	trailingWidth := 0
	var drawTrailing func(image.Rectangle)
	if f.trailing != nil {
		trailingWidth, drawTrailing = f.trailing(gtx, inner.Dy())
	}
	textLeft := inner.Min.X + gtx.Dp(f.paddingX)
	if f.leading != nil {
		leadingSize := gtx.Dp(f.leadingSize)
		if leadingSize <= 0 {
			leadingSize = gtx.Dp(mainMascotSize)
		}
		inset := gtx.Dp(f.leadingInset)
		leadingRect := image.Rect(
			inner.Min.X+inset,
			inner.Min.Y+(inner.Dy()-leadingSize)/2,
			inner.Min.X+inset+leadingSize,
			inner.Min.Y+(inner.Dy()-leadingSize)/2+leadingSize,
		)
		textLeft = leadingRect.Max.X + gtx.Dp(f.gap)
		defer func() {
			f.leading(gtx, leadingRect, fieldVisual{now: gtx.Now, focused: focused, invalid: invalid, loading: f.loading})
		}()
	}

	textRight := nextRect.Min.X - gtx.Dp(f.gap)
	if trailingWidth > 0 {
		textRight = nextRect.Min.X - gtx.Dp(mainHintGap) - trailingWidth
	}
	lineHeight := gtx.Dp(unit.Dp(20))
	editorRect := image.Rect(
		textLeft,
		inner.Min.Y+(inner.Dy()-lineHeight)/2,
		textRight,
		inner.Min.Y+(inner.Dy()-lineHeight)/2+lineHeight,
	)
	if editorRect.Dx() > 0 {
		if f.editor.Len() == 0 && f.placeholder != nil {
			f.placeholder(gtx, editorRect)
		}
		editorGtx := gtx
		editorGtx.Constraints = layout.Exact(editorRect.Size())
		stack := op.Offset(editorRect.Min).Push(gtx.Ops)
		f.editor.Layout(
			editorGtx,
			f.typo.shaper,
			font.Font{Typeface: fontFamily, Weight: font.Normal},
			f.textSize,
			colorMaterial(gtx, mainTextColor),
			colorMaterial(gtx, mainSelectionColor),
		)
		stack.Pop()
	}

	if trailingWidth > 0 && drawTrailing != nil {
		trailingRect := image.Rect(
			nextRect.Min.X-gtx.Dp(mainHintGap)-trailingWidth,
			inner.Min.Y+(inner.Dy()-gtx.Dp(mainHintHeight))/2,
			nextRect.Min.X-gtx.Dp(mainHintGap),
			inner.Min.Y+(inner.Dy()-gtx.Dp(mainHintHeight))/2+gtx.Dp(mainHintHeight),
		)
		drawTrailing(trailingRect)
	}

	if nextRect.Empty() {
		return
	}
	if f.loading {
		drawActionSubmit(gtx, nextRect, 0, true, false, 0)
		drawSpinner(gtx, centerOf(nextRect), gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(1.7)), 0, mainTextColor)
		if f.animationsRun == nil || f.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(mainSpinnerFrameTime)})
		}
		return
	}

	clicked := f.submit.Clicked(gtx)
	hover := float32(0)
	if f.submitHover != nil {
		hover = f.submitHover.update(gtx, f.submit.Hovered())
	}
	shake := errorShake(gtx.Now, f.errorAt)
	stack := op.Offset(nextRect.Min).Push(gtx.Ops)
	drawActionSubmit(gtx, image.Rectangle{Max: nextRect.Size()}, hover, false, invalid, shake)
	stack.Pop()
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(nextRect.Size())
	stack = op.Offset(nextRect.Min).Push(gtx.Ops)
	f.submit.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: nextRect.Size()}
	})
	stack.Pop()
	if clicked && f.onSubmit != nil {
		f.onSubmit(gtx)
	}
	if invalid && (f.animationsRun == nil || f.animationsRun()) {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 60)})
	}
}

// copyField is a read-only value with a trailing copy button. It shares the
// field box but not the submit-on-Enter behaviour of actionField.
type copyField struct {
	typo   *typography
	value  string
	click  *widget.Clickable
	box    fieldBoxStyle
	size   unit.Sp
	viewer func(layout.Context, image.Rectangle)
}

func (f copyField) layout(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() || f.click == nil {
		return false
	}
	inner := drawFieldBox(gtx, rect, false, false, f.box)
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
	textRect := image.Rect(
		inner.Min.X+gtx.Dp(unit.Dp(wpFieldPadX)),
		inner.Min.Y+(inner.Dy()-gtx.Dp(unit.Dp(22)))/2,
		round.Min.X-gtx.Dp(unit.Dp(10)),
		inner.Min.Y+(inner.Dy()-gtx.Dp(unit.Dp(22)))/2+gtx.Dp(unit.Dp(22)),
	)
	if textRect.Dx() > 0 {
		if f.viewer != nil {
			f.viewer(gtx, textRect)
		} else {
			drawReadOnlyValue(gtx, f.typo, textRect, f.value)
		}
	}
	clicked := f.click.Clicked(gtx)
	hovered := f.click.Hovered()
	center := centerOf(round)
	style := iconButtonStyle{
		Shape:       iconButtonCircle,
		Fill:        mainAccentColor1,
		FillHover:   lighten(mainAccentColor1, 0x08),
		Border:      mainAccentColor2,
		BorderHover: lighten(mainAccentColor2, 0x08),
		Gradient:    true,
	}
	progress := float32(0)
	if hovered {
		progress = 1
	}
	buttonStack := op.Offset(round.Min).Push(gtx.Ops)
	drawIconButtonSurface(gtx, image.Rectangle{Max: round.Size()}, style, progress)
	iconGeometry := makeGeometry(gtx, false, false, false, false)
	iconGeometry.scale *= 1.5
	drawCopyIcon(gtx, image.Pt(center.X-round.Min.X, center.Y-round.Min.Y), iconGeometry, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	buttonStack.Pop()
	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(round.Size())
	stack := op.Offset(round.Min).Push(gtx.Ops)
	f.click.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: round.Size()}
	})
	stack.Pop()
	return clicked
}

func drawReadOnlyValue(gtx layout.Context, typo *typography, rect image.Rectangle, value string) {
	if value == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := typo.measure(gtx, value, face, wpKeySize)
	if ink.Dx() <= rect.Dx() {
		typo.drawCentered(gtx, value, face, wpKeySize, mainTextColor, centerOf(rect))
		return
	}
	drawTruncatedText(gtx, typo, value, face, wpKeySize, mainTextColor, rect.Min, rect.Dx())
}
