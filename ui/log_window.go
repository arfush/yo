package ui

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/logging"
)

// logWindow shows the recent application log. The copy action deliberately
// copies only the last error and its context, not the ordinary informational
// lines that may contain the user's viewing history.
type logWindow struct {
	typo *typography

	list layout.List

	backClick widget.Clickable
	copyClick widget.Clickable
	backHover hoverAnim
	copyHover hoverAnim
	copiedAt  time.Time
	onBack    func()
	hidden    func() bool
}

func newLogWindow(typo *typography) *logWindow {
	w := &logWindow{typo: typo}
	w.list.Axis = layout.Vertical
	return w
}

func (w *logWindow) Close() {}

func (w *logWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *logWindow) layout(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	padTop := gtx.Dp(unit.Dp(24))
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, "Журнал", titleFace, mainHistoryTitleSize)
	w.typo.draw(gtx, "Журнал", titleFace, mainHistoryTitleSize, mainTextColor, image.Pt(padX, padTop))

	hintFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	hint := "Последние события приложения. «Скопировать» копирует последнюю ошибку и её контекст."
	hintTop := padTop + titleInk.Dy() + gtx.Dp(unit.Dp(6))
	drawTruncatedText(
		gtx,
		w.typo,
		hint,
		hintFace,
		mainClearSize,
		mainMutedColor,
		image.Pt(padX, hintTop),
		size.X-2*padX,
	)

	lines := logging.Snapshot()
	listTop := hintTop + gtx.Dp(unit.Dp(20))
	listRect := image.Rect(padX, listTop, size.X-padX, size.Y-gtx.Dp(unit.Dp(10)))
	if listRect.Empty() {
		return
	}
	if len(lines) == 0 {
		w.typo.draw(gtx, "Записей пока нет.", hintFace, mainClearSize, mainMutedColor, listRect.Min)
		return
	}

	stack := op.Offset(listRect.Min).Push(gtx.Ops)
	clipStack := clip.Rect(image.Rectangle{Max: listRect.Size()}).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(listRect.Size())
	lineHeight := gtx.Dp(unit.Dp(15))
	w.list.Layout(lgtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		line := lines[index]
		col := mainMutedColor
		if strings.Contains(line, " ERROR ") {
			col = mainDangerColor
		} else if strings.Contains(line, " WARN ") {
			col = color.NRGBA{R: 0xe0, G: 0xb0, B: 0x72, A: 0xff}
		}
		drawTruncatedText(
			gtx,
			w.typo,
			line,
			font.Font{Typeface: fontFamily, Weight: font.Normal},
			unit.Sp(9.5),
			col,
			image.Pt(0, 0),
			listRect.Dx(),
		)
		return layout.Dimensions{Size: image.Pt(listRect.Dx(), lineHeight)}
	})
	clipStack.Pop()
	stack.Pop()
}

func (w *logWindow) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(titlePageButtonHeight)
	top := (size.Y - height) / 2
	backWidth := gtx.Dp(unit.Dp(120))
	copyWidth := gtx.Dp(unit.Dp(132))
	backRect := image.Rect(padX, top, padX+backWidth, top+height)
	copyRect := image.Rect(size.X-padX-copyWidth, top, size.X-padX, top+height)

	if w.layoutButton(gtx, backRect, &w.backClick, &w.backHover, false, "Вернуться") {
		if w.onBack != nil {
			w.onBack()
		}
		return
	}

	label := "Скопировать"
	if !w.copiedAt.IsZero() && gtx.Now.Before(w.copiedAt) {
		label = "Скопировано"
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: w.copiedAt})
		}
	}
	if w.layoutButton(gtx, copyRect, &w.copyClick, &w.copyHover, true, label) {
		text := logging.LastErrorContext()
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(text)),
		})
		w.copiedAt = gtx.Now.Add(1500 * time.Millisecond)
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: w.copiedAt})
		}
	}
}

func (w *logWindow) layoutButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, hover *hoverAnim, primary bool, label string) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	progress := hover.update(gtx, click.Hovered())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		local := image.Rect(0, 0, rect.Dx(), rect.Dy())
		fill := mixColor(titleButtonFill, titleButtonHover, progress)
		border := mixColor(titleButtonBorder, titleButtonHoverBorder, progress)
		text := titleButtonText
		if primary {
			fill = mixColor(color.NRGBA{R: 0x4a, G: 0x35, B: 0x72, A: 0xff}, color.NRGBA{R: 0x5d, G: 0x45, B: 0x8a, A: 0xff}, progress)
			border = mixColor(color.NRGBA{R: 0x6d, G: 0x52, B: 0xa0, A: 0xff}, footerControlHoverBorder, progress)
			text = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		}
		radius := gtx.Dp(unit.Dp(10))
		fillRounded(gtx, fill, local, radius)
		strokeRoundedRect(gtx, local.Inset(gtx.Dp(unit.Dp(1))), float32(radius), float32(gtx.Dp(unit.Dp(1))), border)
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		_, ink := w.typo.measure(gtx, label, face, titlePageButtonSize)
		w.typo.draw(gtx, label, face, titlePageButtonSize, text, image.Pt((local.Dx()-ink.Dx())/2, (local.Dy()-ink.Dy())/2))
		return layout.Dimensions{Size: rect.Size()}
	})
	return clicked
}
