package ui

import (
	"image"
	"image/color"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

const logTimeLayout = "2006-01-02 15:04:05.000"

type logLevel uint8

const (
	logLevelInfo logLevel = iota
	logLevelWarning
	logLevelError
)

func (l logLevel) label() string {
	switch l {
	case logLevelWarning:
		return "Предупреждение"
	case logLevelError:
		return "Ошибка"
	default:
		return "Информация"
	}
}

func (l logLevel) color() color.NRGBA {
	switch l {
	case logLevelWarning:
		return logWarningText
	case logLevelError:
		return logErrorText
	default:
		return logInfoText
	}
}

type logEntry struct {
	raw     string
	at      time.Time
	level   logLevel
	source  string
	message string
	detail  string
}

func (e logEntry) timeLabel() string {
	if e.at.IsZero() {
		return "--:--:--"
	}
	return e.at.Format("15:04:05")
}

func (e logEntry) searchText() string {
	return e.message + " " + e.detail + " " + e.source + " " + e.level.label()
}

type logLineState struct {
	click    widget.Clickable
	expanded bool
}

// parseLogEntry splits the logger's fixed line format into the columns shown by
// the log table. The source is inferred from the message because the logger
// deliberately stays package-independent and has no source field.
func parseLogEntry(line string) logEntry {
	entry := logEntry{raw: line, level: logLevelInfo}
	rest := line
	hasTime := false
	if len(line) >= len(logTimeLayout) {
		if at, err := time.ParseInLocation(logTimeLayout, line[:len(logTimeLayout)], time.Local); err == nil {
			entry.at = at
			hasTime = true
			rest = strings.TrimSpace(line[len(logTimeLayout):])
		}
	}
	if hasTime {
		if levelToken, after, ok := strings.Cut(rest, " "); ok {
			entry.level = parseLogLevel(levelToken)
			rest = after
		} else {
			entry.level = parseLogLevel(rest)
			rest = ""
		}
	}
	entry.message, entry.detail = splitLogDetail(rest)
	entry.source = logSource(entry.message)
	return entry
}

func parseLogLevel(token string) logLevel {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "WARN", "WARNING":
		return logLevelWarning
	case "ERROR":
		return logLevelError
	default:
		// DEBUG is deliberately informational: the window has no separate
		// debug filter, and debug entries are still useful in the search.
		return logLevelInfo
	}
}

// splitLogDetail removes the structured key="value" tail from the visible
// message. Expanding the row shows that tail again for diagnostics.
func splitLogDetail(rest string) (message, detail string) {
	rest = strings.TrimSpace(rest)
	for index := 0; index < len(rest); index++ {
		if rest[index] != ' ' {
			continue
		}
		if logAttrTail(rest[index:]) {
			return strings.TrimSpace(rest[:index]), rest
		}
	}
	return rest, rest
}

func logAttrTail(s string) bool {
	for len(s) > 0 {
		if s[0] != ' ' {
			return false
		}
		s = s[1:]
		start := 0
		for start < len(s) && logAttrKeyByte(s[start]) {
			start++
		}
		if start == 0 || start >= len(s) || s[start] != '=' {
			return false
		}
		s = s[start+1:]
		if len(s) == 0 || s[0] != '"' {
			return false
		}
		s = s[1:]
		closed := false
		for len(s) > 0 {
			switch s[0] {
			case '\\':
				if len(s) < 2 {
					return false
				}
				s = s[2:]
			case '"':
				s = s[1:]
				closed = true
			default:
				s = s[1:]
			}
			if closed {
				break
			}
		}
		if !closed {
			return false
		}
	}
	return true
}

func logAttrKeyByte(b byte) bool {
	return b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' ||
		b == '_' || b == '-' || b == '.'
}

func logSource(message string) string {
	lower := strings.ToLower(message)
	switch {
	case containsAny(lower,
		"github", "release", "updater", "sha-256", "sha256", "новую версию") ||
		containsLogWord(lower, "update"):
		return "Обновления"
	case containsAny(lower,
		"party", "room", "relay", "anchor", "watch together", "совместн", "комнат"):
		return "Комната"
	case containsAny(lower,
		"playback", "player", "mpv", "kodik", "episode", "voice", "audio",
		"subtitle", "stream", "buffer", "seek", "playlist", "серия", "озвучка"):
		return "Плеер"
	default:
		return "Приложение"
	}
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

// containsLogWord matches a whole ASCII word so "update check" maps to updates
// while "presence updated" stays with the application.
func containsLogWord(s, word string) bool {
	for start := 0; start <= len(s)-len(word); {
		index := strings.Index(s[start:], word)
		if index < 0 {
			return false
		}
		index += start
		before := index == 0 || !logAttrKeyByte(s[index-1])
		afterIndex := index + len(word)
		after := afterIndex == len(s) || !logAttrKeyByte(s[afterIndex])
		if before && after {
			return true
		}
		start = index + 1
	}
	return false
}

// logRowHeight is the minimum row height. Expanded rows grow by whole text
// lines instead of clipping the diagnostic tail.
func (w *logWindow) logRowHeight(gtx layout.Context, entry logEntry, width int, expanded bool) int {
	minHeight := gtx.Dp(logRowHeightDp)
	if !expanded {
		return minHeight
	}
	cols := logColumnsForWidth(gtx, width)
	lines := wrapLogText(gtx, w.typo, entry.detail, logFace, logLineSize, cols.messageWidth)
	height := len(lines)*gtx.Dp(logLineHeight) + 2*gtx.Dp(logRowPadY)
	return max(minHeight, height)
}

func (w *logWindow) layoutLine(gtx layout.Context, entry logEntry, width int, state *logLineState) layout.Dimensions {
	cols := logColumnsForWidth(gtx, width)
	lineHeight := gtx.Dp(logLineHeight)
	padY := gtx.Dp(logRowPadY)
	minHeight := gtx.Dp(logRowHeightDp)

	collapsed := truncateText(gtx, w.typo, entry.message, logFace, logLineSize, cols.messageWidth)
	truncated := collapsed != entry.message
	expandable := truncated || entry.detail != entry.message

	if (expandable || state.expanded) && state.click.Clicked(gtx) {
		state.expanded = !state.expanded
		gtx.Execute(op.InvalidateCmd{})
	}

	lines := []string{entry.message}
	if state.expanded {
		lines = wrapLogText(gtx, w.typo, entry.detail, logFace, logLineSize, cols.messageWidth)
	}
	rowHeight := minHeight
	if state.expanded {
		rowHeight = max(minHeight, len(lines)*lineHeight+2*padY)
	}
	rowRect := image.Rect(0, 0, width, rowHeight)
	centerY := rowHeight / 2

	isError := entry.level == logLevelError
	state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if expandable || state.expanded {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		switch {
		case isError:
			fill(gtx, logErrorRowFill, rowRect)
			fill(gtx, logErrorRowBorder, image.Rect(0, 0, width, max(gtx.Dp(1), 1)))
			fill(gtx, logErrorRowBorder, image.Rect(0, rowHeight-max(gtx.Dp(1), 1), width, rowHeight))
		case (expandable || state.expanded) && state.click.Hovered():
			fill(gtx, logRowHover, rowRect)
		}
		if !isError && !((expandable || state.expanded) && state.click.Hovered()) {
			fill(gtx, logRowDivider, image.Rect(0, rowHeight-max(gtx.Dp(1), 1), width, rowHeight))
		}

		capHeight := logCapHeight(gtx, w.typo)
		muted := logInfoText
		messageColor := logMessageColor
		if isError {
			messageColor = logErrorText
		}

		drawLogText(gtx, w.typo, entry.timeLabel(), logFace, logMetaSize, muted, cols.timeX, centerY, cols.timeWidth, capHeight)
		levelColor := entry.level.color()
		iconSize := gtx.Dp(logLevelIconSize)
		drawLogLevelIcon(gtx, entry.level, image.Pt(cols.levelX+iconSize/2, centerY), iconSize, levelColor)
		drawLogText(gtx, w.typo, entry.level.label(), logFace, logLevelSize, levelColor, cols.levelX+iconSize+gtx.Dp(logLevelGap), centerY, cols.levelWidth-iconSize-gtx.Dp(logLevelGap), capHeight)
		drawLogText(gtx, w.typo, entry.source, logFace, logMetaSize, muted, cols.sourceX, centerY, cols.sourceWidth, capHeight)

		if state.expanded {
			top := max(padY, (rowHeight-len(lines)*lineHeight)/2)
			for index, text := range lines {
				lineCenter := top + index*lineHeight + lineHeight/2
				drawLogText(gtx, w.typo, text, logFace, logLineSize, messageColor, cols.messageX, lineCenter, cols.messageWidth, capHeight)
			}
		} else {
			drawLogText(gtx, w.typo, collapsed, logFace, logLineSize, messageColor, cols.messageX, centerY, cols.messageWidth, capHeight)
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

func isLogError(line string) bool {
	return parseLogEntry(line).level == logLevelError
}

func logLineColor(line string) color.NRGBA {
	return parseLogEntry(line).level.color()
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
