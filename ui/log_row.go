package ui

import (
	"image"
	"image/color"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	applog "github.com/arfush/yo/log"
)

const logTimeLayout = "15:04:05.000"

type logLineState struct {
	click    widget.Clickable
	expanded bool
}

func logTimeLabel(at time.Time) string {
	if at.IsZero() {
		return "--:--:--"
	}
	return at.Format(logTimeLayout)
}

func logSearchText(entry applog.Entry) string {
	return entry.Message + " " + entry.Detail + " " + entry.Source + " " + applog.LevelLabel(entry.Level)
}

func logLevelColor(level slog.Level) color.NRGBA {
	switch applog.LevelLabel(level) {
	case "trace":
		return logTraceText
	case "debug":
		return logDebugText
	case "notice":
		return logNoticeText
	case "warn":
		return logWarningText
	case "error":
		return logErrorText
	default:
		return logInfoText
	}
}

// logRowHeight is the minimum row height. Expanded rows grow by whole text
// lines instead of clipping the message or the structured tail.
func (w *logScreen) logRowHeight(gtx layout.Context, entry applog.Entry, width int, expanded bool) int {
	minHeight := gtx.Dp(logRowHeightDp)
	if !expanded {
		return minHeight
	}
	cols := logColumnsForWidth(gtx, width)
	messageLines := wrapLogText(gtx, w.typo, entry.Message, logFace, logLineSize, cols.messageWidth)
	detailLines := 0
	if entry.Detail != "" {
		detailLines = len(wrapLogText(gtx, w.typo, entry.Detail, logFace, logLineSize, cols.messageWidth))
	}
	height := (len(messageLines)+detailLines)*gtx.Dp(logLineHeight) + 2*gtx.Dp(logRowPadY)
	return max(minHeight, height)
}

func (w *logScreen) layoutLine(gtx layout.Context, entry applog.Entry, width, rowHeight, index int, state *logLineState) layout.Dimensions {
	cols := logColumnsForWidth(gtx, width)
	lineHeight := gtx.Dp(logLineHeight)
	padY := gtx.Dp(logRowPadY)

	collapsed := truncateText(gtx, w.typo, entry.Message, logFace, logLineSize, cols.messageWidth)
	truncated := collapsed != entry.Message
	expandable := truncated || entry.Detail != ""

	// Expansion clicks are consumed by layoutTableBody before heights are
	// measured, so a click never changes the row size mid-layout.
	if state.expanded {
		rowHeight = max(rowHeight, w.logRowHeight(gtx, entry, width, true))
	}

	rowRect := image.Rect(0, 0, width, rowHeight)
	centerY := rowHeight / 2
	hovered := state.click.Hovered()
	// The meta cells stay pinned to the first text line: an expanded row must
	// not move its time/level/source away from the neighbouring rows.
	metaY := centerY
	if state.expanded {
		metaY = padY + lineHeight/2
	}

	state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if expandable || state.expanded {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		rowFill := logRowFill
		if index%2 == 1 {
			rowFill = logRowAlt
		}
		switch entry.Level {
		case slog.LevelError:
			rowFill = logErrorRowFill
		case slog.LevelWarn:
			rowFill = logWarnRowFill
		}
		// Hover and expand must not erase the level tint: warn/error rows stay
		// yellow/red while they are highlighted, only slightly brighter. Every
		// row responds to hover the same way, expandable or not.
		switch {
		case state.expanded:
			rowFill = mixColor(rowFill, logRowExpanded, 0.6)
		case hovered:
			rowFill = mixColor(rowFill, logRowHover, 0.7)
		}
		fill(gtx, rowFill, rowRect)
		// Error rows keep a red accent on the left edge so the highlight reads
		// even for a row whose level column is scrolled away.
		if entry.Level >= slog.LevelError {
			barWidth := max(1, gtx.Dp(unit.Dp(3)))
			fill(gtx, logErrorRowBar, image.Rect(0, 0, barWidth, rowHeight))
		}
		// The divider is intentionally stronger than the fills: with softer
		// hover colours the rows still need a visible boundary.
		fill(gtx, logRowDivider, image.Rect(0, rowHeight-max(1, gtx.Dp(1)), width, rowHeight))

		capHeight := logCapHeight(gtx, w.typo)
		levelColor := logLevelColor(entry.Level)

		drawLogText(gtx, w.typo, logTimeLabel(entry.Time), logFace, logMetaSize, logMetaText, cols.timeX, metaY, cols.timeWidth, capHeight)
		drawLogText(gtx, w.typo, applog.LevelLabel(entry.Level), logFace, logLevelSize, levelColor, cols.levelX, metaY, cols.levelWidth, capHeight)
		drawLogText(gtx, w.typo, entry.Source, logFace, logMetaSize, logSourceText, cols.sourceX, metaY, cols.sourceWidth, capHeight)

		if state.expanded {
			messageLines := wrapLogText(gtx, w.typo, entry.Message, logFace, logLineSize, cols.messageWidth)
			var detailLines []string
			if entry.Detail != "" {
				detailLines = wrapLogText(gtx, w.typo, entry.Detail, logFace, logLineSize, cols.messageWidth)
			}
			contentHeight := (len(messageLines) + len(detailLines)) * lineHeight
			top := max(padY, (rowHeight-contentHeight)/2)
			y := top
			for _, text := range messageLines {
				drawLogText(gtx, w.typo, text, logFace, logLineSize, logMessageColor, cols.messageX, y+lineHeight/2, cols.messageWidth, capHeight)
				y += lineHeight
			}
			for _, text := range detailLines {
				drawLogText(gtx, w.typo, text, logFace, logLineSize, logDetailText, cols.messageX, y+lineHeight/2, cols.messageWidth, capHeight)
				y += lineHeight
			}
		} else {
			drawLogText(gtx, w.typo, collapsed, logFace, logLineSize, logMessageColor, cols.messageX, centerY, cols.messageWidth, capHeight)
		}
		return layout.Dimensions{Size: rowRect.Size()}
	})

	return layout.Dimensions{Size: rowRect.Size()}
}

func logCapHeight(gtx layout.Context, typo *typography) int {
	return logCapHeightFor(gtx, typo, logFace, logLineSize)
}

func logCapHeightFor(gtx layout.Context, typo *typography, face font.Font, size unit.Sp) int {
	glyphs, ink := typo.measure(gtx, "Н", face, size)
	if len(glyphs) == 0 {
		return ink.Dy()
	}
	height := int(glyphs[0].Y) - ink.Min.Y
	if height <= 0 {
		return ink.Dy()
	}
	return height
}

func drawLogText(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp, col color.NRGBA, x, centerY, maxWidth, capHeight int) {
	if maxWidth <= 0 || text == "" {
		return
	}
	text = truncateText(gtx, typo, text, face, size, maxWidth)
	_, ink := typo.measure(gtx, text, face, size)
	if ink.Dx() <= 0 {
		return
	}
	typo.paintCenteredOnBaseline(
		gtx,
		text,
		face,
		size,
		col,
		f32.Pt(float32(x)+float32(ink.Dx())/2, float32(centerY)),
		capHeight,
	)
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
