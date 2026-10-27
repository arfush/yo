package ui

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/arfush/yonime/log"
)

const (
	logPad          = unit.Dp(22)
	logInnerPad     = unit.Dp(22)
	logCardRadius   = unit.Dp(14)
	logTitleSize    = unit.Sp(17)
	logSubtitleSize = unit.Sp(11.5)
	logLineSize     = unit.Sp(10)
	logLineHeight   = unit.Dp(14)
	logRowPadX      = unit.Dp(10)
	logRowPadY      = unit.Dp(3)
	logRowGap       = unit.Dp(2)
)

var (
	logErrorRowFill   = color.NRGBA{R: 0x3a, G: 0x14, B: 0x1d, A: 0xff}
	logErrorRowBorder = color.NRGBA{R: 0x67, G: 0x23, B: 0x33, A: 0xff}
	logWarningText    = color.NRGBA{R: 0xe0, G: 0xb0, B: 0x72, A: 0xff}
	logHoverRowFill   = color.NRGBA{R: 0x17, G: 0x15, B: 0x21, A: 0xff}
)

type logWindow struct {
	typo *typography

	list  layout.List
	lines map[string]*logLineState

	back     footerNamedButton
	copy     footerNamedButton
	copiedAt time.Time
	onBack   func()
	hidden   func() bool
}

func newLogWindow(typo *typography) *logWindow {
	w := &logWindow{
		typo:  typo,
		lines: make(map[string]*logLineState),
		back:  newBackFooterButton(),
		copy: footerNamedButton{
			label: "Скопировать",
			style: footerCopyStyle,
			size:  titlePageButtonSize,
		},
	}
	w.list.Axis = layout.Vertical
	// The window opens on the newest entry. The list keeps following the end
	// until the user scrolls up; Enter explicitly jumps back to the end.
	w.list.ScrollToEnd = true
	return w
}

func (w *logWindow) Close() {}

func (w *logWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *logWindow) layout(gtx layout.Context) {
	w.handleKeys(gtx)

	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	pad := gtx.Dp(logPad)
	cardWidth := min(size.X-2*pad, gtx.Dp(unit.Dp(980)))
	cardHeight := min(size.Y-2*pad, gtx.Dp(unit.Dp(720)))
	if cardWidth <= 0 || cardHeight <= 0 {
		return
	}
	card := image.Rect(
		(size.X-cardWidth)/2,
		(size.Y-cardHeight)/2,
		(size.X+cardWidth)/2,
		(size.Y+cardHeight)/2,
	)
	panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: logCardRadius}.layout(gtx, card)

	innerPad := gtx.Dp(logInnerPad)
	left := card.Min.X + innerPad
	right := card.Max.X - innerPad
	top := card.Min.Y + innerPad
	_, dividerY := screenCardHeader{
		title:         "Журнал",
		subtitle:      "Последние события приложения. «Скопировать» копирует последнюю ошибку и её контекст.",
		titleSize:     logTitleSize,
		subtitleSize:  logSubtitleSize,
		titleColor:    mainTextColor,
		subtitleColor: mainMutedColor,
	}.layout(gtx, w.typo, left, right, top)

	lines := log.Snapshot()
	listTop := dividerY + gtx.Dp(unit.Dp(8))
	listRect := image.Rect(left, listTop, right, card.Max.Y-innerPad)
	if listRect.Empty() {
		return
	}
	if len(lines) == 0 {
		emptyState(gtx, w.typo, listRect, "Записей пока нет.", logLineSize, mainMutedColor)
		return
	}

	stack := op.Offset(listRect.Min).Push(gtx.Ops)
	clipStack := clip.Rect(image.Rectangle{Max: listRect.Size()}).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(listRect.Size())
	w.list.Gap = gtx.Dp(logRowGap)
	w.syncFollowMode(gtx, lines, listRect.Dx(), listRect.Dy(), font.Font{Typeface: fontFamily, Weight: font.Normal})
	w.list.Layout(lgtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		line := lines[index]
		return w.layoutLine(gtx, line, listRect.Dx(), w.lineState(line))
	})
	clipStack.Pop()
	stack.Pop()
}

func (w *logWindow) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		w.jumpToEnd()
		gtx.Execute(op.InvalidateCmd{})
	}
}

// jumpToEnd re-enables the list's end-following mode after the user has
// scrolled into older entries.
func (w *logWindow) jumpToEnd() {
	w.list.ScrollToEnd = true
	w.list.Position.BeforeEnd = false
}

// syncFollowMode keeps short logs top-aligned and long logs pinned to the
// newest entry. Once the user scrolls up, BeforeEnd makes the list stop
// following new lines until Enter jumps back.
func (w *logWindow) syncFollowMode(gtx layout.Context, lines []string, width, height int, face font.Font) {
	if len(lines) == 0 || width <= 0 || height <= 0 {
		return
	}
	lineHeight := gtx.Dp(logLineHeight)
	rowHeight := lineHeight + 2*gtx.Dp(logRowPadY)
	total := len(lines)*rowHeight + (len(lines)-1)*gtx.Dp(logRowGap)
	textWidth := max(width-2*gtx.Dp(logRowPadX), 1)
	for _, line := range lines {
		state := w.lines[line]
		if state == nil || !state.expanded {
			continue
		}
		wrapped := wrapLogText(gtx, w.typo, line, face, logLineSize, textWidth)
		total += (len(wrapped) - 1) * lineHeight
	}
	if total <= height {
		w.list.ScrollToEnd = false
		w.list.Position.First = 0
		w.list.Position.Offset = 0
		w.list.Position.BeforeEnd = false
		return
	}
	if w.list.Position.BeforeEnd {
		w.list.ScrollToEnd = false
	}
}

func (w *logWindow) lineState(line string) *logLineState {
	state := w.lines[line]
	if state == nil {
		state = &logLineState{}
		w.lines[line] = state
	}
	return state
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

	if w.back.layout(gtx, w.typo, backRect) {
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
	w.copy.label = label
	if w.copy.layout(gtx, w.typo, copyRect) {
		text := log.LastErrorContext()
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
