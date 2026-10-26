package ui

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

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

type logLineState struct {
	click    widget.Clickable
	expanded bool
}

// logWindow shows the recent application log. The copy action deliberately
// copies only the last error and its context, not the ordinary informational
// lines that may contain the user's viewing history.
type logWindow struct {
	typo *typography

	list  layout.List
	lines map[string]*logLineState

	back     footerButton
	copy     footerButton
	copiedAt time.Time
	onBack   func()
	hidden   func() bool
}

func newLogWindow(typo *typography) *logWindow {
	w := &logWindow{
		typo:  typo,
		lines: make(map[string]*logLineState),
		back:  newBackFooterButton(),
		copy: footerButton{
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
	radius := gtx.Dp(logCardRadius)
	fillRounded(gtx, titlePanelBorder, card, radius)
	fillRounded(gtx, titlePanelFill, card.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	innerPad := gtx.Dp(logInnerPad)
	left := card.Min.X + innerPad
	right := card.Max.X - innerPad
	top := card.Min.Y + innerPad

	accentWidth := gtx.Dp(unit.Dp(3))
	accentHeight := gtx.Dp(unit.Dp(22))
	accent := image.Rect(left, top, left+accentWidth, top+accentHeight)
	fillRounded(gtx, mainAccentColor1, accent, accentWidth/2)

	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, "Журнал", titleFace, logTitleSize)
	titleX := accent.Max.X + gtx.Dp(unit.Dp(10))
	titleY := top + (accentHeight-titleInk.Dy())/2
	w.typo.draw(gtx, "Журнал", titleFace, logTitleSize, mainTextColor, image.Pt(titleX, titleY))

	subtitleFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	subtitle := "Последние события приложения. «Скопировать» копирует последнюю ошибку и её контекст."
	_, subtitleInk := w.typo.measure(gtx, subtitle, subtitleFace, logSubtitleSize)
	subtitleY := accent.Max.Y + gtx.Dp(unit.Dp(5))
	drawTruncatedText(
		gtx,
		w.typo,
		subtitle,
		subtitleFace,
		logSubtitleSize,
		mainMutedColor,
		image.Pt(titleX, subtitleY),
		right-titleX,
	)

	dividerY := subtitleY + subtitleInk.Dy() + gtx.Dp(unit.Dp(14))
	fill(gtx, titlePanelBorder, image.Rect(left, dividerY, right, dividerY+max(1, gtx.Dp(unit.Dp(1)))))

	lines := log.Snapshot()
	listTop := dividerY + gtx.Dp(unit.Dp(8))
	listRect := image.Rect(left, listTop, right, card.Max.Y-innerPad)
	if listRect.Empty() {
		return
	}
	if len(lines) == 0 {
		_, emptyInk := w.typo.measure(gtx, "Записей пока нет.", subtitleFace, logLineSize)
		at := image.Pt(
			listRect.Min.X+(listRect.Dx()-emptyInk.Dx())/2,
			listRect.Min.Y+(listRect.Dy()-emptyInk.Dy())/2,
		)
		w.typo.draw(gtx, "Записей пока нет.", subtitleFace, logLineSize, mainMutedColor, at)
		return
	}

	stack := op.Offset(listRect.Min).Push(gtx.Ops)
	clipStack := clip.Rect(image.Rectangle{Max: listRect.Size()}).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(listRect.Size())
	w.list.Gap = gtx.Dp(logRowGap)
	w.syncFollowMode(gtx, lines, listRect.Dx(), listRect.Dy(), subtitleFace)
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

func (w *logWindow) layoutLine(gtx layout.Context, line string, width int, state *logLineState) layout.Dimensions {
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	padX := gtx.Dp(logRowPadX)
	padY := gtx.Dp(logRowPadY)
	textWidth := max(width-2*padX, 1)
	collapsed := truncateText(gtx, w.typo, line, face, logLineSize, textWidth)
	truncated := collapsed != line

	if truncated || state.expanded {
		if state.click.Clicked(gtx) {
			state.expanded = !state.expanded
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	lines := []string{collapsed}
	if state.expanded {
		lines = wrapLogText(gtx, w.typo, line, face, logLineSize, textWidth)
	}

	lineHeight := gtx.Dp(logLineHeight)
	rowHeight := len(lines)*lineHeight + 2*padY
	rowRect := image.Rect(0, 0, width, rowHeight)
	rowRadius := gtx.Dp(unit.Dp(6))
	isError := isLogError(line)
	textColor := logLineColor(line)

	state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if truncated || state.expanded {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		switch {
		case isError:
			fillRounded(gtx, logErrorRowBorder, rowRect, rowRadius)
			fillRounded(gtx, logErrorRowFill, rowRect.Inset(gtx.Dp(unit.Dp(1))), rowRadius-gtx.Dp(unit.Dp(1)))
		case (truncated || state.expanded) && state.click.Hovered():
			fillRounded(gtx, logHoverRowFill, rowRect, rowRadius)
		}
		for index, text := range lines {
			w.typo.draw(
				gtx,
				text,
				face,
				logLineSize,
				textColor,
				image.Pt(padX, padY+index*lineHeight),
			)
		}
		return layout.Dimensions{Size: rowRect.Size()}
	})

	return layout.Dimensions{Size: rowRect.Size()}
}

func isLogError(line string) bool {
	return strings.Contains(line, " ERROR ")
}

func logLineColor(line string) color.NRGBA {
	switch {
	case isLogError(line):
		return mainDangerColor
	case strings.Contains(line, " WARN "):
		return logWarningText
	default:
		return mainControlColor
	}
}

// wrapLogText breaks a log entry at spaces, and at rune boundaries for a word
// that is wider than the row. It keeps every rune of the original line so an
// expanded row is never truncated.
func wrapLogText(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp, maxWidth int) []string {
	text = strings.TrimSpace(text)
	if maxWidth <= 0 || text == "" {
		return []string{text}
	}
	if _, ink := typo.measure(gtx, text, face, size); ink.Dx() <= maxWidth {
		return []string{text}
	}

	runes := []rune(text)
	lines := make([]string, 0, 2)
	for len(runes) > 0 {
		for len(runes) > 0 && unicode.IsSpace(runes[0]) {
			runes = runes[1:]
		}
		if len(runes) == 0 {
			break
		}
		if _, ink := typo.measure(gtx, string(runes), face, size); ink.Dx() <= maxWidth {
			lines = append(lines, string(runes))
			break
		}

		fit := runesThatFit(gtx, typo, runes, face, size, maxWidth)
		if fit < 1 {
			fit = 1
		}
		cut := fit
		for index := fit; index > 0; index-- {
			if unicode.IsSpace(runes[index-1]) {
				cut = index - 1
				break
			}
		}
		if cut <= 0 {
			cut = fit
		}
		lines = append(lines, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

func runesThatFit(gtx layout.Context, typo *typography, runes []rune, face font.Font, size unit.Sp, maxWidth int) int {
	low, high := 1, len(runes)
	best := 0
	for low <= high {
		middle := (low + high) / 2
		if _, ink := typo.measure(gtx, string(runes[:middle]), face, size); ink.Dx() <= maxWidth {
			best = middle
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	return best
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
