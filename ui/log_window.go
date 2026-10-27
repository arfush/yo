package ui

import (
	"fmt"
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
	"gioui.org/widget"

	"github.com/arfush/yonime/log"
	"github.com/arfush/yonime/ui/widgets"
)

const (
	logScreenPadX      = unit.Dp(28)
	logScreenPadTop    = unit.Dp(14)
	logScreenPadBottom = unit.Dp(14)
	logFieldRadius     = unit.Dp(8)
	logTableRadius     = unit.Dp(8)

	logToolbarHeight    = unit.Dp(34)
	logToolbarGap       = unit.Dp(11)
	logToolbarButtonGap = unit.Dp(8)
	logFilterWidth      = unit.Dp(155)
	logPauseWidth       = unit.Dp(80)
	logPauseWideWidth   = unit.Dp(112)
	logClearWidth       = unit.Dp(101)
	logToolbarPadX      = unit.Dp(12)
	logToolbarIconSize  = unit.Dp(14)
	logToolbarIconGap   = unit.Dp(8)
	logToolbarTextSize  = unit.Sp(11.5)

	logTableHeaderHeight = unit.Dp(30)
	logGroupHeight       = unit.Dp(25)
	logRowHeightDp       = unit.Dp(31)
	logLineSize          = unit.Sp(11.5)
	logLineHeight        = unit.Dp(16)
	logRowPadY           = unit.Dp(6)
	logMetaSize          = unit.Sp(10.5)
	logLevelSize         = unit.Sp(11)
	logGroupSize         = unit.Sp(11)
	logHeaderSize        = unit.Sp(10)
	logStatsSize         = unit.Sp(10.5)
	logStatsHeight       = unit.Dp(28)
	logLevelIconSize     = unit.Dp(14)
	logLevelGap          = unit.Dp(7)
	logCellPadX          = unit.Dp(14)
	logCellGap           = unit.Dp(7)
	logTimeColumn        = unit.Dp(96)
	logLevelColumn       = unit.Dp(126)
	logSourceColumn      = unit.Dp(112)
	logAutoCheckbox      = unit.Dp(14)
	logAutoGap           = unit.Dp(7)
	logMenuPad           = unit.Dp(4)
	logMenuItemHeight    = unit.Dp(30)
)

var (
	logFieldFill        = color.NRGBA{R: 0x12, G: 0x12, B: 0x1d, A: 0xff}
	logFieldBorder      = color.NRGBA{R: 0x31, G: 0x2d, B: 0x42, A: 0xff}
	logFieldHoverFill   = color.NRGBA{R: 0x1a, G: 0x18, B: 0x2b, A: 0xff}
	logFieldHoverBorder = color.NRGBA{R: 0x4b, G: 0x3e, B: 0x6c, A: 0xff}
	logPlaceholder      = color.NRGBA{R: 0x78, G: 0x73, B: 0x8d, A: 0xff}
	logAccent           = color.NRGBA{R: 0xba, G: 0x9a, B: 0xff, A: 0xff}
	logToolbarText      = color.NRGBA{R: 0xf1, G: 0xef, B: 0xf8, A: 0xff}

	logHeaderFill  = color.NRGBA{R: 0x1a, G: 0x18, B: 0x27, A: 0xff}
	logHeaderText  = color.NRGBA{R: 0x78, G: 0x73, B: 0x8d, A: 0xff}
	logGroupFill   = color.NRGBA{R: 0x12, G: 0x12, B: 0x1d, A: 0xff}
	logGroupText   = color.NRGBA{R: 0xaa, G: 0xa5, B: 0xbb, A: 0xff}
	logRowFill     = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x27, A: 0xff}
	logRowDivider  = color.NRGBA{R: 0x1f, G: 0x1e, B: 0x2b, A: 0xff}
	logRowHover    = color.NRGBA{R: 0x22, G: 0x20, B: 0x32, A: 0xff}
	logTableBorder = color.NRGBA{R: 0x25, G: 0x23, B: 0x31, A: 0xff}

	logMessageColor   = color.NRGBA{R: 0xf1, G: 0xef, B: 0xf8, A: 0xff}
	logErrorRowFill   = color.NRGBA{R: 0x21, G: 0x19, B: 0x23, A: 0xff}
	logErrorRowBorder = color.NRGBA{R: 0x2f, G: 0x27, B: 0x33, A: 0xff}
	logInfoText       = color.NRGBA{R: 0xaa, G: 0xa5, B: 0xbb, A: 0xff}
	logWarningText    = color.NRGBA{R: 0xdf, G: 0xba, B: 0x78, A: 0xff}
	logErrorText      = color.NRGBA{R: 0xec, G: 0x92, B: 0x9e, A: 0xff}
)

var logFace = font.Font{Typeface: fontFamily, Weight: font.Normal}

type logLevelFilter uint8

const (
	logFilterAll logLevelFilter = iota
	logFilterInfo
	logFilterWarning
	logFilterError
)

var logFilterOptions = [...]logLevelFilter{
	logFilterAll,
	logFilterInfo,
	logFilterWarning,
	logFilterError,
}

func (f logLevelFilter) label() string {
	switch f {
	case logFilterInfo:
		return "Информация"
	case logFilterWarning:
		return "Предупреждение"
	case logFilterError:
		return "Ошибка"
	default:
		return "Все уровни"
	}
}

func (f logLevelFilter) matches(level logLevel) bool {
	switch f {
	case logFilterInfo:
		return level == logLevelInfo
	case logFilterWarning:
		return level == logLevelWarning
	case logFilterError:
		return level == logLevelError
	default:
		return true
	}
}

type logWindow struct {
	typo *typography

	list  layout.List
	lines map[string]*logLineState

	raw     []string
	entries []logEntry
	paused  bool

	search      widget.Editor
	filter      logLevelFilter
	filterClick widget.Clickable
	filterHover hoverAnim
	pauseClick  widget.Clickable
	pauseHover  hoverAnim
	clearClick  widget.Clickable
	clearHover  hoverAnim
	autoScroll  bool
	autoClick   widget.Clickable
	autoHover   hoverAnim
	levelMenu   logLevelMenu

	back     footerNamedButton
	copy     footerNamedButton
	copiedAt time.Time
	onBack   func()
	hidden   func() bool
}

func newLogWindow(typo *typography) *logWindow {
	w := &logWindow{
		typo:       typo,
		lines:      make(map[string]*logLineState),
		back:       newBackFooterButton(),
		filter:     logFilterAll,
		autoScroll: true,
		copy: footerNamedButton{
			label: "Скопировать",
			style: footerCopyStyle,
			size:  titlePageButtonSize,
		},
	}
	w.list.Axis = layout.Vertical
	w.search.SingleLine = true
	w.search.InputHint = key.HintText
	// The window opens on the newest entry and the checkbox follows it until
	// the user scrolls up or turns autoscroll off.
	w.list.ScrollToEnd = true
	return w
}

func (w *logWindow) Close() {}

func (w *logWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *logWindow) layout(gtx layout.Context) {
	w.handleKeys(gtx)
	w.refreshEntries()

	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	padX := gtx.Dp(logScreenPadX)
	padTop := gtx.Dp(logScreenPadTop)
	padBottom := gtx.Dp(logScreenPadBottom)
	inner := image.Rect(padX, padTop, size.X-padX, size.Y-padBottom)
	if inner.Dx() < 200 || inner.Dy() < 120 {
		return
	}

	toolbarHeight := gtx.Dp(logToolbarHeight)
	toolbar := image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, inner.Min.Y+toolbarHeight)
	gap := gtx.Dp(logToolbarButtonGap)
	clearWidth := gtx.Dp(logClearWidth)
	pauseWidth := gtx.Dp(logPauseWidth)
	if w.paused {
		// "Продолжить" is wider than "Пауза". The shorter idle width keeps the
		// toolbar aligned with the reference while the paused state stays
		// readable.
		pauseWidth = gtx.Dp(logPauseWideWidth)
	}
	filterWidth := gtx.Dp(logFilterWidth)
	clearRect := image.Rect(inner.Max.X-clearWidth, toolbar.Min.Y, inner.Max.X, toolbar.Max.Y)
	pauseRect := image.Rect(clearRect.Min.X-gap-pauseWidth, toolbar.Min.Y, clearRect.Min.X-gap, toolbar.Max.Y)
	filterRect := image.Rect(pauseRect.Min.X-gap-filterWidth, toolbar.Min.Y, pauseRect.Min.X-gap, toolbar.Max.Y)
	searchRect := image.Rect(inner.Min.X, toolbar.Min.Y, filterRect.Min.X-gap, toolbar.Max.Y)
	if searchRect.Dx() <= 0 {
		return
	}

	w.layoutSearchField(gtx, searchRect)

	if w.layoutFilterButton(gtx, filterRect) {
		w.levelMenu.toggle(filterRect)
		gtx.Execute(op.InvalidateCmd{})
	}

	pauseLabel := "Пауза"
	pauseIcon := "pause"
	if w.paused {
		pauseLabel = "Продолжить"
		pauseIcon = "play"
	}
	if w.layoutToolbarButton(gtx, pauseRect, &w.pauseClick, &w.pauseHover, pauseIcon, pauseLabel, logAccent) {
		w.togglePause()
		gtx.Execute(op.InvalidateCmd{})
	}
	if w.layoutToolbarButton(gtx, clearRect, &w.clearClick, &w.clearHover, "trash", "Очистить", logAccent) {
		w.clearEntries()
		gtx.Execute(op.InvalidateCmd{})
	}

	entries := w.visibleEntries()

	contentGap := gtx.Dp(logToolbarGap)
	statsHeight := gtx.Dp(logStatsHeight)
	tableTop := toolbar.Max.Y + contentGap
	statsTop := inner.Max.Y - statsHeight
	tableBottom := statsTop - contentGap
	showStats := true
	minTableHeight := gtx.Dp(logTableHeaderHeight) + gtx.Dp(logGroupHeight) + gtx.Dp(logRowHeightDp)
	if tableBottom-tableTop < minTableHeight {
		// Very short windows keep the table and drop the summary rather than
		// letting the table collapse into a header-only strip.
		showStats = false
		statsTop = inner.Max.Y
		tableBottom = inner.Max.Y
	}
	tableRect := image.Rect(inner.Min.X, tableTop, inner.Max.X, tableBottom)
	if tableRect.Dy() <= 0 {
		return
	}
	w.layoutTable(gtx, tableRect, entries)

	if showStats && statsTop > tableRect.Max.Y {
		w.layoutStats(gtx, image.Rect(inner.Min.X, statsTop, inner.Max.X, inner.Max.Y), entries)
	}

	w.levelMenu.layout(
		gtx,
		w.typo,
		image.Rectangle{Max: size},
		w.filter,
		func(filter logLevelFilter) {
			w.filter = filter
			if w.autoScroll {
				w.jumpToEnd()
			}
			gtx.Execute(op.InvalidateCmd{})
		},
	)
}

// refreshEntries snapshots the ring only when it changed. The window is paused
// by simply skipping this step, which keeps the visible rows frozen while the
// logger continues collecting new lines.
func (w *logWindow) refreshEntries() {
	if w.paused {
		return
	}
	lines := log.Snapshot()
	if len(lines) == len(w.raw) {
		same := true
		for index := range lines {
			if lines[index] != w.raw[index] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	w.raw = lines
	w.entries = make([]logEntry, len(lines))
	for index, line := range lines {
		w.entries[index] = parseLogEntry(line)
	}
}

func (w *logWindow) visibleEntries() []logEntry {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	out := make([]logEntry, 0, len(w.entries))
	for _, entry := range w.entries {
		if !w.filter.matches(entry.level) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(entry.searchText()), query) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (w *logWindow) togglePause() {
	w.paused = !w.paused
	if !w.paused {
		w.refreshEntries()
	}
}

func (w *logWindow) clearEntries() {
	log.Clear()
	w.raw = nil
	w.entries = nil
	w.lines = make(map[string]*logLineState)
	w.list.Position = layout.Position{}
	w.list.ScrollToEnd = w.autoScroll
}

func (w *logWindow) setAutoScroll(on bool) {
	w.autoScroll = on
	if on {
		w.list.Position.BeforeEnd = false
		w.list.ScrollToEnd = true
	} else {
		w.list.ScrollToEnd = false
	}
}

func (w *logWindow) handleKeys(gtx layout.Context) {
	if gtx.Focused(&w.search) {
		return
	}
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

// jumpToEnd re-enables both the list's end-following mode and the autoscroll
// checkbox after the user has scrolled into older entries.
func (w *logWindow) jumpToEnd() {
	w.autoScroll = true
	w.list.ScrollToEnd = true
	w.list.Position.BeforeEnd = false
}

// syncFollowMode keeps short logs top-aligned and long logs pinned to the
// newest entry while autoscroll is enabled. Scrolling up turns the checkbox
// off so a later manual scroll position is not pulled away by new lines.
func (w *logWindow) syncFollowMode(gtx layout.Context, entries []logEntry, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	total := 0
	for _, entry := range entries {
		expanded := false
		if state := w.lines[entry.raw]; state != nil {
			expanded = state.expanded
		}
		total += w.logRowHeight(gtx, entry, width, expanded)
	}
	if total <= height {
		w.list.ScrollToEnd = false
		w.list.Position.First = 0
		w.list.Position.Offset = 0
		w.list.Position.BeforeEnd = false
		return
	}
	if !w.autoScroll {
		w.list.ScrollToEnd = false
		return
	}
	if w.list.Position.BeforeEnd {
		w.autoScroll = false
		w.list.ScrollToEnd = false
		return
	}
	w.list.ScrollToEnd = true
}

func (w *logWindow) lineState(line string) *logLineState {
	state := w.lines[line]
	if state == nil {
		state = &logLineState{}
		w.lines[line] = state
	}
	return state
}

func (w *logWindow) layoutSearchField(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fillRounded(gtx, logFieldFill, local, radius)
	border := logFieldBorder
	if gtx.Focused(&w.search) {
		border = mainInputFocusBorder
	}
	strokeRoundedRect(gtx, local, float32(radius), float32(max(1, gtx.Dp(1))), border)

	iconSize := gtx.Dp(logToolbarIconSize)
	iconCenter := image.Pt(
		gtx.Dp(logToolbarPadX)+iconSize/2,
		local.Dy()/2,
	)
	drawSearchIcon(gtx, iconCenter, iconSize, logHeaderText)

	lineHeight := gtx.Dp(unit.Dp(17))
	textRect := image.Rect(
		gtx.Dp(logToolbarPadX)+iconSize+gtx.Dp(logToolbarIconGap),
		(local.Dy()-lineHeight)/2,
		local.Max.X-gtx.Dp(logToolbarPadX),
		(local.Dy()+lineHeight)/2,
	)
	if textRect.Dx() > 0 {
		editorStack := op.Offset(textRect.Min).Push(gtx.Ops)
		editorGtx := gtx
		editorGtx.Constraints = layout.Exact(textRect.Size())
		w.search.Layout(
			editorGtx,
			w.typo.shaper,
			logFace,
			logToolbarTextSize,
			colorMaterial(gtx, mainTextColor),
			colorMaterial(gtx, mainSelectionColor),
		)
		editorStack.Pop()
	}
	if w.search.Len() == 0 && !gtx.Focused(&w.search) {
		drawLogText(
			gtx,
			w.typo,
			"Поиск по сообщениям...",
			logFace,
			logToolbarTextSize,
			logPlaceholder,
			textRect.Min.X,
			local.Dy()/2,
			textRect.Dx(),
			logCapHeightFor(gtx, w.typo, logFace, logToolbarTextSize),
		)
	}
}

func (w *logWindow) layoutFilterButton(gtx layout.Context, rect image.Rectangle) bool {
	clicked := w.filterClick.Clicked(gtx)
	hover := w.filterHover.update(gtx, w.filterClick.Hovered())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fill := mixColor(logFieldFill, logFieldHoverFill, hover)
	border := mixColor(logFieldBorder, logFieldHoverBorder, hover)
	if w.levelMenu.open {
		border = mainInputFocusBorder
	}
	fillRounded(gtx, fill, local, radius)
	strokeRoundedRect(gtx, local, float32(radius), float32(max(1, gtx.Dp(1))), border)

	capHeight := logCapHeightFor(gtx, w.typo, logFace, logToolbarTextSize)
	textX := gtx.Dp(logToolbarPadX)
	chevronSize := gtx.Dp(unit.Dp(8))
	textWidth := local.Dx() - textX - gtx.Dp(logToolbarPadX) - chevronSize - gtx.Dp(unit.Dp(6))
	drawLogText(gtx, w.typo, w.filter.label(), logFace, logToolbarTextSize, logInfoText, textX, local.Dy()/2, textWidth, capHeight)
	chevronCenter := image.Pt(local.Max.X-gtx.Dp(logToolbarPadX)-chevronSize/2, local.Dy()/2)
	if w.levelMenu.open {
		drawChevronUp(gtx, chevronCenter, chevronSize, max(1, gtx.Dp(1)), logInfoText)
	} else {
		drawChevronDown(gtx, chevronCenter, chevronSize, max(1, gtx.Dp(1)), logInfoText)
	}

	w.filterClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: local.Size()}
	})
	return clicked
}

func (w *logWindow) layoutToolbarButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, hover *hoverAnim, icon, label string, iconColor color.NRGBA) bool {
	clicked := click.Clicked(gtx)
	progress := hover.update(gtx, click.Hovered())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fill := mixColor(logFieldFill, logFieldHoverFill, progress)
	border := mixColor(logFieldBorder, logFieldHoverBorder, progress)
	fillRounded(gtx, fill, local, radius)
	strokeRoundedRect(gtx, local, float32(radius), float32(max(1, gtx.Dp(1))), border)

	iconSize := gtx.Dp(logToolbarIconSize)
	iconCenter := image.Pt(gtx.Dp(logToolbarPadX)+iconSize/2, local.Dy()/2)
	drawAppIcon(gtx, icon, iconCenter, iconSize, iconColor)
	textX := iconCenter.X + iconSize/2 + gtx.Dp(logToolbarIconGap)
	textWidth := local.Max.X - textX - gtx.Dp(logToolbarPadX)
	drawLogText(
		gtx,
		w.typo,
		label,
		logFace,
		logToolbarTextSize,
		logToolbarText,
		textX,
		local.Dy()/2,
		textWidth,
		logCapHeightFor(gtx, w.typo, logFace, logToolbarTextSize),
	)

	click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: local.Size()}
	})
	return clicked
}

type logColumns struct {
	timeX        int
	timeWidth    int
	levelX       int
	levelWidth   int
	sourceX      int
	sourceWidth  int
	messageX     int
	messageWidth int
}

func logColumnsForWidth(gtx layout.Context, width int) logColumns {
	padX := gtx.Dp(logCellPadX)
	gap := gtx.Dp(logCellGap)
	timeWidth := min(gtx.Dp(logTimeColumn), width/5)
	levelWidth := min(gtx.Dp(logLevelColumn), width/4)
	sourceWidth := min(gtx.Dp(logSourceColumn), width/5)
	timeX := padX
	levelX := timeX + timeWidth
	sourceX := levelX + levelWidth
	messageX := sourceX + sourceWidth
	return logColumns{
		timeX:        timeX,
		timeWidth:    max(timeWidth-gap, 1),
		levelX:       levelX,
		levelWidth:   max(levelWidth-gap, 1),
		sourceX:      sourceX,
		sourceWidth:  max(sourceWidth-gap, 1),
		messageX:     messageX,
		messageWidth: max(width-messageX-padX, 1),
	}
}

func (w *logWindow) layoutTable(gtx layout.Context, rect image.Rectangle, entries []logEntry) {
	if rect.Empty() || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	radius := gtx.Dp(logTableRadius)
	fillRounded(gtx, logRowFill, rect, radius)
	clipStack := clip.UniformRRect(rect, radius).Push(gtx.Ops)

	headerHeight := min(gtx.Dp(logTableHeaderHeight), rect.Dy())
	headerRect := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+headerHeight)
	fill(gtx, logHeaderFill, headerRect)
	w.layoutTableHeader(gtx, headerRect)

	if rect.Dy() > headerHeight {
		groupHeight := min(gtx.Dp(logGroupHeight), rect.Dy()-headerHeight)
		groupRect := image.Rect(rect.Min.X, headerRect.Max.Y, rect.Max.X, headerRect.Max.Y+groupHeight)
		fill(gtx, logGroupFill, groupRect)
		drawLogText(
			gtx,
			w.typo,
			w.sessionLabel(),
			logFace,
			logGroupSize,
			logGroupText,
			rect.Min.X+gtx.Dp(logCellPadX),
			groupRect.Min.Y+groupRect.Dy()/2,
			groupRect.Dx()-2*gtx.Dp(logCellPadX),
			logCapHeightFor(gtx, w.typo, logFace, logGroupSize),
		)
		fill(gtx, logRowDivider, image.Rect(groupRect.Min.X, groupRect.Max.Y-max(1, gtx.Dp(1)), groupRect.Max.X, groupRect.Max.Y))

		body := image.Rect(rect.Min.X, groupRect.Max.Y, rect.Max.X, rect.Max.Y)
		if body.Dy() > 0 {
			w.layoutTableBody(gtx, body, entries)
		}
	}

	clipStack.Pop()
	strokeRoundedRect(gtx, rect, float32(radius), float32(max(1, gtx.Dp(1))), logTableBorder)
}

func (w *logWindow) layoutTableHeader(gtx layout.Context, rect image.Rectangle) {
	if rect.Dy() <= 0 {
		return
	}
	cols := logColumnsForWidth(gtx, rect.Dx())
	centerY := rect.Min.Y + rect.Dy()/2
	capHeight := logCapHeightFor(gtx, w.typo, logFace, logHeaderSize)
	headerFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	drawHeader := func(text string, x, maxWidth int) {
		drawLogText(gtx, w.typo, text, headerFace, logHeaderSize, logHeaderText, x, centerY, maxWidth, capHeight)
	}
	baseX := rect.Min.X
	drawHeader("ВРЕМЯ", baseX+cols.timeX, cols.timeWidth)
	_, ink := w.typo.measure(gtx, "ВРЕМЯ", headerFace, logHeaderSize)
	arrowCenter := image.Pt(baseX+cols.timeX+ink.Dx()+gtx.Dp(5), centerY)
	if arrowCenter.X+gtx.Dp(4) < baseX+cols.levelX {
		drawChevronDown(gtx, arrowCenter, gtx.Dp(7), max(1, gtx.Dp(1)), logHeaderText)
	}
	drawHeader("УРОВЕНЬ", baseX+cols.levelX, cols.levelWidth)
	drawHeader("ИСТОЧНИК", baseX+cols.sourceX, cols.sourceWidth)
	drawHeader("СООБЩЕНИЕ", baseX+cols.messageX, cols.messageWidth)
	fill(gtx, logRowDivider, image.Rect(rect.Min.X, rect.Max.Y-max(1, gtx.Dp(1)), rect.Max.X, rect.Max.Y))
}

func (w *logWindow) layoutTableBody(gtx layout.Context, body image.Rectangle, entries []logEntry) {
	if len(entries) == 0 {
		text := "Записей пока нет."
		if len(w.entries) > 0 {
			text = "Ничего не найдено."
		}
		emptyState(gtx, w.typo, body, text, logLineSize, logInfoText)
		return
	}
	stack := op.Offset(body.Min).Push(gtx.Ops)
	clipStack := clip.Rect(image.Rectangle{Max: body.Size()}).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(body.Size())
	w.syncFollowMode(gtx, entries, body.Dx(), body.Dy())
	w.list.Gap = 0
	w.list.Layout(lgtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
		entry := entries[index]
		return w.layoutLine(gtx, entry, body.Dx(), w.lineState(entry.raw))
	})
	clipStack.Pop()
	stack.Pop()
}

func (w *logWindow) sessionLabel() string {
	at := time.Now()
	if len(w.entries) > 0 && !w.entries[0].at.IsZero() {
		at = w.entries[0].at
	}
	return formatLogDate(at) + " · Текущий сеанс"
}

func formatLogDate(at time.Time) string {
	if at.IsZero() {
		at = time.Now()
	}
	months := [...]string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	month := at.Month()
	if month < time.January || month > time.December {
		return at.Format("02.01.2006")
	}
	return fmt.Sprintf("%d %s %d", at.Day(), months[month-1], at.Year())
}

type logCounts struct {
	total   int
	info    int
	warning int
	error   int
}

func countLogEntries(entries []logEntry) logCounts {
	counts := logCounts{total: len(entries)}
	for _, entry := range entries {
		switch entry.level {
		case logLevelWarning:
			counts.warning++
		case logLevelError:
			counts.error++
		default:
			counts.info++
		}
	}
	return counts
}

func logStatsLabels(counts logCounts) [4]string {
	return [4]string{
		fmt.Sprintf("%d %s", counts.total, pluralRU(counts.total, "запись", "записи", "записей")),
		fmt.Sprintf("%d %s", counts.info, pluralRU(counts.info, "информационная", "информационные", "информационных")),
		fmt.Sprintf("%d %s", counts.warning, pluralRU(counts.warning, "предупреждение", "предупреждения", "предупреждений")),
		fmt.Sprintf("%d %s", counts.error, pluralRU(counts.error, "ошибка", "ошибки", "ошибок")),
	}
}

func pluralRU(n int, one, few, many string) string {
	mod100 := n % 100
	mod10 := n % 10
	switch {
	case mod100 >= 11 && mod100 <= 14:
		return many
	case mod10 == 1:
		return one
	case mod10 >= 2 && mod10 <= 4:
		return few
	default:
		return many
	}
}

func (w *logWindow) layoutStats(gtx layout.Context, rect image.Rectangle, entries []logEntry) {
	if rect.Empty() {
		return
	}
	counts := countLogEntries(entries)
	labels := logStatsLabels(counts)
	colors := [4]color.NRGBA{logInfoText, logHeaderText, logWarningText, logErrorText}
	centerY := rect.Min.Y + rect.Dy()/2
	capHeight := logCapHeightFor(gtx, w.typo, logFace, logStatsSize)

	labelWidth := max(gtx.Dp(logAutoCheckbox)+gtx.Dp(logAutoGap)+textWidth(gtx, w.typo, "Авто-прокрутка", logFace, logStatsSize), 1)
	autoRect := image.Rect(rect.Max.X-labelWidth, rect.Min.Y, rect.Max.X, rect.Max.Y)
	gap := gtx.Dp(unit.Dp(12))
	x := rect.Min.X
	for index, label := range labels {
		width := textWidth(gtx, w.typo, label, logFace, logStatsSize)
		if x+width > autoRect.Min.X-gap {
			break
		}
		drawLogText(gtx, w.typo, label, logFace, logStatsSize, colors[index], x, centerY, width, capHeight)
		x += width + gap
	}

	clicked := w.autoClick.Clicked(gtx)
	hover := w.autoHover.update(gtx, w.autoClick.Hovered())
	if clicked {
		w.setAutoScroll(!w.autoScroll)
		gtx.Execute(op.InvalidateCmd{})
	}
	stack := op.Offset(autoRect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: autoRect.Size()}
	checkboxSize := gtx.Dp(logAutoCheckbox)
	box := image.Rect(0, (local.Dy()-checkboxSize)/2, checkboxSize, (local.Dy()+checkboxSize)/2)
	boxRadius := gtx.Dp(unit.Dp(4))
	if w.autoScroll {
		fillRounded(gtx, mainAccentColor2, box, boxRadius)
		fillRounded(gtx, mainAccentColor1, box.Inset(max(1, gtx.Dp(1))), max(0, boxRadius-gtx.Dp(1)))
		drawCheckIcon(gtx, centerOf(box), gtx.Dp(unit.Dp(9)), mainTextColor)
	} else {
		fillRounded(gtx, mixColor(logFieldFill, logFieldHoverFill, hover), box, boxRadius)
		strokeRoundedRect(gtx, box, float32(boxRadius), float32(max(1, gtx.Dp(1))), mixColor(logFieldBorder, mainInputFocusBorder, hover))
	}
	labelX := checkboxSize + gtx.Dp(logAutoGap)
	drawLogText(
		gtx,
		w.typo,
		"Авто-прокрутка",
		logFace,
		logStatsSize,
		logInfoText,
		labelX,
		local.Dy()/2,
		local.Max.X-labelX,
		capHeight,
	)
	w.autoClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: local.Size()}
	})
}

func textWidth(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp) int {
	_, ink := typo.measure(gtx, text, face, size)
	return ink.Dx()
}

type logLevelMenu struct {
	open   bool
	anchor image.Rectangle
	clicks [4]widget.Clickable
	hovers [4]hoverAnim
	scrim  widget.Clickable
}

func (m *logLevelMenu) toggle(anchor image.Rectangle) {
	if m.open {
		m.open = false
		return
	}
	m.open = true
	m.anchor = anchor
}

func (m *logLevelMenu) layout(gtx layout.Context, typo *typography, bounds image.Rectangle, current logLevelFilter, onSelect func(logLevelFilter)) {
	if !m.open {
		return
	}
	if widgets.LayoutOverlayScrim(gtx, bounds, &m.scrim) {
		m.open = false
		return
	}
	itemHeight := gtx.Dp(logMenuItemHeight)
	pad := gtx.Dp(logMenuPad)
	height := 2*pad + len(logFilterOptions)*itemHeight
	menuRect := widgets.PlacePopupList(m.anchor, height, gtx.Dp(unit.Dp(6)), bounds)
	if menuRect.Empty() {
		return
	}
	drawPopupMenu(gtx, menuRect)
	capHeight := logCapHeightFor(gtx, typo, logFace, unit.Sp(11))
	for index, option := range logFilterOptions {
		itemRect := image.Rect(
			menuRect.Min.X+pad,
			menuRect.Min.Y+pad+index*itemHeight,
			menuRect.Max.X-pad,
			menuRect.Min.Y+pad+(index+1)*itemHeight,
		)
		clicked := m.clicks[index].Clicked(gtx)
		hover := m.hovers[index].update(gtx, m.clicks[index].Hovered())
		if option == current {
			fillRounded(gtx, settingsMenuSelected, itemRect, gtx.Dp(unit.Dp(5)))
		} else if hover > 0 {
			fillRounded(gtx, mixColor(settingsMenuFill, settingsMenuSelected, hover), itemRect, gtx.Dp(unit.Dp(5)))
		}
		textColor := mainTextColor
		if option == current {
			textColor = logAccent
		}
		drawLogText(
			gtx,
			typo,
			option.label(),
			logFace,
			unit.Sp(11),
			textColor,
			itemRect.Min.X+gtx.Dp(unit.Dp(10)),
			itemRect.Min.Y+itemRect.Dy()/2,
			itemRect.Dx()-gtx.Dp(unit.Dp(30)),
			capHeight,
		)
		if option == current {
			drawCheckIcon(
				gtx,
				image.Pt(itemRect.Max.X-gtx.Dp(unit.Dp(14)), itemRect.Min.Y+itemRect.Dy()/2),
				gtx.Dp(unit.Dp(9)),
				logAccent,
			)
		}
		stack := op.Offset(itemRect.Min).Push(gtx.Ops)
		m.clicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: itemRect.Size()}
		})
		stack.Pop()
		if clicked {
			onSelect(option)
			m.open = false
			return
		}
	}
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
