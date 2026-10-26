package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/animelib"
	"github.com/arfush/yonime/internal/library"
)

const (
	mainPaddingX         = unit.Dp(28)
	mainPaddingTop       = unit.Dp(16)
	mainInputHeight      = unit.Dp(60)
	mainInputRadius      = unit.Dp(16)
	mainMascotSize       = unit.Dp(42)
	mainNextSize         = unit.Dp(40)
	mainNextInset        = unit.Dp(10)
	mainInputGap         = unit.Dp(12)
	mainHistoryGap       = unit.Dp(18)
	mainHistoryHeight    = unit.Dp(34)
	mainListGap          = unit.Dp(6)
	mainHistoryTailSpace = unit.Dp(14)
	mainRowHeight        = unit.Dp(88)
	mainRowRadius        = unit.Dp(12)
	mainRowPadding       = unit.Dp(12)
	mainDragHandleWidth  = unit.Dp(20)
	mainPosterWidth      = unit.Dp(40)
	mainPosterHeight     = unit.Dp(52)
	mainControlSize      = unit.Dp(34)
	mainControlRadius    = unit.Dp(9)
	mainControlGap       = unit.Dp(8)
	mainHintHeight       = unit.Dp(24)
	mainHintPaddingX     = unit.Dp(8)
	mainHintGap          = unit.Dp(10)
	mainHistoryTitleSize = unit.Sp(16)
	mainHistoryCountSize = unit.Sp(11.5)
	mainClearSize        = unit.Sp(11.5)
	mainRowTitleSize     = unit.Sp(14)
	// Status pills are deliberately 10% larger than the original 10sp/23dp/9dp
	// set: the release and user tags carry real information and were too easy
	// to skim in a history row.
	mainStatusSize      = unit.Sp(11)
	mainPosterTitleSize = unit.Sp(15)
	mainInputTextSize   = unit.Sp(14)
	mainHintTextSize    = unit.Sp(10)
	mainStatusHeight    = unit.Dp(25.3)
	mainStatusPaddingX  = unit.Dp(9.9)
	mainStatusGap       = unit.Dp(6)
	mainFilterSize      = unit.Sp(10.5)
	mainFilterPaddingX  = unit.Dp(10)
	mainFilterGap       = unit.Dp(4)
	mainFilterMaxGap    = unit.Dp(6)
	mainFilterHeight    = unit.Dp(28)
	// mainFilterReserve keeps the filter row anchored when the number in
	// "История (n)" grows from one digit to another. Four digits fit comfortably;
	// wider counts only push the row if they actually collide with it.
	mainFilterReserve    = unit.Dp(42)
	mainFrameTime        = time.Second / 60
	mainSpinnerFrameTime = time.Second / 60
	// mainErrorFeedbackDuration is how long the Next button and the mascot stay
	// red after an empty link or a failed load.
	mainErrorFeedbackDuration = 3 * time.Second
	// mainIconShakeOffset is the peak horizontal offset of the Next arrow. The
	// arrow is only about 13 dp wide, so a large offset reads as jumping.
	mainIconShakeOffset = unit.Dp(0.7)
	// mainMascotShakeOffset is deliberately smaller: at 42 dp the character
	// reads any larger movement as jumping, not trembling.
	mainMascotShakeOffset = unit.Dp(0.9)
	mainErrorShakeHz      = 11.0
)

var (
	mainInputColor        = color.NRGBA{R: 0x12, G: 0x11, B: 0x1a, A: 0xff}
	mainInputBorder       = color.NRGBA{R: 0x2a, G: 0x25, B: 0x38, A: 0xff}
	mainInputFocusBorder  = color.NRGBA{R: 0x64, G: 0x4a, B: 0xa8, A: 0xff}
	mainPlaceholderColor  = color.NRGBA{R: 0x76, G: 0x70, B: 0x84, A: 0xff}
	mainTextColor         = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	mainMutedColor        = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	mainFaintColor        = color.NRGBA{R: 0x5e, G: 0x58, B: 0x6d, A: 0xff}
	mainSelectionColor    = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0x66}
	mainAccentColor1      = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	mainAccentColor2      = color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff}
	mainDisabledColor1    = color.NRGBA{R: 0x49, G: 0x3b, B: 0x67, A: 0xff}
	mainDisabledColor2    = color.NRGBA{R: 0x35, G: 0x2a, B: 0x50, A: 0xff}
	mainHintFillColor     = color.NRGBA{R: 0x1b, G: 0x18, B: 0x26, A: 0xff}
	mainHintBorderColor   = color.NRGBA{R: 0x2d, G: 0x28, B: 0x3c, A: 0xff}
	mainHintTextColor     = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	mainRowColor          = color.NRGBA{R: 0x12, G: 0x11, B: 0x1a, A: 0xff}
	mainRowHoverColor     = color.NRGBA{R: 0x18, G: 0x16, B: 0x22, A: 0xff}
	mainRowSelectedColor  = color.NRGBA{R: 0x1a, G: 0x16, B: 0x26, A: 0xff}
	mainRowBorderColor    = color.NRGBA{R: 0x24, G: 0x21, B: 0x2f, A: 0xff}
	mainRowHoverBorder    = color.NRGBA{R: 0x35, G: 0x2f, B: 0x48, A: 0xff}
	mainControlColor      = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
	mainControlFill       = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	mainControlBorder     = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	mainPlayButtonFill    = color.NRGBA{R: 0x1c, G: 0x42, B: 0x30, A: 0xff}
	mainPlayButtonBorder  = color.NRGBA{R: 0x2f, G: 0x6a, B: 0x49, A: 0xff}
	mainTrashButtonFill   = color.NRGBA{R: 0x43, G: 0x1e, B: 0x26, A: 0xff}
	mainTrashButtonBorder = color.NRGBA{R: 0x71, G: 0x2e, B: 0x39, A: 0xff}
	mainDragDotColor      = color.NRGBA{R: 0x53, G: 0x4d, B: 0x63, A: 0xff}
	mainDragDotActive     = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}
	mainDangerColor       = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}
	mainDangerColor2      = color.NRGBA{R: 0xd1, G: 0x31, B: 0x48, A: 0xff}
	mainStatusDoneText    = color.NRGBA{R: 0x58, G: 0xdc, B: 0x9a, A: 0xff}
	mainStatusDoneFill    = color.NRGBA{R: 0x10, G: 0x2c, B: 0x21, A: 0xff}
	mainStatusDoneBorder  = color.NRGBA{R: 0x24, G: 0x55, B: 0x3e, A: 0xff}
	mainStatusLiveText    = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}
	mainStatusLiveFill    = color.NRGBA{R: 0x1f, G: 0x18, B: 0x35, A: 0xff}
	mainStatusLiveBorder  = color.NRGBA{R: 0x42, G: 0x32, B: 0x6e, A: 0xff}
	mainStatusAnnText     = color.NRGBA{R: 0x7f, G: 0xd6, B: 0xff, A: 0xff}
	mainStatusAnnFill     = color.NRGBA{R: 0x10, G: 0x24, B: 0x2e, A: 0xff}
	mainStatusAnnBorder   = color.NRGBA{R: 0x24, G: 0x53, B: 0x6a, A: 0xff}
)

type historyRow struct {
	titleClick  widget.Clickable
	playClick   widget.Clickable
	trashClick  widget.Clickable
	statusClick widget.Clickable

	hover       hoverAnim
	playHover   hoverAnim
	trashHover  hoverAnim
	statusHover hoverAnim
}

type titleLoadResult struct {
	sequence int
	title    historyTitle
	history  []historyTitle
	err      error
}

type coverResult struct {
	id   int64
	path string
	err  error
}

// mainWindow is the application workspace: URL input and local history.
// historyPlayResult tells the history row whether direct playback started; any
// other result opens the title page instead.
type historyPlayResult int

const (
	historyPlayFailed historyPlayResult = iota
	historyPlayStarted
)

type mainWindow struct {
	typo    *typography
	library *library.Library

	editor widget.Editor
	next   widget.Clickable
	clear  widget.Clickable

	nextHover  hoverAnim
	clearHover hoverAnim
	hintAnim   hoverAnim

	hintPulse      time.Time
	hintForcedHide bool
	hintForceAt    time.Time

	// allHistory keeps the complete library order. history is the filtered view
	// currently drawn; it is rebuilt from allHistory whenever the filter or a
	// deletion changes.
	allHistory []historyTitle
	history    []historyTitle
	rows       []historyRow
	list       layout.List

	filter       userStatusFilter
	filterClicks [6]widget.Clickable
	filterHovers [6]hoverAnim
	statusMenu   userStatusMenu

	listRect      image.Rectangle
	listRowHeight int
	listGap       int

	selected          int
	loading           bool
	loadingTarget     int
	loadingAnimateRow bool
	loadError         string
	loadSequence      int
	loadCancel        context.CancelFunc
	loadResults       chan titleLoadResult
	coverResults      chan coverResult
	invalidate        func()
	// refreshRequests lets background work (episode totals, notification
	// checks) ask the UI thread to reload history without touching its slices.
	refreshRequests chan struct{}
	// refreshPending remembers a request that arrived while a row was being
	// dragged: rebuilding the list mid-drag would drop the preview order.
	refreshPending bool
	// hidden reports that the window is parked in the notification area. The
	// mascot and spinner clocks stop scheduling frames while it is true.
	hidden        func() bool
	onOpenTitle   func(historyTitle)
	onPlayHistory func(historyTitle) historyPlayResult
	onUserStatus  func(historyTitle, library.UserStatus) error
	onError       func(error)
	onClearError  func()

	playing   bool
	playingID int64

	drag    dragState
	dragTag struct{}

	deleteRequested bool
	deleteIndex     int

	inputFocused bool
	errorAt      time.Time
	placeholder  placeholderState

	undo undoDeleteState
}

func newMainWindow(typo *typography, lib *library.Library, history []library.Title) *mainWindow {
	displayHistory := make([]historyTitle, 0, len(history))
	for _, item := range history {
		displayHistory = append(displayHistory, titleFromLibrary(item))
	}
	m := &mainWindow{
		typo:            typo,
		library:         lib,
		allHistory:      displayHistory,
		selected:        -1,
		loadingTarget:   -1,
		loadResults:     make(chan titleLoadResult, 4),
		coverResults:    make(chan coverResult, 16),
		refreshRequests: make(chan struct{}, 1),
		drag:            dragState{index: -1},
		placeholder:     newPlaceholderState(),
	}
	m.editor.SingleLine = true
	m.editor.InputHint = key.HintURL
	m.list.Axis = layout.Vertical
	m.history = m.visibleHistory(displayHistory)
	m.rows = make([]historyRow, len(m.history))
	return m
}

// setInputFocused leaves history selection when the URL field becomes
// focused. Clicking the input after selecting a history row must not keep the
// row highlighted.
func (m *mainWindow) setInputFocused(focused bool) {
	if focused && !m.inputFocused {
		m.selected = -1
	}
	m.inputFocused = focused
}

// clearSelection drops the history highlight. It is called when the window
// loses focus so a title does not stay selected while another application is
// in front.
func (m *mainWindow) clearSelection() {
	m.selected = -1
}

// noteWindowPress clears the history selection when the press happened outside
// the selected card. Any other control, the header, the footer, the URL field
// or empty window space therefore releases the highlight. Pressing the
// selected row itself (including its play, trash and drag handle) keeps it.
func (m *mainWindow) noteWindowPress(contentPoint image.Point) {
	if m.drag.active || m.selected < 0 || m.selected >= len(m.history) {
		return
	}
	if m.listRect.Empty() || m.listRowHeight <= 0 {
		m.selected = -1
		return
	}
	stride := m.listRowHeight + m.listGap
	top := m.listRect.Min.Y + m.historyRowTop(m.selected, stride)
	card := image.Rect(
		m.listRect.Min.X,
		top,
		m.listRect.Max.X,
		top+m.listRowHeight,
	)
	visible := card.Intersect(m.listRect)
	if visible.Empty() || !contentPoint.In(visible) {
		m.selected = -1
	}
}

// onShown resets the URL prompt when the main screen becomes visible again.
// Returning from the preparation screen must not replay a partial typewriter
// animation.
func (m *mainWindow) onShown() {
	m.placeholder.reset()
	// A link typed before opening a title must not be waiting in the input when
	// the user comes back to the main screen.
	m.editor.SetText("")
	m.editor.ClearSelection()
}

// triggerEmptyLink flashes the red feedback when Next is pressed with an empty
// URL field. An empty field is a prompt to type, not an error, so nothing is
// sent to the header.
func (m *mainWindow) triggerEmptyLink(gtx layout.Context) {
	if m.loading {
		return
	}
	m.errorAt = gtx.Now
	m.placeholder.showFull(gtx.Now)
	gtx.Execute(op.InvalidateCmd{})
}

func (m *mainWindow) reportError(err error) {
	if err == nil {
		return
	}
	if m.onError != nil {
		m.onError(err)
	}
}

// clearLoadError drops the message and the error feedback together, so a retry
// paints the button purple again immediately.
func (m *mainWindow) clearLoadError() {
	m.loadError = ""
	m.errorAt = time.Time{}
	if m.onClearError != nil {
		m.onClearError()
	}
}

// errorFeedbackActive reports whether the red error state is still running.
func errorFeedbackActive(now, start time.Time) bool {
	if start.IsZero() {
		return false
	}
	elapsed := now.Sub(start)
	return elapsed >= 0 && elapsed < mainErrorFeedbackDuration
}

// errorShake returns the horizontal tremor of the error feedback in [-1, 1].
// The amplitude fades out, so the button settles before the red window ends
// instead of stopping abruptly.
func errorShake(now, start time.Time) float32 {
	if !errorFeedbackActive(now, start) {
		return 0
	}
	elapsed := now.Sub(start)
	decay := 1 - float32(elapsed)/float32(mainErrorFeedbackDuration)
	seconds := float64(elapsed) / float64(time.Second)
	return decay * float32(math.Sin(2*math.Pi*mainErrorShakeHz*seconds))
}

// errorShakePixels converts the tremor into device pixels for the given peak
// offset.
func errorShakePixels(gtx layout.Context, shake float32, peak unit.Dp) int {
	if shake == 0 {
		return 0
	}
	return int(math.Round(float64(shake) * float64(gtx.Dp(peak))))
}

// updateCovers consumes covers downloaded in the background after a title is
// saved.
func (m *mainWindow) updateCovers(gtx layout.Context) {
	for {
		select {
		case result := <-m.coverResults:
			if result.err != nil || result.path == "" {
				continue
			}
			for index := range m.history {
				if m.history[index].id == result.id {
					m.history[index].coverPath = result.path
					m.history[index].coverNeedsRetry = false
				}
			}
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

// Close releases resources held by the main window and cancels an in-flight
// title fetch.
func (m *mainWindow) Close() {
	if m.loadCancel != nil {
		m.loadCancel()
		m.loadCancel = nil
	}
}

func (m *mainWindow) layout(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	m.handleKeys(gtx)
	m.updateLoading(gtx)
	m.updateCovers(gtx)
	m.updateUndoDelete(gtx)
	m.updateHistoryRefresh()
	focused := gtx.Focused(&m.editor)
	m.setInputFocused(focused)
	m.placeholder.update(gtx, m.editor.Len(), focused)
	m.updateHint(gtx, focused)

	padX := gtx.Dp(mainPaddingX)
	if size.X < gtx.Dp(700) {
		padX = gtx.Dp(unit.Dp(20))
	}
	padTop := gtx.Dp(mainPaddingTop)
	inputHeight := gtx.Dp(mainInputHeight)
	if size.Y < gtx.Dp(380) {
		inputHeight = gtx.Dp(unit.Dp(54))
	}

	inputRect := image.Rect(
		padX,
		padTop,
		size.X-padX,
		padTop+inputHeight,
	)
	if inputRect.Empty() {
		return
	}
	m.layoutInput(gtx, inputRect, focused)

	historyGap := gtx.Dp(mainHistoryGap)
	historyHeight := gtx.Dp(mainHistoryHeight)
	headerRect := image.Rect(
		inputRect.Min.X,
		inputRect.Max.Y+historyGap,
		inputRect.Max.X,
		inputRect.Max.Y+historyGap+historyHeight,
	)
	m.layoutHistoryHeader(gtx, headerRect)

	listTop := headerRect.Max.Y + gtx.Dp(mainListGap)
	// The list reaches the footer directly; no padding strip is left below the
	// cards. The list clips its own content, so a partially visible last row is
	// still handled correctly.
	listRect := image.Rect(
		inputRect.Min.X,
		listTop,
		inputRect.Max.X,
		size.Y,
	)
	if !listRect.Empty() {
		m.layoutHistory(gtx, listRect)
	}

	// Both the mascot and the prompt typewriter are animated from this clock.
	// A window hidden in the notification area stops asking for frames: nothing
	// is visible, and the 60 Hz wake-ups would only burn CPU.
	if m.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(mainFrameTime)})
	}
}

// animationsRun reports whether this screen may keep scheduling its own frames.
func (m *mainWindow) animationsRun() bool {
	return m.hidden == nil || !m.hidden()
}

func (m *mainWindow) handleKeys(gtx layout.Context) {
	// Only claim the shortcuts handled here. A filter matching every Ctrl
	// chord would consume Ctrl+C, Ctrl+V, Ctrl+Z and the like before the
	// editor gets a chance to process them.
	shortcutFilters := []event.Filter{
		key.Filter{Name: "K", Required: key.ModCtrl},
		key.Filter{Name: "k", Required: key.ModCtrl},
		key.Filter{Name: "Л", Required: key.ModCtrl},
		key.Filter{Name: "л", Required: key.ModCtrl},
		key.Filter{Name: key.NameDeleteBackward, Required: key.ModCtrl, Optional: key.ModShift},
		key.Filter{Name: key.NameDeleteForward, Required: key.ModCtrl, Optional: key.ModShift},
	}
	for {
		ev, ok := gtx.Event(shortcutFilters...)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case isCtrlK(e.Name):
			m.focusInput(gtx)
		case e.Name == key.NameDeleteBackward:
			if gtx.Focused(&m.editor) {
				deleteEditorWord(&m.editor, true)
				gtx.Execute(op.InvalidateCmd{})
			}
		case e.Name == key.NameDeleteForward:
			if gtx.Focused(&m.editor) {
				deleteEditorWord(&m.editor, false)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	filters := []event.Filter{
		key.Filter{Name: key.NameEscape},
		key.Filter{Name: key.NameUpArrow},
		key.Filter{Name: key.NameDownArrow},
		key.Filter{Name: key.NameReturn},
		key.Filter{Name: key.NameEnter},
	}
	// Single-letter shortcuts and plain Delete belong to the text editor while
	// it has focus. Registering their filters then would consume the events
	// before the editor sees them, even if the handler itself did nothing.
	if !gtx.Focused(&m.editor) {
		filters = append(filters,
			key.Filter{Name: key.NameLeftArrow},
			key.Filter{Name: key.NameRightArrow},
			key.Filter{Name: key.NameDeleteForward},
			key.Filter{Name: "P"},
			key.Filter{Name: "p"},
			key.Filter{Name: "З"},
			key.Filter{Name: "з"},
		)
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			if m.statusMenu.open {
				m.statusMenu.close()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			m.cancelLoading(gtx)
			m.focusHistory(gtx)
			continue
		}
		if m.statusMenu.open {
			continue
		}
		switch {
		case e.Name == key.NameLeftArrow:
			m.stepFilter(gtx, -1)
		case e.Name == key.NameRightArrow:
			m.stepFilter(gtx, 1)
		case e.Name == key.NameUpArrow:
			m.moveSelection(gtx, -1)
		case e.Name == key.NameDownArrow:
			m.moveSelection(gtx, 1)
		case isPlayKey(e.Name):
			if m.selected >= 0 && m.selected < len(m.history) {
				m.playHistory(gtx, m.selected)
			}
		case e.Name == key.NameDeleteForward:
			if m.selected >= 0 && m.selected < len(m.history) {
				m.deleteHistoryItem(gtx, m.selected)
			}
		case e.Name == key.NameReturn, e.Name == key.NameEnter:
			if m.loading {
				continue
			}
			if gtx.Focused(&m.editor) {
				if strings.TrimSpace(m.editor.Text()) == "" {
					m.triggerEmptyLink(gtx)
				} else {
					m.startLoading(gtx, -1, false)
				}
				continue
			}
			if m.selected >= 0 && m.selected < len(m.history) {
				m.startLoading(gtx, m.selected, false)
			}
		}
	}
}

func matchesLetterKey(name key.Name, latin, localized string) bool {
	switch strings.ToUpper(string(name)) {
	case latin, localized:
		return true
	default:
		return false
	}
}

func isCtrlK(name key.Name) bool {
	return matchesLetterKey(name, "K", "Л")
}

func isPlayKey(name key.Name) bool {
	return matchesLetterKey(name, "P", "З")
}

func isSaveKey(name key.Name) bool {
	return matchesLetterKey(name, "S", "Ы")
}

func (m *mainWindow) shortcutInfo() shortcutInfo {
	return shortcutInfo{
		page: "Главная",
		items: []shortcutItem{
			{keys: "F1", label: "Открыть или закрыть сводку"},
			{keys: "← / →", label: "Переключить фильтр истории"},
			{keys: "↑ / ↓", label: "Выбрать тайтл в истории"},
			{keys: "Enter", label: "Открыть выбранный тайтл"},
			{keys: "P", label: "Смотреть выбранный тайтл"},
			{keys: "Del", label: "Удалить тайтл из истории"},
			{keys: "Ctrl + K", label: "Перейти к полю ссылки"},
			{keys: "Esc", label: "Вернуться к истории или отменить загрузку"},
		},
	}
}

// deleteEditorWord implements the Ctrl+Backspace and Ctrl+Delete edits that
// Gio's editor does not bind on Windows. A whitespace run is treated as a
// word of its own, matching the editor's internal word deletion.
func deleteEditorWord(editor *widget.Editor, backward bool) {
	start, end := editor.Selection()
	if start != end {
		editor.Delete(1)
		return
	}
	if start <= 0 && backward {
		return
	}

	text := []rune(editor.Text())
	caret := start
	from, to := caret, caret
	if backward {
		for from > 0 && unicode.IsSpace(text[from-1]) {
			from--
		}
		for from > 0 && !unicode.IsSpace(text[from-1]) {
			from--
		}
	} else {
		for to < len(text) && unicode.IsSpace(text[to]) {
			to++
		}
		for to < len(text) && !unicode.IsSpace(text[to]) {
			to++
		}
	}
	if from == to {
		return
	}
	editor.SetCaret(from, to)
	editor.Delete(1)
}

func (m *mainWindow) focusInput(gtx layout.Context) {
	if m.hintAnim.progress > 0.02 {
		m.hintPulse = gtx.Now
	}
	m.hintForcedHide = true
	m.hintForceAt = gtx.Now
	gtx.Execute(key.FocusCmd{Tag: &m.editor})
	gtx.Execute(op.InvalidateCmd{})
}

// focusHistory moves the keyboard caret out of the URL field. Arrow-key
// selection and Escape both leave text input mode.
func (m *mainWindow) focusHistory(gtx layout.Context) {
	m.hintForcedHide = false
	gtx.Execute(key.FocusCmd{Tag: nil})
	gtx.Execute(op.InvalidateCmd{})
}

func (m *mainWindow) updateHint(gtx layout.Context, focused bool) {
	hidden := focused || m.hintForcedHide
	if focused {
		m.hintForcedHide = false
	} else if m.hintForcedHide && gtx.Now.Sub(m.hintForceAt) > time.Second {
		m.hintForcedHide = false
		hidden = false
	}
	m.hintAnim.update(gtx, !hidden)

	if !m.hintPulse.IsZero() {
		if gtx.Now.Sub(m.hintPulse) < 190*time.Millisecond {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
		} else {
			m.hintPulse = time.Time{}
		}
	}
}

func (m *mainWindow) moveSelection(gtx layout.Context, delta int) {
	if len(m.history) == 0 {
		return
	}
	m.focusHistory(gtx)
	m.selected = nextSelection(m.selected, delta, len(m.history))
	m.ensureSelectionVisible()
	gtx.Execute(op.InvalidateCmd{})
}

// playHistory is shared by the row play control and the P shortcut. Failure
// falls back to the title page, exactly like a mouse click.
func (m *mainWindow) playHistory(gtx layout.Context, index int) {
	if m.loading || m.playing || index < 0 || index >= len(m.history) {
		return
	}
	item := m.history[index]
	m.selected = index
	m.focusHistory(gtx)
	result := historyPlayFailed
	if m.onPlayHistory != nil {
		result = m.onPlayHistory(item)
	}
	switch result {
	case historyPlayStarted:
		m.playing = true
		m.playingID = item.id
	case historyPlayFailed:
		// Fall back to the title page when direct playback is not possible
		// (no voiceovers, missing mpv, and so on).
		m.startLoading(gtx, index, true)
	}
	gtx.Execute(op.InvalidateCmd{})
}

// nextSelection applies an arrow-key step to the current history selection.
// The first Down selects the first title and the first Up selects the last.
func nextSelection(current, delta, count int) int {
	if count <= 0 {
		return -1
	}
	if current < 0 {
		if delta < 0 {
			return count - 1
		}
		return 0
	}
	next := current + delta
	if next < 0 {
		return 0
	}
	if next >= count {
		return count - 1
	}
	return next
}

// reorderIndex maps an item index through a move from -> to. It is used when
// dragging a history row so the selection and loading state follow their title.
func reorderIndex(index, from, to int) int {
	if index == from {
		return to
	}
	if from < to && index > from && index <= to {
		return index - 1
	}
	if to < from && index >= to && index < from {
		return index + 1
	}
	return index
}

func (m *mainWindow) ensureSelectionVisible() {
	if m.selected < 0 {
		return
	}
	p := m.list.Position
	if p.Count <= 0 {
		m.list.ScrollTo(m.selected)
		return
	}
	if m.selected < p.First {
		m.list.ScrollTo(m.selected)
		return
	}
	if m.selected >= p.First+p.Count {
		first := m.selected - p.Count + 1
		if first < 0 {
			first = 0
		}
		m.list.ScrollTo(first)
	}
}

func (m *mainWindow) startLoading(gtx layout.Context, target int, animateRow bool) {
	if m.loading {
		return
	}
	input := strings.TrimSpace(m.editor.Text())
	if target >= 0 && target < len(m.history) {
		input = m.history[target].slug
		m.selected = target
	} else if target >= 0 {
		return
	}
	if input == "" {
		m.triggerEmptyLink(gtx)
		return
	}

	m.loading = true
	m.loadingTarget = target
	m.loadingAnimateRow = animateRow && target >= 0
	m.clearLoadError()
	m.loadSequence++
	sequence := m.loadSequence
	ctx, cancel := context.WithCancel(context.Background())
	m.loadCancel = cancel

	go func() {
		defer cancel()
		if m.library == nil {
			m.deliverLoadResult(titleLoadResult{sequence: sequence, err: context.Canceled})
			return
		}
		// A title typed into the URL field is only fetched; it enters the local
		// history when the user presses Save in the title window. A title
		// opened from history is already saved and is refreshed in place.
		var item library.Title
		var err error
		if target >= 0 {
			item, err = m.library.Load(ctx, input)
		} else {
			item, err = m.library.Fetch(ctx, input)
		}
		result := titleLoadResult{sequence: sequence, err: err}
		if err == nil {
			result.title = titleFromLibrary(item)
			if target >= 0 {
				if history, historyErr := m.library.History(); historyErr == nil {
					result.history = make([]historyTitle, 0, len(history))
					for _, entry := range history {
						result.history = append(result.history, titleFromLibrary(entry))
					}
				}
			}
		}
		m.deliverLoadResult(result)
	}()
	gtx.Execute(op.InvalidateCmd{})
}

func (m *mainWindow) deliverLoadResult(result titleLoadResult) {
	// Never drop a result: a dropped reply leaves m.loading set and the row
	// spinner turning until the application restarts. The UI drains this
	// channel every frame, so a blocking send cannot deadlock in practice.
	m.loadResults <- result
	if m.invalidate != nil {
		m.invalidate()
	}
}

// updateLoading consumes background fetches. There is no artificial delay:
// the title opens as soon as AnimeLIB responds. Stale results from a cancelled
// load are discarded, but draining continues so a current reply queued behind
// one is not left waiting for the next frame.
func (m *mainWindow) updateLoading(gtx layout.Context) {
	// Drain even when idle: a cancelled load still delivers its result, and
	// leaving those in the channel would block their goroutines forever.
	for {
		select {
		case result := <-m.loadResults:
			if !m.loading || result.sequence != m.loadSequence {
				continue
			}
			m.finishLoading(gtx, result)
			return
		default:
			return
		}
	}
}

func (m *mainWindow) finishLoading(gtx layout.Context, result titleLoadResult) {
	m.loading = false
	m.loadingTarget = -1
	m.loadingAnimateRow = false
	m.loadCancel = nil

	if result.err != nil {
		if result.err != context.Canceled && !strings.Contains(result.err.Error(), "context canceled") {
			m.loadError = result.err.Error()
			m.errorAt = gtx.Now
			m.reportError(result.err)
		}
		m.dismissRateLimitNotice()
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	m.clearLoadError()
	// The title came through, so the rate-limit phrase must go immediately
	// even if the throttle pause is technically still winding down.
	animelib.DismissRateLimitNotice()
	if result.history != nil {
		m.allHistory = result.history
		m.history = m.visibleHistory(m.allHistory)
		m.rows = make([]historyRow, len(m.history))
	}
	m.selected = -1
	for index := range m.history {
		if m.history[index].id == result.title.id {
			m.selected = index
			break
		}
	}
	if m.onOpenTitle != nil {
		m.onOpenTitle(result.title)
	}
	gtx.Execute(op.InvalidateCmd{})
}

// requestHistoryRefresh asks the layout loop to reload history from the
// database. It is safe to call from background goroutines.
func (m *mainWindow) requestHistoryRefresh() {
	select {
	case m.refreshRequests <- struct{}{}:
	default:
	}
	if m.invalidate != nil {
		m.invalidate()
	}
}

// updateHistoryRefresh applies a background refresh request on the UI thread.
func (m *mainWindow) updateHistoryRefresh() {
	select {
	case <-m.refreshRequests:
		m.refreshPending = true
	default:
	}
	if !m.refreshPending || m.drag.active {
		return
	}
	m.refreshPending = false
	m.refreshHistory(0)
}

// refreshHistory reloads the visible history after a title is saved from the
// title window and highlights the saved entry. A zero id keeps the current
// selection, which is what background refreshes use.
func (m *mainWindow) refreshHistory(selectedID int64) {
	if m.library == nil {
		return
	}
	history, err := m.library.History()
	if err != nil {
		m.loadError = err.Error()
		m.reportError(err)
		return
	}
	m.allHistory = make([]historyTitle, 0, len(history))
	for _, entry := range history {
		m.allHistory = append(m.allHistory, titleFromLibrary(entry))
	}
	m.history = m.visibleHistory(m.allHistory)
	m.rows = make([]historyRow, len(m.history))
	if selectedID == 0 {
		if m.selected >= len(m.history) {
			m.selected = -1
		}
	} else {
		m.selected = -1
		for index := range m.history {
			if m.history[index].id == selectedID {
				m.selected = index
				break
			}
		}
	}
	m.loadError = ""
}

func (m *mainWindow) cancelLoading(gtx layout.Context) {
	if m.loadCancel != nil {
		m.loadCancel()
		m.loadCancel = nil
	}
	if !m.loading {
		return
	}
	m.loadSequence++
	m.loading = false
	m.loadingTarget = -1
	m.loadingAnimateRow = false
	m.dismissRateLimitNotice()
	gtx.Execute(op.InvalidateCmd{})
}

// dismissRateLimitNotice hides the header phrase when a load ends without a
// request getting through (cancel or final error). A pause that is still
// running keeps the notice, because the throttle is still active.
func (m *mainWindow) dismissRateLimitNotice() {
	if animelib.RateLimitRemaining() == 0 {
		animelib.DismissRateLimitNotice()
	}
}

func (m *mainWindow) clearHistory(gtx layout.Context) {
	if !m.queueClearHistory(gtx.Now) {
		return
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (m *mainWindow) deleteHistoryItem(gtx layout.Context, index int) {
	if !m.queueHistoryDelete(index, gtx.Now) {
		return
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (m *mainWindow) persistHistoryOrder() {
	if m.library == nil {
		return
	}
	m.syncVisibleStatusOrder()
	var err error
	if m.filter == filterAll {
		err = m.library.ReorderHistory(displayTitles(m.allHistory))
	} else if status, ok := m.filter.userStatus(); ok {
		err = m.library.ReorderStatusHistory(status, displayTitles(m.history))
	}
	if err != nil {
		m.loadError = err.Error()
		m.reportError(err)
	}
}

func displayTitles(items []historyTitle) []library.Title {
	out := make([]library.Title, 0, len(items))
	for _, item := range items {
		out = append(out, library.Title{ID: item.id})
	}
	return out
}

func (m *mainWindow) layoutInput(gtx layout.Context, rect image.Rectangle, focused bool) {
	radius := gtx.Dp(mainInputRadius)
	border := color.NRGBA{R: mainInputBorder.R, G: mainInputBorder.G, B: mainInputBorder.B, A: mainInputBorder.A}
	if focused {
		border = mainInputFocusBorder
	}

	fillRounded(gtx, border, rect, radius)
	inner := rect.Inset(gtx.Dp(unit.Dp(1)))
	if inner.Empty() {
		return
	}
	fillRounded(gtx, mainInputColor, inner, radius-gtx.Dp(unit.Dp(1)))

	mascotSize := gtx.Dp(mainMascotSize)
	mascot := image.Rect(
		inner.Min.X+gtx.Dp(unit.Dp(11)),
		inner.Min.Y+(inner.Dy()-mascotSize)/2,
		inner.Min.X+gtx.Dp(unit.Dp(11))+mascotSize,
		inner.Min.Y+(inner.Dy()-mascotSize)/2+mascotSize,
	)

	nextSize := gtx.Dp(mainNextSize)
	nextInset := gtx.Dp(mainNextInset)
	nextRect := image.Rect(
		inner.Max.X-nextInset-nextSize,
		inner.Min.Y+(inner.Dy()-nextSize)/2,
		inner.Max.X-nextInset,
		inner.Min.Y+(inner.Dy()-nextSize)/2+nextSize,
	)

	hintFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, hintInk := m.typo.measure(gtx, "Ctrl + K", hintFace, mainHintTextSize)
	hintWidth := hintInk.Dx() + 2*gtx.Dp(mainHintPaddingX)
	hintHeight := gtx.Dp(mainHintHeight)
	hintRect := image.Rect(
		nextRect.Min.X-gtx.Dp(mainHintGap)-hintWidth,
		inner.Min.Y+(inner.Dy()-hintHeight)/2,
		nextRect.Min.X-gtx.Dp(mainHintGap),
		inner.Min.Y+(inner.Dy()-hintHeight)/2+hintHeight,
	)

	textLeft := mascot.Max.X + gtx.Dp(mainInputGap)
	textRight := hintRect.Min.X - gtx.Dp(mainInputGap)
	lineHeight := gtx.Dp(unit.Dp(20))
	editorRect := image.Rect(
		textLeft,
		inner.Min.Y+(inner.Dy()-lineHeight)/2,
		textRight,
		inner.Min.Y+(inner.Dy()-lineHeight)/2+lineHeight,
	)
	if !editorRect.Empty() {
		m.layoutEditor(gtx, editorRect)
	}
	// Draw the mascot after the editor so the error tremor moves her over the
	// prompt instead of the text painting on top of her.
	shake := errorShake(gtx.Now, m.errorAt)
	offset := op.Offset(image.Pt(errorShakePixels(gtx, shake, mainMascotShakeOffset), 0)).Push(gtx.Ops)
	drawCatgirl(gtx, mascot, gtx.Now, mascotHatNone, errorFeedbackActive(gtx.Now, m.errorAt))
	offset.Pop()

	m.drawHint(gtx, hintRect)
	m.layoutNext(gtx, nextRect)
}

func (m *mainWindow) layoutEditor(gtx layout.Context, rect image.Rectangle) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	if m.editor.Len() == 0 {
		text := m.placeholder.visibleText()
		if text != "" {
			_, ink := m.typo.measure(gtx, text, face, mainInputTextSize)
			if !ink.Empty() {
				m.drawTruncated(
					gtx,
					text,
					face,
					mainInputTextSize,
					mainPlaceholderColor,
					image.Pt(0, (rect.Dy()-ink.Dy())/2),
					rect.Dx(),
				)
			}
		}
	}

	textMaterial := colorMaterial(gtx, mainTextColor)
	selectMaterial := colorMaterial(gtx, mainSelectionColor)
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(rect.Size())
	m.editor.Layout(editorGtx, m.typo.shaper, face, mainInputTextSize, textMaterial, selectMaterial)
}

func (m *mainWindow) drawHint(gtx layout.Context, rect image.Rectangle) {
	progress := m.hintAnim.progress
	if progress <= 0.002 {
		return
	}

	scale := 1.0
	if !m.hintPulse.IsZero() {
		elapsed := gtx.Now.Sub(m.hintPulse)
		if elapsed >= 0 && elapsed < 190*time.Millisecond {
			t := float64(elapsed) / float64(190*time.Millisecond)
			scale += 0.06 * math.Sin(t*math.Pi)
		}
	}

	center := image.Pt((rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2)
	width := int(float64(rect.Dx()) * (0.96 + 0.04*float64(progress)) * scale)
	height := int(float64(rect.Dy()) * (0.96 + 0.04*float64(progress)) * scale)
	chip := image.Rect(
		center.X-width/2,
		center.Y-height/2,
		center.X-width/2+width,
		center.Y-height/2+height,
	)

	fillRounded(gtx, withAlpha(mainHintFillColor, uint8(float64(0xff)*float64(progress))), chip, height/2)
	strokeRoundedRect(gtx, chip, float32(height/2), float32(gtx.Dp(unit.Dp(1))), withAlpha(mainHintBorderColor, uint8(float64(0xff)*float64(progress))))
	m.typo.drawCentered(
		gtx,
		"Ctrl + K",
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		mainHintTextSize,
		withAlpha(mainHintTextColor, uint8(float64(0xff)*float64(progress))),
		center,
	)
}

func (m *mainWindow) layoutNext(gtx layout.Context, rect image.Rectangle) {
	if m.loading {
		m.drawNextFace(gtx, rect, 0, true, false)
		drawSpinner(gtx, centerOf(rect), gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(1.7)), 0, mainTextColor)
		if m.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(mainSpinnerFrameTime)})
		}
		return
	}

	_, clicked := m.next.Update(gtx)
	hovered := m.next.Hovered()
	hover := m.nextHover.update(gtx, hovered)
	if clicked {
		if strings.TrimSpace(m.editor.Text()) == "" {
			m.triggerEmptyLink(gtx)
		} else {
			m.startLoading(gtx, -1, false)
		}
	}

	errored := errorFeedbackActive(gtx.Now, m.errorAt)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	m.next.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		m.drawNextFace(gtx, image.Rectangle{Max: rect.Size()}, hover, false, errored)
		return layout.Dimensions{Size: rect.Size()}
	})
}

func (m *mainWindow) drawNextFace(gtx layout.Context, rect image.Rectangle, hover float32, disabled, alarm bool) {
	if rect.Empty() {
		return
	}
	radius := rect.Dy() / 2
	c1, c2 := mainAccentColor1, mainAccentColor2
	if disabled {
		c1, c2 = mainDisabledColor1, mainDisabledColor2
	} else if alarm {
		c1, c2 = mainDangerColor, mainDangerColor2
	} else if hover > 0 {
		c1 = mixColor(c1, color.NRGBA{R: 0xae, G: 0x8b, B: 0xff, A: 0xff}, hover)
		c2 = mixColor(c2, color.NRGBA{R: 0x82, G: 0x3e, B: 0xee, A: 0xff}, hover)
	}
	fillRoundedGradient(gtx, rect, rect, radius, c1, c2)
	if !disabled {
		iconCenter := centerOf(rect)
		if alarm {
			// Only the arrow trembles; the red circle itself stays in place.
			shake := errorShake(gtx.Now, m.errorAt)
			iconCenter.X += errorShakePixels(gtx, shake, mainIconShakeOffset)
		}
		drawChevronRight(gtx, iconCenter, rect.Dx()/3, gtx.Dp(unit.Dp(2)), color.NRGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff})
	}
}

func (m *mainWindow) layoutHistoryHeader(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}

	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := m.typo.measure(gtx, "История", titleFace, mainHistoryTitleSize)
	titleTop := rect.Min.Y + (rect.Dy()-titleInk.Dy())/2
	m.typo.draw(gtx, "История", titleFace, mainHistoryTitleSize, mainTextColor, image.Pt(rect.Min.X, titleTop))

	countFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	count := fmt.Sprintf("(%d)", len(m.history))
	_, countInk := m.typo.measure(gtx, count, countFace, mainHistoryCountSize)
	countTop := rect.Min.Y + (rect.Dy()-countInk.Dy())/2
	m.typo.draw(
		gtx,
		count,
		countFace,
		mainHistoryCountSize,
		mainMutedColor,
		image.Pt(rect.Min.X+titleInk.Dx()+gtx.Dp(unit.Dp(7)), countTop),
	)

	clearFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	clearLabel := "Очистить всё"
	if m.undo.active {
		clearLabel = "Отменить"
	}
	_, clearInk := m.typo.measure(gtx, clearLabel, clearFace, mainClearSize)
	iconSize := gtx.Dp(unit.Dp(14))
	clearGap := gtx.Dp(unit.Dp(6))
	clearWidth := clearInk.Dx() + clearGap + iconSize
	clearHeight := rect.Dy()
	clearRect := image.Rect(rect.Max.X-clearWidth, rect.Min.Y, rect.Max.X, rect.Min.Y+clearHeight)
	if clearRect.Empty() {
		return
	}

	middleLeft := rect.Min.X + titleInk.Dx() + gtx.Dp(unit.Dp(7)) + countInk.Dx() + gtx.Dp(unit.Dp(14))
	middleRight := clearRect.Min.X - gtx.Dp(unit.Dp(12))
	if m.undo.active {
		// The countdown sits in the free space between the history title and
		// the action button, matching the rate-limit notice in the header.
		m.layoutUndoCountdown(gtx, middleLeft, middleRight, rect.Min.Y+rect.Dy()/2)
	} else {
		filterLeft, filterRight := historyFilterSpan(gtx, rect, titleInk, countInk, clearRect)
		m.layoutHistoryFilters(gtx, filterLeft, filterRight, rect.Min.Y+rect.Dy()/2)
	}

	_, clicked := m.clear.Update(gtx)
	hovered := m.clear.Hovered()
	hover := m.clearHover.update(gtx, hovered)
	if clicked {
		gtx.Execute(key.FocusCmd{Tag: nil})
		if m.undo.active {
			m.undoHistoryDelete()
			gtx.Execute(op.InvalidateCmd{})
		} else {
			m.clearHistory(gtx)
		}
	}
	labelColor := mixColor(mainMutedColor, mainDangerColor, hover)
	if m.undo.active {
		labelColor = mixColor(undoActionColor, undoActionHoverColor, hover)
	}
	labelX := clearRect.Min.X
	labelY := clearRect.Min.Y + (clearRect.Dy()-clearInk.Dy())/2
	m.typo.draw(gtx, clearLabel, clearFace, mainClearSize, labelColor, image.Pt(labelX, labelY))
	iconCenter := image.Pt(clearRect.Max.X-iconSize/2, clearRect.Min.Y+clearRect.Dy()/2)
	if m.undo.active {
		drawUndoArrow(gtx, iconCenter, iconSize, labelColor)
	} else {
		drawTrash(gtx, iconCenter, iconSize, labelColor)
	}

	stack := op.Offset(clearRect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(clearRect.Size())
	m.clear.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: clearRect.Size()}
	})
}

type filterRowMetrics struct {
	widths  []int
	gap     int
	size    unit.Sp
	padding unit.Dp
	total   int
}

type filterPillStep struct {
	size    unit.Sp
	padding unit.Dp
	gap     unit.Dp
}

// filterPillSteps are tried from the designed metrics to a compact version.
// The last step is used even if the window is smaller still, so the filters do
// not disappear completely on an unexpectedly narrow viewport.
var filterPillSteps = [...]filterPillStep{
	{size: mainFilterSize, padding: mainFilterPaddingX, gap: mainFilterGap},
	{size: 10, padding: unit.Dp(7), gap: unit.Dp(4)},
	{size: 9, padding: unit.Dp(5), gap: unit.Dp(3)},
	{size: 8, padding: unit.Dp(3), gap: unit.Dp(2)},
}

// measureFilterRow picks the largest pill metrics that fit in available. On a
// wide window the gaps grow a little, up to mainFilterMaxGap, so the row still
// follows the window, but the pills stay a compact cluster.
func measureFilterRow(gtx layout.Context, typo *typography, available int) filterRowMetrics {
	measure := func(step filterPillStep) filterRowMetrics {
		metrics := filterRowMetrics{
			widths:  make([]int, len(userStatusFilterOptions)),
			size:    step.size,
			padding: step.padding,
			gap:     gtx.Dp(step.gap),
		}
		for index, option := range userStatusFilterOptions {
			metrics.widths[index] = filterPillWidth(gtx, typo, option.label, step.size, step.padding)
			metrics.total += metrics.widths[index]
		}
		metrics.total += metrics.gap * (len(metrics.widths) - 1)
		return metrics
	}

	metrics := measure(filterPillSteps[0])
	if metrics.total > available {
		for _, step := range filterPillSteps[1:] {
			metrics = measure(step)
			if metrics.total <= available {
				break
			}
		}
		return metrics
	}

	maxGap := gtx.Dp(mainFilterMaxGap)
	if metrics.gap < maxGap && len(metrics.widths) > 1 {
		grow := (available - metrics.total) / (len(metrics.widths) - 1)
		if grow > maxGap-metrics.gap {
			grow = maxGap - metrics.gap
		}
		if grow > 0 {
			metrics.gap += grow
			metrics.total += grow * (len(metrics.widths) - 1)
		}
	}
	return metrics
}

// historyFilterSpan is the stable free area used by the filter row. The count
// reserve, rather than the current "(n)" width, keeps the row from moving when
// titles are added or removed. The right edge follows the clear action, so the
// span still adapts to window resizes.
func historyFilterSpan(gtx layout.Context, rect, titleInk, countInk, clearRect image.Rectangle) (int, int) {
	countReserve := max(gtx.Dp(mainFilterReserve), countInk.Dx())
	left := rect.Min.X + titleInk.Dx() + gtx.Dp(unit.Dp(7)) + countReserve + gtx.Dp(unit.Dp(14))
	right := clearRect.Min.X - gtx.Dp(unit.Dp(12))
	return left, right
}

// centeredFilterRowStart keeps a row inside its available span. A row wider
// than the span starts at the left edge; at that point the header clips it
// rather than dropping the controls.
func centeredFilterRowStart(left, available, total int) int {
	if total >= available {
		return left
	}
	return left + (available-total)/2
}

// layoutHistoryFilters draws the six status filters centred in the stable
// middle span. Inactive pills are neutral gray; selecting one reveals its
// status color.
func (m *mainWindow) layoutHistoryFilters(gtx layout.Context, left, right, centerY int) {
	available := right - left
	if available <= 0 {
		return
	}
	metrics := measureFilterRow(gtx, m.typo, available)
	x := centeredFilterRowStart(left, available, metrics.total)
	height := gtx.Dp(mainFilterHeight)
	clickedIndex := -1
	for index, option := range userStatusFilterOptions {
		rect := image.Rect(x, centerY-height/2, x+metrics.widths[index], centerY+height/2)
		click := &m.filterClicks[index]
		hover := m.filterHovers[index].update(gtx, click.Hovered())
		if click.Clicked(gtx) {
			clickedIndex = index
		}
		m.layoutHitArea(gtx, rect, click, true, func(gtx layout.Context) {
			drawFilterPill(gtx, m.typo, image.Rect(0, 0, rect.Dx(), rect.Dy()), option, m.filter == option.filter, hover, metrics.size)
		})
		x += metrics.widths[index] + metrics.gap
	}
	if clickedIndex >= 0 {
		m.setFilter(gtx, userStatusFilterOptions[clickedIndex].filter)
	}
}

// setFilter applies a history filter from either a mouse click or a keyboard
// shortcut. It is a no-op when the filter is already selected.
func (m *mainWindow) setFilter(gtx layout.Context, next userStatusFilter) {
	if next == m.filter {
		return
	}
	m.filter = next
	m.applyFilter()
	gtx.Execute(op.InvalidateCmd{})
}

// stepFilter moves through the filter pills in their visual order and wraps
// around, so the left and right arrows always have a visible result.
func (m *mainWindow) stepFilter(gtx layout.Context, delta int) {
	count := len(userStatusFilterOptions)
	if count == 0 {
		return
	}
	current := 0
	for index, option := range userStatusFilterOptions {
		if option.filter == m.filter {
			current = index
			break
		}
	}
	delta %= count
	next := (current + delta + count) % count
	m.setFilter(gtx, userStatusFilterOptions[next].filter)
}

func (m *mainWindow) layoutHistory(gtx layout.Context, rect image.Rectangle) {
	m.list.Gap = gtx.Dp(mainListGap)
	rowHeight := gtx.Dp(mainRowHeight)
	gap := m.list.Gap
	m.listRect = rect
	m.listRowHeight = rowHeight
	m.listGap = gap
	m.handleHistoryDrag(gtx, rect, rowHeight, gap)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	// One synthetic tail row lets the last real title scroll up enough to leave
	// a little breathing room above the footer.
	listCount := len(m.history)
	tailHeight := gtx.Dp(mainHistoryTailSpace)
	m.list.Layout(lgtx, listCount+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == listCount {
			return layout.Dimensions{Size: image.Pt(rect.Dx(), tailHeight)}
		}
		m.layoutHistoryRow(gtx, index, rect.Dx(), rowHeight)
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	// The row click is handled after the list has finished laying out: removing
	// an item while List is iterating would leave its cached count stale for the
	// rest of this frame.
	if m.deleteRequested {
		m.deleteRequested = false
		m.deleteHistoryItem(gtx, m.deleteIndex)
	}
	// The strip is added after the list so it sits above row content and only
	// captures presses in the six-dot handle column.
	m.addHistoryDragInput(gtx, rect)
}

type historyRowGeometry struct {
	handle        image.Rectangle
	poster        image.Rectangle
	play          image.Rectangle
	trash         image.Rectangle
	releaseStatus image.Rectangle
	status        image.Rectangle
	titleHit      image.Rectangle
	titleX        int
}

type historyRowVisual struct {
	selected    bool
	dragging    bool
	hover       float32
	playHover   float32
	trashHover  float32
	loading     bool
	itemLoading bool
}

func (m *mainWindow) historyRowGeometry(gtx layout.Context, width, height int, item historyTitle) historyRowGeometry {
	posterWidth := gtx.Dp(mainPosterWidth)
	posterHeight := gtx.Dp(mainPosterHeight)
	controlSize := gtx.Dp(mainControlSize)
	controlGap := gtx.Dp(mainControlGap)
	pad := gtx.Dp(mainRowPadding)
	handleWidth := gtx.Dp(mainDragHandleWidth)
	handle := image.Rect(pad, 0, pad+handleWidth, height)
	poster := image.Rect(
		handle.Max.X+gtx.Dp(unit.Dp(6)),
		(height-posterHeight)/2,
		handle.Max.X+gtx.Dp(unit.Dp(6))+posterWidth,
		(height-posterHeight)/2+posterHeight,
	)
	trash := image.Rect(width-pad-controlSize, (height-controlSize)/2, width-pad, (height-controlSize)/2+controlSize)
	play := image.Rect(trash.Min.X-controlGap-controlSize, trash.Min.Y, trash.Min.X-controlGap, trash.Max.Y)
	titleX := poster.Max.X + gtx.Dp(unit.Dp(14))
	statusWidth := m.userStatusWidth(gtx)
	statusHeight := gtx.Dp(mainStatusHeight)
	statusGap := gtx.Dp(mainStatusGap)
	status := image.Rect(
		play.Min.X-gtx.Dp(unit.Dp(14))-statusWidth,
		(height-statusHeight)/2,
		play.Min.X-gtx.Dp(unit.Dp(14)),
		(height-statusHeight)/2+statusHeight,
	)
	releaseStatus := image.Rect(
		status.Min.X-statusGap-statusWidth,
		status.Min.Y,
		status.Min.X-statusGap,
		status.Max.Y,
	)
	titleHit := image.Rect(handle.Max.X, 0, releaseStatus.Min.X-gtx.Dp(unit.Dp(8)), height)
	if titleHit.Max.X < titleHit.Min.X+1 {
		titleHit.Max.X = titleHit.Min.X + 1
	}
	return historyRowGeometry{
		handle:        handle,
		poster:        poster,
		play:          play,
		trash:         trash,
		releaseStatus: releaseStatus,
		status:        status,
		titleHit:      titleHit,
		titleX:        titleX,
	}
}

// drawHistoryRowCard paints a card at rect. The geometry rectangles are local
// to the card, so the same code can draw both a list row and the floating drag
// overlay.
func (m *mainWindow) drawHistoryRowCard(gtx layout.Context, rect image.Rectangle, item historyTitle, geom historyRowGeometry, vis historyRowVisual) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	rowRect := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(mainRowRadius)
	background := mainRowColor
	if vis.selected {
		background = mainRowSelectedColor
	}
	if vis.hover > 0 {
		background = mixColor(background, mainRowHoverColor, vis.hover)
	}
	fillRounded(gtx, background, rowRect, radius)

	border := mainRowBorderColor
	if vis.hover > 0 {
		border = mixColor(border, mainRowHoverBorder, vis.hover)
	}
	if vis.selected || vis.dragging {
		border = mainAccentColor1
	}
	strokeRoundedRect(
		gtx,
		rowRect.Inset(gtx.Dp(unit.Dp(1))),
		float32(radius),
		float32(gtx.Dp(unit.Dp(1))),
		border,
	)

	handleColor := mainDragDotColor
	if vis.selected || vis.dragging {
		handleColor = mainDragDotActive
	}
	drawDragHandle(gtx, geom.handle, handleColor)

	m.drawPoster(gtx, geom.poster, item)

	titleRight := geom.releaseStatus.Min.X - gtx.Dp(unit.Dp(14))
	if titleRight < geom.titleX+1 {
		titleRight = geom.titleX + 1
	}
	titleFace := font.Font{Typeface: fontFamily, Weight: font.SemiBold}
	_, titleInk := m.typo.measure(gtx, item.title, titleFace, mainRowTitleSize)
	label := item.progressLabel()
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	var labelInk image.Rectangle
	if label != "" {
		_, labelInk = m.typo.measure(gtx, label, labelFace, episodeProgressLabelSz)
	}
	// A film has no episode counter, but its watch progress bar still belongs
	// in the row. Series without a known episode keep the old title-only layout.
	showProgress := item.movie || label != ""
	if !showProgress {
		m.drawTruncated(
			gtx,
			item.title,
			titleFace,
			mainRowTitleSize,
			mainTextColor,
			image.Pt(geom.titleX, (rowRect.Dy()-titleInk.Dy())/2),
			titleRight-geom.titleX,
		)
	} else {
		barHeight := gtx.Dp(episodeProgressHeight)
		progressGap := gtx.Dp(historyProgressGap)
		progressHeight := barHeight
		if labelInk.Dy() > progressHeight {
			progressHeight = labelInk.Dy()
		}
		blockTop := (rowRect.Dy() - (titleInk.Dy() + progressGap + progressHeight)) / 2
		m.drawTruncated(
			gtx,
			item.title,
			titleFace,
			mainRowTitleSize,
			mainTextColor,
			image.Pt(geom.titleX, blockTop),
			titleRight-geom.titleX,
		)

		barTop := blockTop + titleInk.Dy() + progressGap + (progressHeight-barHeight)/2
		labelGap := gtx.Dp(episodeProgressGap)
		barWidth := rowRect.Dx() * 28 / 100
		maxBar := titleRight - geom.titleX
		if labelInk.Dx() > 0 {
			maxBar -= labelGap + labelInk.Dx()
		}
		if barWidth > maxBar {
			barWidth = maxBar
		}
		if barWidth >= gtx.Dp(unit.Dp(56)) {
			bar := image.Rect(geom.titleX, barTop, geom.titleX+barWidth, barTop+barHeight)
			drawEpisodeProgressBar(gtx, bar, episodeProgressValue(item.resumeAt, item.duration))
			if label != "" {
				m.typo.draw(
					gtx,
					label,
					labelFace,
					episodeProgressLabelSz,
					episodeProgressLabelCol,
					image.Pt(bar.Max.X+labelGap, barTop+(barHeight-labelInk.Dy())/2),
				)
			}
		} else if label != "" {
			m.typo.draw(
				gtx,
				label,
				labelFace,
				episodeProgressLabelSz,
				episodeProgressLabelCol,
				image.Pt(titleRight-labelInk.Dx(), barTop+(barHeight-labelInk.Dy())/2),
			)
		}
	}
	drawStatusPill(gtx, m.typo, geom.releaseStatus, item.status)
	drawUserStatusPill(gtx, m.typo, geom.status, item.userStatus, true)

	if vis.itemLoading {
		drawControlButton(gtx, geom.play, mainControlFill, mainAccentColor1)
		drawSpinner(gtx, centerOf(geom.play), gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(1.5)), 0, mainAccentColor1)
	} else {
		buttonTextColor := mainControlColor
		if vis.loading {
			buttonTextColor = withAlpha(mainFaintColor, 0xb0)
		}
		fill := mixColor(mainControlFill, mainPlayButtonFill, vis.playHover)
		stroke := mixColor(mainControlBorder, mainPlayButtonBorder, vis.playHover)
		drawControlButton(gtx, geom.play, fill, stroke)
		drawPlay(gtx, centerOf(geom.play), gtx.Dp(unit.Dp(11)), buttonTextColor)
	}

	trashFill := mixColor(mainControlFill, mainTrashButtonFill, vis.trashHover)
	trashBorder := mixColor(mainControlBorder, mainTrashButtonBorder, vis.trashHover)
	drawControlButton(gtx, geom.trash, trashFill, trashBorder)
	drawTrash(gtx, centerOf(geom.trash), gtx.Dp(unit.Dp(14)), mainControlColor)
}

func (m *mainWindow) layoutHistoryRow(gtx layout.Context, index, width, height int) {
	if index < 0 || index >= len(m.history) || index >= len(m.rows) {
		return
	}
	row := &m.rows[index]
	item := m.history[index]
	geom := m.historyRowGeometry(gtx, width, height, item)

	_, titleClicked := row.titleClick.Update(gtx)
	if titleClicked && !m.drag.active {
		if m.selected == index {
			// The first press only highlights a title. Pressing the already
			// highlighted card opens it, so no double click is needed.
			m.startLoading(gtx, index, false)
		} else {
			m.selected = index
			m.focusHistory(gtx)
		}
	}

	playClicked := false
	if !m.loading && !m.playing {
		_, playClicked = row.playClick.Update(gtx)
		if playClicked && !m.drag.active {
			m.playHistory(gtx, index)
		}
	}
	_, trashClicked := row.trashClick.Update(gtx)
	if trashClicked && !m.drag.active {
		m.deleteRequested = true
		m.deleteIndex = index
	}
	statusClickable := item.status != statusAnnounced
	if statusClickable && row.statusClick.Clicked(gtx) && !m.drag.active {
		stride := m.listRowHeight + m.listGap
		top := m.listRect.Min.Y + m.historyRowTop(index, stride)
		anchor := geom.status.Add(image.Pt(m.listRect.Min.X, top))
		m.statusMenu.show(anchor, item.userStatus, item.id, geom.status.Dx(), geom.status.Dy())
		m.selected = index
		gtx.Execute(op.InvalidateCmd{})
	}

	titleHovered := row.titleClick.Hovered()
	playHovered := !m.loading && !m.playing && row.playClick.Hovered()
	trashHovered := row.trashClick.Hovered()
	statusHovered := statusClickable && row.statusClick.Hovered()
	rowHover := row.hover.update(gtx, titleHovered || playHovered || trashHovered || statusHovered)
	playHover := row.playHover.update(gtx, playHovered)
	trashHover := row.trashHover.update(gtx, trashHovered)

	dragging := m.drag.active && m.drag.index == index
	if dragging {
		// The dragged card is drawn by the window-level overlay. Its slot shows
		// a quiet drop target while the other rows reflow around it.
		rowRect := image.Rect(0, 0, width, height)
		radius := gtx.Dp(mainRowRadius)
		fillRounded(gtx, withAlpha(mainAccentColor1, 0x10), rowRect, radius)
		strokeRoundedRect(
			gtx,
			rowRect.Inset(gtx.Dp(unit.Dp(1))),
			float32(radius),
			float32(gtx.Dp(unit.Dp(1))),
			withAlpha(mainAccentColor1, 0x66),
		)
		return
	}

	loadingItem := m.loading && m.loadingTarget == index && m.loadingAnimateRow
	playingItem := m.playing && m.playingID == item.id
	itemLoading := loadingItem || playingItem
	m.drawHistoryRowCard(gtx, image.Rect(0, 0, width, height), item, geom, historyRowVisual{
		selected:    m.selected == index,
		hover:       rowHover,
		playHover:   playHover,
		trashHover:  trashHover,
		loading:     m.loading || m.playing,
		itemLoading: itemLoading,
	})

	if !m.loading && !m.playing {
		m.layoutHitArea(gtx, geom.play, &row.playClick, true, nil)
	}
	m.layoutHitArea(gtx, geom.trash, &row.trashClick, true, nil)
	if statusClickable {
		m.layoutHitArea(gtx, geom.status, &row.statusClick, true, nil)
	}
	m.layoutHitArea(gtx, geom.titleHit, &row.titleClick, false, nil)

	if itemLoading && m.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(mainSpinnerFrameTime)})
	}
}

func drawControlButton(gtx layout.Context, rect image.Rectangle, fill, border color.NRGBA) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(mainControlRadius)
	fillRounded(gtx, fill, rect, radius)
	strokeRoundedRect(
		gtx,
		rect.Inset(gtx.Dp(unit.Dp(1))),
		float32(radius),
		float32(gtx.Dp(unit.Dp(1))),
		border,
	)
}

func drawDragHandle(gtx layout.Context, rect image.Rectangle, col color.NRGBA) {
	if rect.Empty() {
		return
	}
	dotRadius := max(1, gtx.Dp(unit.Dp(1.25)))
	gapX := gtx.Dp(unit.Dp(3.5))
	gapY := gtx.Dp(unit.Dp(4.5))
	cx := rect.Min.X + rect.Dx()/2
	cy := rect.Min.Y + rect.Dy()/2
	for row := -1; row <= 1; row++ {
		for column := -1; column <= 1; column += 2 {
			fillCircle(gtx, image.Pt(cx+column*gapX/2, cy+row*gapY), dotRadius, col)
		}
	}
}

func (m *mainWindow) userStatusWidth(gtx layout.Context) int {
	return userStatusPillWidth(gtx, m.typo)
}

func (m *mainWindow) historyItemByID(id int64) historyTitle {
	if index := historyIndexByID(m.allHistory, id); index >= 0 {
		return m.allHistory[index]
	}
	if index := historyIndexByID(m.history, id); index >= 0 {
		return m.history[index]
	}
	return historyTitle{}
}

// dismissPopup closes the status menu when the window loses focus.
func (m *mainWindow) dismissPopup() {
	m.statusMenu.close()
}

// setUserStatus updates a title in both the complete and visible history. A
// changed status also moves the title to the top of its new status list. When a
// filter is active the row may no longer belong in the visible list, so the
// filtered view is rebuilt instead of patched in place.
func (m *mainWindow) setUserStatus(id int64, status library.UserStatus) {
	if id == 0 {
		return
	}
	m.ensureAllHistory()
	target := library.NormalizeUserStatus(status)
	previous, found := library.UserStatus(0), false
	for index := range m.allHistory {
		if m.allHistory[index].id == id {
			previous = library.NormalizeUserStatus(m.allHistory[index].userStatus)
			found = true
			break
		}
	}
	if !found || previous == target {
		return
	}

	statusOrder := int64(0)
	hasOrder := false
	for index := range m.allHistory {
		item := &m.allHistory[index]
		if item.id == id || library.NormalizeUserStatus(item.userStatus) != target {
			continue
		}
		if !hasOrder || item.statusOrder < statusOrder {
			statusOrder = item.statusOrder
			hasOrder = true
		}
	}
	if hasOrder {
		statusOrder -= 1000
	}

	for index := range m.allHistory {
		if m.allHistory[index].id != id {
			continue
		}
		m.allHistory[index].userStatus = target
		m.allHistory[index].statusOrder = statusOrder
	}
	for index := range m.history {
		if m.history[index].id != id {
			continue
		}
		m.history[index].userStatus = target
		m.history[index].statusOrder = statusOrder
	}
	if m.filter != filterAll {
		m.applyFilter()
	}
}

func statusPillWidth(gtx layout.Context, typo *typography) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	// Release and user statuses share one width so both pills in a row look
	// like a single, stable column even when their labels differ.
	width := 0
	for _, label := range []string{"Завершён", "Онгоинг", "Анонс"} {
		if _, ink := typo.measure(gtx, label, face, mainStatusSize); ink.Dx() > width {
			width = ink.Dx()
		}
	}
	for _, status := range userStatusMenuOptions {
		if _, ink := typo.measure(gtx, userStatusLabel(status), face, mainStatusSize); ink.Dx() > width {
			width = ink.Dx()
		}
	}
	return 2*gtx.Dp(mainStatusPaddingX) + width
}

func (m *mainWindow) drawStatus(gtx layout.Context, rect image.Rectangle, status titleStatus) {
	drawStatusPill(gtx, m.typo, rect, status)
}

func drawStatusPill(gtx layout.Context, typo *typography, rect image.Rectangle, status titleStatus) {
	if rect.Empty() {
		return
	}
	label := "Завершён"
	textColor := mainStatusDoneText
	fillColor := mainStatusDoneFill
	borderColor := mainStatusDoneBorder
	switch status {
	case statusOngoing:
		label = "Онгоинг"
		textColor = mainStatusLiveText
		fillColor = mainStatusLiveFill
		borderColor = mainStatusLiveBorder
	case statusAnnounced:
		label = "Анонс"
		textColor = mainStatusAnnText
		fillColor = mainStatusAnnFill
		borderColor = mainStatusAnnBorder
	}
	drawPill(gtx, typo, rect, label, mainStatusSize, textColor, fillColor, borderColor)
}

func (m *mainWindow) drawTruncated(gtx layout.Context, text string, face font.Font, size unit.Sp, col color.NRGBA, at image.Point, maxWidth int) {
	drawTruncatedText(gtx, m.typo, text, face, size, col, at, maxWidth)
}

func drawTruncatedText(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp, col color.NRGBA, at image.Point, maxWidth int) {
	if maxWidth <= 0 {
		return
	}
	text = truncateText(gtx, typo, text, face, size, maxWidth)
	if text == "" {
		return
	}
	typo.draw(gtx, text, face, size, col, at)
}

// truncateText shortens text with a trailing ellipsis until it fits maxWidth.
func truncateText(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if _, ink := typo.measure(gtx, text, face, size); ink.Dx() <= maxWidth {
		return text
	}

	return truncateAtWord(text, func(candidate string) bool {
		_, ink := typo.measure(gtx, candidate, face, size)
		return ink.Dx() <= maxWidth
	})
}

// truncateAtWord cuts text at the last whole word that fits and appends an
// ellipsis. Punctuation and spaces before the cut are dropped, so a title never
// ends with a dangling colon or comma. A single word wider than the available
// space is cut by rune as a last resort.
func truncateAtWord(text string, fits func(string) bool) string {
	const ellipsis = "…"
	if fits(text) {
		return text
	}

	runes := []rune(text)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		if fits(string(runes[:mid]) + ellipsis) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low <= 0 {
		return ""
	}

	cut := low
	if cut < len(runes) && !isTextBoundary(runes[cut]) {
		for cut > 0 && !isTextBoundary(runes[cut-1]) {
			cut--
		}
	}
	// A word boundary can itself be a separator (a space or punctuation). Drop
	// it before the ellipsis instead of keeping "Тайтл:…" or "Тайтл …".
	for cut > 0 && isTextSeparator(runes[cut-1]) {
		cut--
	}
	if cut == 0 {
		// The first word is longer than the available width. Falling back to the
		// measured rune cut keeps the row useful instead of empty.
		cut = low
		for cut > 0 && isTextSeparator(runes[cut-1]) {
			cut--
		}
	}
	if cut <= 0 {
		return ""
	}
	return string(runes[:cut]) + ellipsis
}

func isTextBoundary(r rune) bool {
	return unicode.IsSpace(r) || isTextSeparator(r)
}

func isTextSeparator(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case ',', '.', ';', ':', '!', '?', '—', '–', '-', '(', ')', '[', ']', '{', '}',
		'«', '»', '"', '\'', '…':
		return true
	}
	return false
}

func (m *mainWindow) layoutHitArea(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, pointerCursor bool, draw func(layout.Context)) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		if pointerCursor {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		if draw != nil {
			draw(gtx)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
}

func (m *mainWindow) layoutClickableDraw(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context)) {
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

func drawIn(gtx layout.Context, rect image.Rectangle, draw func(layout.Context)) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	draw(gtx)
}

func (m *mainWindow) drawPoster(gtx layout.Context, rect image.Rectangle, item historyTitle) {
	drawPosterArt(gtx, m.typo, rect, item, mainPosterTitleSize)
}

func drawGeneratedPoster(gtx layout.Context, typo *typography, rect image.Rectangle, item historyTitle, initialSize unit.Sp) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(unit.Dp(6))
	fillRoundedGradient(gtx, rect, rect, radius, item.poster1, item.poster2)

	// A soft circle and diagonal band keep the generated posters from reading
	// as flat color swatches.
	circle := image.Rect(rect.Min.X+rect.Dx()/3, rect.Min.Y-gtx.Dp(3), rect.Max.X+gtx.Dp(5), rect.Min.Y+rect.Dy()/2)
	fillRounded(gtx, withAlpha(color.NRGBA{R: 255, G: 255, B: 255, A: 0xff}, 0x16), circle, circle.Dy()/2)
	strokeRoundedRect(gtx, rect, float32(radius), float32(gtx.Dp(unit.Dp(1))), color.NRGBA{R: 255, G: 255, B: 255, A: 0x18})

	typo.drawCentered(
		gtx,
		item.initial,
		font.Font{Typeface: fontFamily, Weight: font.Bold},
		initialSize,
		color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x66},
		image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2),
	)
}

func drawSpinner(gtx layout.Context, center image.Point, radius, dotRadius int, phase float64, col color.NRGBA) {
	if radius <= 0 || dotRadius <= 0 {
		return
	}
	phase += float64(gtx.Now.UnixNano()%int64(2*time.Second)) / float64(2*time.Second) * 2 * math.Pi
	for i := 0; i < 8; i++ {
		angle := phase + float64(i)*math.Pi/4
		p := image.Pt(
			center.X+int(math.Cos(angle)*float64(radius)),
			center.Y+int(math.Sin(angle)*float64(radius)),
		)
		c := col
		c.A = uint8(35 + int(float64(190)*float64(i)/7))
		fillCircle(gtx, p, dotRadius, c)
	}
}

func drawPlay(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	if size <= 0 {
		return
	}
	half := size / 2
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(center.X-half/2), float32(center.Y-half)))
	path.LineTo(f32.Pt(float32(center.X+half), float32(center.Y)))
	path.LineTo(f32.Pt(float32(center.X-half/2), float32(center.Y+half)))
	path.Close()
	stack := clip.Outline{Path: path.End()}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
}

func drawTrash(gtx layout.Context, center image.Point, size int, col color.NRGBA) {
	if size <= 0 {
		return
	}
	half := float32(size) / 2
	width := float32(max(1, size/9))
	// Lid and handle.
	strokeLine(gtx, f32.Pt(float32(center.X)-half*0.8, float32(center.Y)-half*0.45), f32.Pt(float32(center.X)+half*0.8, float32(center.Y)-half*0.45), width, col)
	strokeLine(gtx, f32.Pt(float32(center.X)-half*0.28, float32(center.Y)-half*0.72), f32.Pt(float32(center.X)+half*0.28, float32(center.Y)-half*0.72), width, col)
	// Slightly tapered can.
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(center.X)-half*0.62, float32(center.Y)-half*0.25))
	path.LineTo(f32.Pt(float32(center.X)-half*0.47, float32(center.Y)+half*0.72))
	path.LineTo(f32.Pt(float32(center.X)+half*0.47, float32(center.Y)+half*0.72))
	path.LineTo(f32.Pt(float32(center.X)+half*0.62, float32(center.Y)-half*0.25))
	stack := clip.Stroke{Path: path.End(), Width: width}.Op().Push(gtx.Ops)
	paint.Fill(gtx.Ops, col)
	stack.Pop()
	// Two slits.
	for _, dx := range []float32{-0.2, 0.2} {
		strokeLine(gtx,
			f32.Pt(float32(center.X)+half*dx, float32(center.Y)-half*0.02),
			f32.Pt(float32(center.X)+half*dx, float32(center.Y)+half*0.5),
			width,
			col,
		)
	}
}

func drawChevronRight(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	if size <= 0 {
		return
	}
	half := float32(size) / 2
	strokeLine(gtx,
		f32.Pt(float32(center.X)-half*0.35, float32(center.Y)-half*0.8),
		f32.Pt(float32(center.X)+half*0.55, float32(center.Y)),
		float32(width),
		col,
	)
	strokeLine(gtx,
		f32.Pt(float32(center.X)+half*0.55, float32(center.Y)),
		f32.Pt(float32(center.X)-half*0.35, float32(center.Y)+half*0.8),
		float32(width),
		col,
	)
}

func fillCircle(gtx layout.Context, center image.Point, radius int, col color.NRGBA) {
	if radius <= 0 {
		return
	}
	fillRounded(gtx, col, image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius), radius)
}

func centerOf(rect image.Rectangle) image.Point {
	return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
}

func colorMaterial(gtx layout.Context, col color.NRGBA) op.CallOp {
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	return macro.Stop()
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func mixColor(a, b color.NRGBA, t float32) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	lerp := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: lerp(a.A, b.A)}
}
