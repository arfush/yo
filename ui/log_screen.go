package ui

import (
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	applog "github.com/arfush/yo/log"
	"github.com/arfush/yo/ui/widgets"
)

const (
	logScreenPadX      = unit.Dp(28)
	logScreenPadTop    = unit.Dp(14)
	logScreenPadBottom = unit.Dp(10)
	logFieldRadius     = unit.Dp(8)
	logTableRadius     = unit.Dp(8)

	logToolbarHeight    = unit.Dp(32)
	logToolbarGap       = unit.Dp(11)
	logToolbarButtonGap = unit.Dp(8)
	logFilterWidth      = unit.Dp(124)
	logAutoWidth        = unit.Dp(32)
	logToolbarIconWidth = unit.Dp(32)
	logToolbarPadX      = unit.Dp(10)
	logToolbarIconSize  = unit.Dp(18)
	logToolbarIconGap   = unit.Dp(8)
	logToolbarTextSize  = unit.Sp(12)

	logTableHeaderHeight = unit.Dp(30)
	logRowHeightDp       = unit.Dp(30)
	logLineSize          = unit.Sp(11.5)
	logLineHeight        = unit.Dp(16)
	logRowPadY           = unit.Dp(7)
	logMetaSize          = unit.Sp(10.5)
	logLevelSize         = unit.Sp(11)
	logHeaderSize        = unit.Sp(10.5)
	logCellPadX          = unit.Dp(14)
	logCellGap           = unit.Dp(8)
	logTimeColumn        = unit.Dp(96)
	logLevelColumn       = unit.Dp(74)
	logSourceColumn      = unit.Dp(108)
	logScrollbarWidth    = unit.Dp(5)
)

var (
	logAccent      = color.NRGBA{R: 0xba, G: 0x9a, B: 0xff, A: 0xff}
	logToolbarText = color.NRGBA{R: 0xf1, G: 0xef, B: 0xf8, A: 0xff}

	logHeaderFill  = color.NRGBA{R: 0x1a, G: 0x18, B: 0x25, A: 0xff}
	logHeaderText  = color.NRGBA{R: 0x8f, G: 0x88, B: 0x9f, A: 0xff}
	logRowFill     = color.NRGBA{R: 0x15, G: 0x14, B: 0x1d, A: 0xff}
	logRowAlt      = color.NRGBA{R: 0x17, G: 0x16, B: 0x1f, A: 0xff}
	logRowDivider  = color.NRGBA{R: 0x28, G: 0x25, B: 0x35, A: 0xff}
	logRowHover    = color.NRGBA{R: 0x1e, G: 0x1b, B: 0x2a, A: 0xff}
	logRowExpanded = color.NRGBA{R: 0x1c, G: 0x19, B: 0x26, A: 0xff}
	logTableBorder = color.NRGBA{R: 0x2c, G: 0x28, B: 0x3b, A: 0xff}

	logMessageColor = color.NRGBA{R: 0xf1, G: 0xef, B: 0xf8, A: 0xff}
	logErrorRowFill = color.NRGBA{R: 0x26, G: 0x17, B: 0x1b, A: 0xff}
	logWarnRowFill  = color.NRGBA{R: 0x24, G: 0x20, B: 0x15, A: 0xff}
	logDetailText   = color.NRGBA{R: 0xb9, G: 0xb3, B: 0xc6, A: 0xff}
	logMetaText     = color.NRGBA{R: 0xa8, G: 0xa2, B: 0xb8, A: 0xff}
	logSourceText   = color.NRGBA{R: 0xb0, G: 0xa9, B: 0xbd, A: 0xff}
	logTraceText    = color.NRGBA{R: 0x74, G: 0x6f, B: 0x82, A: 0xff}
	logDebugText    = color.NRGBA{R: 0x93, G: 0x8d, B: 0xa3, A: 0xff}
	logInfoText     = color.NRGBA{R: 0xb8, G: 0xb2, B: 0xc4, A: 0xff}
	logNoticeText   = color.NRGBA{R: 0xc3, G: 0xa9, B: 0xff, A: 0xff}
	logWarningText  = color.NRGBA{R: 0xe0, G: 0xbb, B: 0x7b, A: 0xff}
	logErrorText    = color.NRGBA{R: 0xec, G: 0x96, B: 0xa1, A: 0xff}

	logScrollTrack = color.NRGBA{R: 0x20, G: 0x1e, B: 0x2b, A: 0xff}
	logScrollThumb = color.NRGBA{R: 0x3a, G: 0x35, B: 0x48, A: 0xff}
	// logScrollThumbActive is the thumb while it is dragged with the mouse.
	logScrollThumbActive = color.NRGBA{R: 0x57, G: 0x50, B: 0x6b, A: 0xff}
)

var logFace = font.Font{Typeface: fontFamily, Weight: font.Normal}

type logLevelFilter struct {
	all   bool
	level slog.Level
}

var logFilterOptions = [...]logLevelFilter{
	{all: true},
	{level: applog.LevelTrace},
	{level: slog.LevelDebug},
	{level: slog.LevelInfo},
	{level: applog.LevelNotice},
	{level: slog.LevelWarn},
	{level: slog.LevelError},
}

func (f logLevelFilter) label() string {
	if f.all {
		return "Все уровни"
	}
	return applog.LevelLabel(f.level)
}

func (f logLevelFilter) matches(level slog.Level) bool {
	// Standard logging ladder: picking a level shows it and everything more
	// severe. "Trace" therefore shows the whole journal, "Debug" everything
	// except trace, and so on.
	return f.all || level >= f.level
}

type logScreen struct {
	typo *typography

	entries []applog.Entry
	paused  bool

	search      widget.Editor
	filter      logLevelFilter
	filterClick widget.Clickable
	filterHover hoverAnim
	pauseClick  widget.Clickable
	pauseHover  hoverAnim
	clearClick  widget.Clickable
	clearHover  hoverAnim
	autoClick   widget.Clickable
	autoHover   hoverAnim
	autoScroll  bool
	levelMenu   logLevelMenu

	lineStates map[uint64]*logLineState
	scrollTag  struct{}
	scroll     float32
	target     float32
	lastFrame  time.Time
	maxScroll  int
	bodyHeight int

	// scrollbarDrag tracks a pressed scrollbar thumb. scrollbarGrabY is the
	// distance from the press point to the thumb top, so dragging keeps the
	// thumb under the cursor; dragging sets the target directly instead of
	// smoothing it.
	scrollbarDrag    bool
	scrollbarGrabY   float32
	scrollbarTrackH  float32
	scrollbarThumbH  float32
	scrollbarDragTag struct{}

	back     footerNamedButton
	copy     footerNamedButton
	copiedAt time.Time
	onBack   func()
	hidden   func() bool
}

func newLogScreen(typo *typography) *logScreen {
	w := &logScreen{
		typo:       typo,
		filter:     logFilterOptions[0],
		autoScroll: true,
		lineStates: make(map[uint64]*logLineState),
		back:       newBackFooterButton(),
		copy: footerNamedButton{
			label: "Скопировать",
			style: footerCopyStyle,
			size:  titlePageButtonSize,
		},
	}
	w.search.SingleLine = true
	w.search.InputHint = key.HintText
	return w
}

func (w *logScreen) Close() {}

func (w *logScreen) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *logScreen) layout(gtx layout.Context) {
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
	clearWidth := gtx.Dp(logToolbarIconWidth)
	pauseWidth := gtx.Dp(logToolbarIconWidth)
	autoWidth := gtx.Dp(logAutoWidth)
	filterWidth := gtx.Dp(logFilterWidth)
	clearRect := image.Rect(inner.Max.X-clearWidth, toolbar.Min.Y, inner.Max.X, toolbar.Max.Y)
	pauseRect := image.Rect(clearRect.Min.X-gap-pauseWidth, toolbar.Min.Y, clearRect.Min.X-gap, toolbar.Max.Y)
	autoRect := image.Rect(pauseRect.Min.X-gap-autoWidth, toolbar.Min.Y, pauseRect.Min.X-gap, toolbar.Max.Y)
	filterRect := image.Rect(autoRect.Min.X-gap-filterWidth, toolbar.Min.Y, autoRect.Min.X-gap, toolbar.Max.Y)
	searchRect := image.Rect(inner.Min.X, toolbar.Min.Y, filterRect.Min.X-gap, toolbar.Max.Y)
	if searchRect.Dx() <= 0 {
		return
	}

	w.layoutSearchField(gtx, searchRect)

	if w.layoutFilterButton(gtx, filterRect) {
		w.levelMenu.toggle(filterRect)
		gtx.Execute(key.FocusCmd{Tag: nil})
		gtx.Execute(op.InvalidateCmd{})
	}

	if w.layoutAutoButton(gtx, autoRect) {
		w.setAutoScroll(!w.autoScroll)
		gtx.Execute(key.FocusCmd{Tag: nil})
		gtx.Execute(op.InvalidateCmd{})
	}

	// Pause and clear are icon-only. The pause button shades the same way as
	// autoscroll: grey while the log feeds, purple while it is held.
	pauseCol := logInfoText
	if w.paused {
		pauseCol = logAccent
	}
	if w.layoutLogIconButton(gtx, pauseRect, &w.pauseClick, &w.pauseHover, "pause", pauseCol) {
		w.togglePause()
		gtx.Execute(key.FocusCmd{Tag: nil})
		gtx.Execute(op.InvalidateCmd{})
	}
	if w.layoutLogIconButton(gtx, clearRect, &w.clearClick, &w.clearHover, "trash", logInfoText) {
		w.clearEntries()
		gtx.Execute(key.FocusCmd{Tag: nil})
		gtx.Execute(op.InvalidateCmd{})
	}

	entries := w.visibleEntries()
	contentGap := gtx.Dp(logToolbarGap)
	tableRect := image.Rect(inner.Min.X, toolbar.Max.Y+contentGap, inner.Max.X, inner.Max.Y)
	if tableRect.Dy() <= 0 {
		return
	}
	w.layoutTable(gtx, tableRect, entries)

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

// refreshEntries snapshots the ring only when its newest record changed. Pause
// simply skips this step, keeping the visible rows frozen while the logger
// continues to collect new entries.
func (w *logScreen) refreshEntries() {
	if w.paused {
		return
	}
	entries := applog.Recent()
	if len(entries) == len(w.entries) {
		if len(entries) == 0 || entries[len(entries)-1].ID == w.entries[len(w.entries)-1].ID {
			return
		}
	}
	w.entries = entries
}

func (w *logScreen) visibleEntries() []applog.Entry {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	out := make([]applog.Entry, 0, len(w.entries))
	for _, entry := range w.entries {
		if !w.filter.matches(entry.Level) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(logSearchText(entry)), query) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (w *logScreen) togglePause() {
	w.paused = !w.paused
	if !w.paused {
		w.refreshEntries()
	}
}

func (w *logScreen) clearEntries() {
	applog.Clear()
	w.entries = nil
	w.lineStates = make(map[uint64]*logLineState)
	w.scroll = 0
	w.target = 0
	w.maxScroll = 0
}

func (w *logScreen) setAutoScroll(on bool) {
	w.autoScroll = on
	if on {
		w.target = float32(w.maxScroll)
	}
}

func (w *logScreen) jumpToEnd() {
	w.autoScroll = true
	w.target = float32(w.maxScroll)
}

func (w *logScreen) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameHome},
			key.Filter{Name: key.NameEnd},
			key.Filter{Name: key.NamePageUp},
			key.Filter{Name: key.NamePageDown},
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameReturn},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			switch {
			case w.levelMenu.open:
				w.levelMenu.open = false
			case gtx.Focused(&w.search):
				gtx.Execute(key.FocusCmd{Tag: nil})
			case w.onBack != nil:
				w.onBack()
			}
			gtx.Execute(op.InvalidateCmd{})
			continue
		}
		if gtx.Focused(&w.search) {
			continue
		}

		changed := true
		line := gtx.Dp(logLineHeight) * 2
		page := max(line, w.bodyHeight-gtx.Dp(logLineHeight)*3)
		switch e.Name {
		case key.NameHome:
			w.autoScroll = false
			w.target = 0
		case key.NameEnd:
			w.jumpToEnd()
		case key.NamePageUp:
			w.autoScroll = false
			w.target -= float32(page)
		case key.NamePageDown:
			w.autoScroll = false
			w.target += float32(page)
		case key.NameUpArrow:
			w.autoScroll = false
			w.target -= float32(line)
		case key.NameDownArrow:
			w.autoScroll = false
			w.target += float32(line)
		case key.NameEnter, key.NameReturn:
			w.jumpToEnd()
		default:
			changed = false
		}
		if changed {
			w.clampTarget()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (w *logScreen) clampTarget() {
	maxScroll := float32(w.maxScroll)
	if w.target > maxScroll {
		w.target = maxScroll
	}
	if w.target < 0 {
		w.target = 0
	}
}

func (w *logScreen) lineState(id uint64) *logLineState {
	state := w.lineStates[id]
	if state == nil {
		state = &logLineState{}
		w.lineStates[id] = state
	}
	return state
}

func (w *logScreen) layoutSearchField(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	focused := gtx.Focused(&w.search)
	inner := drawFieldBox(gtx, local, focused, false, fieldBoxStyle{
		fill:        tokenDomainFill,
		border:      tokenDomainBorder,
		focusBorder: mainInputFocusBorder,
		radius:      logFieldRadius,
	})
	if inner.Empty() {
		return
	}

	padX := gtx.Dp(settingsFieldPadX)
	iconSize := gtx.Dp(unit.Dp(14))
	iconCenter := image.Pt(inner.Min.X+padX+iconSize/2, local.Dy()/2)
	drawSearchIcon(gtx, iconCenter, iconSize, logHeaderText)

	textLeft := iconCenter.X + iconSize/2 + gtx.Dp(unit.Dp(7))
	lineHeight := gtx.Dp(unit.Dp(18))
	textRect := image.Rect(
		textLeft,
		(local.Dy()-lineHeight)/2,
		local.Max.X-padX,
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
			settingsFieldSize,
			colorMaterial(gtx, mainTextColor),
			colorMaterial(gtx, mainSelectionColor),
		)
		editorStack.Pop()
	}
	if w.search.Len() == 0 && !focused {
		drawLogText(
			gtx,
			w.typo,
			"Поиск по логам...",
			logFace,
			settingsFieldSize,
			mainPlaceholderColor,
			textRect.Min.X,
			local.Dy()/2,
			textRect.Dx(),
			logCapHeightFor(gtx, w.typo, logFace, settingsFieldSize),
		)
	}
}

func (w *logScreen) layoutFilterButton(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	clicked := w.filterClick.Clicked(gtx)
	active := w.filterClick.Hovered() || w.levelMenu.open
	hover := w.filterHover.update(gtx, active)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fill := tokenDomainFill
	if active {
		fill = mixColor(tokenDomainFillHover, tokenDomainFill, 1-hover)
	}
	fillRounded(gtx, tokenDomainBorder, local, radius)
	fillRounded(gtx, fill, local.Inset(max(1, gtx.Dp(1))), max(0, radius-gtx.Dp(1)))

	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	capHeight := logCapHeightFor(gtx, w.typo, face, settingsButtonSize)
	textX := gtx.Dp(logToolbarPadX)
	chevronSize := gtx.Dp(unit.Dp(9))
	textWidth := local.Dx() - textX - gtx.Dp(logToolbarPadX) - chevronSize - gtx.Dp(unit.Dp(6))
	drawLogText(gtx, w.typo, w.filter.label(), face, settingsButtonSize, mainTextColor, textX, local.Dy()/2, textWidth, capHeight)
	chevronCenter := image.Pt(local.Max.X-gtx.Dp(logToolbarPadX)-chevronSize/2, local.Dy()/2)
	width := gtx.Dp(unit.Dp(1.6))
	if w.levelMenu.open {
		drawChevronUp(gtx, chevronCenter, chevronSize, width, settingsNoteColor)
	} else {
		drawChevronDown(gtx, chevronCenter, chevronSize, width, settingsNoteColor)
	}

	w.filterClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: local.Size()}
	})
	return clicked
}

func (w *logScreen) layoutLogIconButton(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, hover *hoverAnim, icon string, col color.NRGBA) bool {
	if rect.Empty() {
		return false
	}
	clicked := click.Clicked(gtx)
	progress := hover.update(gtx, click.Hovered())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fill := mixColor(tokenDomainFill, tokenDomainFillHover, progress)
	fillRounded(gtx, tokenDomainBorder, local, radius)
	fillRounded(gtx, fill, local.Inset(max(1, gtx.Dp(1))), max(0, radius-gtx.Dp(1)))

	iconSize := gtx.Dp(logToolbarIconSize)
	col = mixColor(col, logToolbarText, progress*0.4)
	drawAppIcon(gtx, icon, centerOf(local), iconSize, col)

	click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: local.Size()}
	})
	return clicked
}

func (w *logScreen) layoutAutoButton(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	clicked := w.autoClick.Clicked(gtx)
	progress := w.autoHover.update(gtx, w.autoClick.Hovered())
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(logFieldRadius)
	fill := mixColor(tokenDomainFill, tokenDomainFillHover, progress)
	fillRounded(gtx, tokenDomainBorder, local, radius)
	fillRounded(gtx, fill, local.Inset(max(1, gtx.Dp(1))), max(0, radius-gtx.Dp(1)))

	col := logInfoText
	if w.autoScroll {
		col = logAccent
	}
	col = mixColor(col, logToolbarText, progress*0.4)
	iconSize := gtx.Dp(unit.Dp(20))
	drawAutoscrollIcon(gtx, centerOf(local), iconSize, col)

	w.autoClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
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
	levelWidth := min(gtx.Dp(logLevelColumn), width/6)
	sourceWidth := min(gtx.Dp(logSourceColumn), width/5)
	timeX := padX
	levelX := timeX + timeWidth
	sourceX := levelX + levelWidth
	messageX := sourceX + sourceWidth
	rightPad := padX + gtx.Dp(logScrollbarWidth) + gtx.Dp(unit.Dp(4))
	return logColumns{
		timeX:        timeX,
		timeWidth:    max(timeWidth-gap, 1),
		levelX:       levelX,
		levelWidth:   max(levelWidth-gap, 1),
		sourceX:      sourceX,
		sourceWidth:  max(sourceWidth-gap, 1),
		messageX:     messageX,
		messageWidth: max(width-messageX-rightPad, 1),
	}
}

func (w *logScreen) layoutTable(gtx layout.Context, rect image.Rectangle, entries []applog.Entry) {
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
	fill(gtx, logRowDivider, image.Rect(rect.Min.X, headerRect.Max.Y-max(1, gtx.Dp(1)), rect.Max.X, headerRect.Max.Y))

	body := image.Rect(rect.Min.X, headerRect.Max.Y, rect.Max.X, rect.Max.Y)
	if body.Dy() > 0 {
		w.layoutTableBody(gtx, body, entries)
	}

	clipStack.Pop()
	strokeRoundedRect(gtx, rect, float32(radius), float32(max(1, gtx.Dp(1))), logTableBorder)
}

func (w *logScreen) layoutTableHeader(gtx layout.Context, rect image.Rectangle) {
	if rect.Dy() <= 0 {
		return
	}
	cols := logColumnsForWidth(gtx, rect.Dx())
	centerY := rect.Min.Y + rect.Dy()/2
	headerFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	capHeight := logCapHeightFor(gtx, w.typo, headerFace, logHeaderSize)
	drawHeader := func(text string, x, maxWidth int) {
		drawLogText(gtx, w.typo, text, headerFace, logHeaderSize, logHeaderText, rect.Min.X+x, centerY, maxWidth, capHeight)
	}
	drawHeader("Time", cols.timeX, cols.timeWidth)
	drawHeader("Level", cols.levelX, cols.levelWidth)
	drawHeader("Source", cols.sourceX, cols.sourceWidth)
	drawHeader("Message", cols.messageX, cols.messageWidth)
}

// consumeLineClick processes expansion clicks for one row. It must run with
// the row's own clip so the clickable area does not cover the whole body; and
// it must run before heights are measured so a growing row never shifts the
// rows below it by a stale offset for a single frame.
func (w *logScreen) consumeLineClick(gtx layout.Context, width, height int, state *logLineState, entry applog.Entry) {
	if height <= 0 {
		return
	}
	rowClip := clip.Rect(image.Rect(0, 0, width, height)).Push(gtx.Ops)
	defer rowClip.Pop()
	rowGtx := gtx
	rowGtx.Constraints = layout.Exact(image.Pt(width, height))
	collapsed := truncateText(rowGtx, w.typo, entry.Message, logFace, logLineSize, logColumnsForWidth(rowGtx, width).messageWidth)
	expandable := collapsed != entry.Message || entry.Detail != ""
	if (expandable || state.expanded) && state.click.Clicked(rowGtx) {
		state.expanded = !state.expanded
		gtx.Execute(key.FocusCmd{Tag: nil})
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (w *logScreen) layoutTableBody(gtx layout.Context, body image.Rectangle, entries []applog.Entry) {
	if body.Dy() <= 0 {
		return
	}
	if len(entries) == 0 {
		text := "Записей пока нет."
		if len(w.entries) > 0 {
			text = "Ничего не найдено."
		}
		emptyState(gtx, w.typo, body, text, logLineSize, logInfoText)
		return
	}

	heights := make([]int, len(entries))
	total := 0
	{
		// Expansion clicks must be consumed before the heights are measured:
		// a row that grows in the middle of the measurement pass would push
		// every following row by a stale offset for one frame, making the text
		// jump. The rows were drawn (and registered their click targets) in
		// the previous frame with exactly these bounds, so the click processing
		// here sees the same hit areas.
		offset := int(math.Round(float64(w.scroll)))
		y := -offset
		for index, entry := range entries {
			state := w.lineState(entry.ID)
			height := w.logRowHeight(gtx, entry, body.Dx(), state.expanded)
			if y+height > 0 && y < body.Dy() {
				stack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
				w.consumeLineClick(gtx, body.Dx(), height, state, entry)
				stack.Pop()
				height = w.logRowHeight(gtx, entry, body.Dx(), state.expanded)
			}
			heights[index] = height
			total += height
			y += height
		}
	}
	maxScroll := max(0, total-body.Dy())
	w.maxScroll = maxScroll
	w.bodyHeight = body.Dy()
	w.scrollStep(gtx, maxScroll)

	stack := op.Offset(body.Min).Push(gtx.Ops)
	clipStack := clip.Rect(image.Rectangle{Max: body.Size()}).Push(gtx.Ops)

	offset := int(math.Round(float64(w.scroll)))
	y := -offset
	for index, entry := range entries {
		height := heights[index]
		if y+height <= 0 {
			y += height
			continue
		}
		if y >= body.Dy() {
			break
		}
		rowStack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		rowGtx := gtx
		rowGtx.Constraints = layout.Exact(image.Pt(body.Dx(), height))
		w.layoutLine(rowGtx, entry, body.Dx(), height, index, w.lineState(entry.ID))
		rowStack.Pop()
		y += height
	}

	if maxScroll > 0 {
		w.drawScrollbar(gtx, body.Size(), total, maxScroll)
	}

	clipStack.Pop()
	stack.Pop()
}

func (w *logScreen) scrollStep(gtx layout.Context, maxScroll int) {
	event.Op(gtx.Ops, &w.scrollTag)
	delta := float32(0)
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  &w.scrollTag,
			Kinds:   pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: math.MinInt, Max: math.MaxInt},
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Kind != pointer.Scroll {
			continue
		}
		delta += e.Scroll.Y
	}
	if delta != 0 {
		w.autoScroll = false
		w.target += delta * 0.85
		w.clampTarget()
		if w.target >= float32(maxScroll) {
			w.autoScroll = true
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	if w.autoScroll {
		w.target = float32(maxScroll)
	}
	w.clampTarget()

	if abs := float32(math.Abs(float64(w.target - w.scroll))); abs < 0.35 {
		w.scroll = w.target
	} else {
		dt := float32(0)
		if !w.lastFrame.IsZero() {
			dt = float32(gtx.Now.Sub(w.lastFrame).Seconds())
		}
		w.lastFrame = gtx.Now
		if dt <= 0 || dt > 0.1 {
			dt = 1.0 / 60.0
		}
		smoothing := 1 - float32(math.Exp(float64(-dt*20)))
		w.scroll += (w.target - w.scroll) * smoothing
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if w.scroll < 0 {
		w.scroll = 0
	}
	if max := float32(maxScroll); w.scroll > max {
		w.scroll = max
	}
}

func (w *logScreen) drawScrollbar(gtx layout.Context, viewport image.Point, total, maxScroll int) {
	width := gtx.Dp(logScrollbarWidth)
	inset := gtx.Dp(unit.Dp(4))
	track := image.Rect(
		viewport.X-width-inset,
		inset,
		viewport.X-inset,
		viewport.Y-inset,
	)
	if track.Dy() <= 0 {
		return
	}
	radius := width / 2
	fillRounded(gtx, logScrollTrack, track, radius)
	thumbHeight := max(gtx.Dp(unit.Dp(24)), int(float64(viewport.Y)*float64(viewport.Y)/float64(total)))
	thumbHeight = min(thumbHeight, track.Dy())
	available := track.Dy() - thumbHeight
	thumbY := track.Min.Y
	if maxScroll > 0 && available > 0 {
		thumbY += int(float64(available) * float64(w.scroll) / float64(maxScroll))
		thumbY = min(thumbY, track.Min.Y+available)
	}
	thumb := image.Rect(track.Min.X, thumbY, track.Max.X, thumbY+thumbHeight)

	// The thumb is draggable: press grabs it, move scrolls, release drops it.
	// The pointer area is scoped to the track so clicks on the rows do not
	// interfere with the drag.
	if maxScroll > 0 {
		trackArea := clip.Rect(track).Push(gtx.Ops)
		// The event target must be registered in the clipped track area;
		// gtx.Event alone only reads an already registered target. Without this
		// Op the thumb hovered but never received Press or Drag events.
		event.Op(gtx.Ops, &w.scrollbarDragTag)
		pointer.CursorPointer.Add(gtx.Ops)
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: &w.scrollbarDragTag,
				Kinds:  pointer.Press | pointer.Drag | pointer.Release,
			})
			if !ok {
				break
			}
			e, ok := ev.(pointer.Event)
			if !ok {
				continue
			}
			position := float32(e.Position.Y)
			switch e.Kind {
			case pointer.Press:
				w.scrollbarDrag = true
				w.autoScroll = false
				w.scrollbarTrackH = float32(track.Dy())
				w.scrollbarThumbH = float32(thumbHeight)
				w.scrollbarGrabY = position - float32(thumbY)
				if w.scrollbarGrabY < 0 {
					w.scrollbarGrabY = 0
				}
				if w.scrollbarGrabY > float32(thumbHeight) {
					w.scrollbarGrabY = float32(thumbHeight)
				}
				w.setScrollFromThumb(position-float32(track.Min.Y), maxScroll)
				w.scroll = w.target
			case pointer.Drag, pointer.Move:
				if w.scrollbarDrag {
					w.setScrollFromThumb(position-float32(track.Min.Y), maxScroll)
					w.scroll = w.target
				}
			case pointer.Release, pointer.Cancel:
				w.scrollbarDrag = false
			}
		}
		trackArea.Pop()
	}

	thumbColor := logScrollThumb
	if w.scrollbarDrag {
		thumbColor = logScrollThumbActive
	}
	fillRounded(gtx, thumbColor, thumb, radius)
}

// setScrollFromThumb maps a pointer Y inside the track to the scroll target.
// scrollbarGrabY is the press offset inside the thumb, so the thumb top keeps
// following the cursor instead of jumping to it.
func (w *logScreen) setScrollFromThumb(pointerY float32, maxScroll int) {
	available := w.scrollbarTrackH - w.scrollbarThumbH
	if available <= 0 || maxScroll <= 0 {
		w.target = 0
		return
	}
	top := pointerY - w.scrollbarGrabY
	if top < 0 {
		top = 0
	}
	if top > available {
		top = available
	}
	w.target = float32(maxScroll) * top / available
}

type logLevelMenu struct {
	open   bool
	anchor image.Rectangle
	clicks [len(logFilterOptions)]widget.Clickable
	hovers [len(logFilterOptions)]hoverAnim
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
	pad := gtx.Dp(settingsProviderMenuPad)
	itemHeight := gtx.Dp(settingsProviderItemHeight)
	itemGap := gtx.Dp(settingsProviderItemGap)
	height := len(logFilterOptions)*itemHeight + 2*pad
	if len(logFilterOptions) > 1 {
		height += (len(logFilterOptions) - 1) * itemGap
	}
	menuRect := widgets.PlacePopupList(m.anchor, height, gtx.Dp(settingsProviderMenuGap), bounds)
	if menuRect.Empty() {
		return
	}
	drawPopupMenu(gtx, menuRect)
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	capHeight := logCapHeightFor(gtx, typo, face, settingsButtonSize)
	for index, option := range logFilterOptions {
		itemRect := image.Rect(
			menuRect.Min.X+pad,
			menuRect.Min.Y+pad+index*(itemHeight+itemGap),
			menuRect.Max.X-pad,
			menuRect.Min.Y+pad+index*(itemHeight+itemGap)+itemHeight,
		)
		clicked := m.clicks[index].Clicked(gtx)
		hover := m.hovers[index].update(gtx, m.clicks[index].Hovered())
		switch {
		case option == current:
			fillRounded(gtx, settingsMenuSelected, itemRect, gtx.Dp(unit.Dp(6)))
		case hover > 0:
			fillRounded(gtx, tokenDomainFillHover, itemRect, gtx.Dp(unit.Dp(6)))
		}
		textColor := mainTextColor
		if option == current {
			textColor = badgeTextColor
		}
		padX := gtx.Dp(unit.Dp(10))
		drawLogText(gtx, typo, option.label(), face, settingsButtonSize, textColor, itemRect.Min.X+padX, itemRect.Min.Y+itemRect.Dy()/2, itemRect.Dx()-2*padX, capHeight)
		stack := op.Offset(itemRect.Min).Push(gtx.Ops)
		m.clicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Dimensions{Size: itemRect.Size()}
		})
		stack.Pop()
		if clicked {
			onSelect(option)
			m.open = false
			gtx.Execute(key.FocusCmd{Tag: nil})
			gtx.Execute(op.InvalidateCmd{})
			return
		}
	}
}

func (w *logScreen) layoutFooter(gtx layout.Context) {
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
		text := applog.LastErrorContext()
		if text == "" {
			text = "Ошибок пока не было."
		}
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
