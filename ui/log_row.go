package ui

import (
	"image"
	"image/color"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

type logLineState struct {
	click    widget.Clickable
	expanded bool
}

// logWindow shows the recent application log. The copy action deliberately
// copies only the last error and its context, not the ordinary informational
// lines that may contain the user's viewing history.

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
