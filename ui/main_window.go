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

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yoanime/animelib"
	"github.com/arfush/yoanime/catalog"
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
	mainErrorFeedbackDuration = 5 * time.Second
	// mainIconShakeOffset is the peak horizontal offset of the Next arrow. The
	// arrow is only about 13 dp wide, so a large offset reads as jumping.
	mainIconShakeOffset = unit.Dp(0.7)
	// mainMascotShakeOffset is deliberately smaller: at 42 dp the character
	// reads any larger movement as jumping, not trembling.
	mainMascotShakeOffset = unit.Dp(0.9)
	mainErrorShakeHz      = 11.0
)

var (
	mainInputColor       = color.NRGBA{R: 0x12, G: 0x11, B: 0x1a, A: 0xff}
	mainInputBorder      = color.NRGBA{R: 0x2a, G: 0x25, B: 0x38, A: 0xff}
	mainInputFocusBorder = color.NRGBA{R: 0x64, G: 0x4a, B: 0xa8, A: 0xff}
	mainPlaceholderColor = color.NRGBA{R: 0x76, G: 0x70, B: 0x84, A: 0xff}
	mainTextColor        = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	mainMutedColor       = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	mainFaintColor       = color.NRGBA{R: 0x5e, G: 0x58, B: 0x6d, A: 0xff}
	mainSelectionColor   = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0x66}
	mainAccentColor1     = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	mainAccentColor2     = color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff}
	mainDisabledColor1   = color.NRGBA{R: 0x49, G: 0x3b, B: 0x67, A: 0xff}
	mainDisabledColor2   = color.NRGBA{R: 0x35, G: 0x2a, B: 0x50, A: 0xff}
	mainHintFillColor    = color.NRGBA{R: 0x1b, G: 0x18, B: 0x26, A: 0xff}
	mainHintBorderColor  = color.NRGBA{R: 0x2d, G: 0x28, B: 0x3c, A: 0xff}
	mainHintTextColor    = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
	mainRowColor         = color.NRGBA{R: 0x12, G: 0x11, B: 0x1a, A: 0xff}
	mainRowHoverColor    = color.NRGBA{R: 0x18, G: 0x16, B: 0x22, A: 0xff}
	mainRowSelectedColor = color.NRGBA{R: 0x1a, G: 0x16, B: 0x26, A: 0xff}
	mainRowBorderColor   = color.NRGBA{R: 0x24, G: 0x21, B: 0x2f, A: 0xff}
	mainRowHoverBorder   = color.NRGBA{R: 0x35, G: 0x2f, B: 0x48, A: 0xff}
	mainControlColor     = color.NRGBA{R: 0xa9, G: 0xa2, B: 0xb9, A: 0xff}
	mainControlFill      = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	mainControlBorder    = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	mainHistoryPlayColor = color.NRGBA{R: 0x6c, G: 0xc8, B: 0x95, A: 0xff}
	mainDragDotColor     = color.NRGBA{R: 0x53, G: 0x4d, B: 0x63, A: 0xff}
	mainDragDotActive    = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}
	mainDangerColor      = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}
	mainDangerColor2     = color.NRGBA{R: 0xd1, G: 0x31, B: 0x48, A: 0xff}
	mainStatusDoneText   = color.NRGBA{R: 0x58, G: 0xdc, B: 0x9a, A: 0xff}
	mainStatusDoneFill   = color.NRGBA{R: 0x10, G: 0x2c, B: 0x21, A: 0xff}
	mainStatusDoneBorder = color.NRGBA{R: 0x24, G: 0x55, B: 0x3e, A: 0xff}
	mainStatusLiveText   = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}
	mainStatusLiveFill   = color.NRGBA{R: 0x1f, G: 0x18, B: 0x35, A: 0xff}
	mainStatusLiveBorder = color.NRGBA{R: 0x42, G: 0x32, B: 0x6e, A: 0xff}
	mainStatusAnnText    = color.NRGBA{R: 0x7f, G: 0xd6, B: 0xff, A: 0xff}
	mainStatusAnnFill    = color.NRGBA{R: 0x10, G: 0x24, B: 0x2e, A: 0xff}
	mainStatusAnnBorder  = color.NRGBA{R: 0x24, G: 0x53, B: 0x6a, A: 0xff}
)

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

// titleSource is the part of the library the main screen uses. Keeping the
// screen behind this interface means it can render and test without opening
// the API client or the SQLite database.
type titleSource interface {
	Load(context.Context, string) (catalog.Title, error)
	Fetch(context.Context, string) (catalog.Title, error)
	History() ([]catalog.Title, error)
	DeleteHistory(int64) error
	ReorderHistory([]catalog.Title) error
	ReorderStatusHistory(catalog.UserStatus, []catalog.Title) error
}

type mainWindow struct {
	typo   *typography
	titles titleSource

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
	onUserStatus  func(historyTitle, catalog.UserStatus) error
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

func newMainWindow(typo *typography, titles titleSource, history []catalog.Title) *mainWindow {
	displayHistory := make([]historyTitle, 0, len(history))
	for _, item := range history {
		displayHistory = append(displayHistory, titleFromLibrary(item))
	}
	m := &mainWindow{
		typo:            typo,
		titles:          titles,
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
	if m.hintAnim.Progress > 0.02 {
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
		if m.titles == nil {
			m.deliverLoadResult(titleLoadResult{sequence: sequence, err: context.Canceled})
			return
		}
		// A title typed into the URL field is only fetched; it enters the local
		// history when the user presses Save in the title window. A title
		// opened from history is already saved and is refreshed in place.
		var item catalog.Title
		var err error
		if target >= 0 {
			item, err = m.titles.Load(ctx, input)
		} else {
			item, err = m.titles.Fetch(ctx, input)
		}
		result := titleLoadResult{sequence: sequence, err: err}
		if err == nil {
			result.title = titleFromLibrary(item)
			if target >= 0 {
				if history, historyErr := m.titles.History(); historyErr == nil {
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
	if m.titles == nil {
		return
	}
	history, err := m.titles.History()
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
	if m.titles == nil {
		return
	}
	m.syncVisibleStatusOrder()
	var err error
	if m.filter == filterAll {
		err = m.titles.ReorderHistory(displayTitles(m.allHistory))
	} else if status, ok := m.filter.userStatus(); ok {
		err = m.titles.ReorderStatusHistory(status, displayTitles(m.history))
	}
	if err != nil {
		m.loadError = err.Error()
		m.reportError(err)
	}
}

func displayTitles(items []historyTitle) []catalog.Title {
	out := make([]catalog.Title, 0, len(items))
	for _, item := range items {
		out = append(out, catalog.Title{ID: item.id})
	}
	return out
}

func (m *mainWindow) layoutInput(gtx layout.Context, rect image.Rectangle, focused bool) {
	field := actionField{
		typo:        m.typo,
		editor:      &m.editor,
		submit:      &m.next,
		submitHover: &m.nextHover,
		box: fieldBoxStyle{
			fill:        mainInputColor,
			border:      mainInputBorder,
			focusBorder: mainInputFocusBorder,
			errorBorder: mainDangerColor,
			radius:      mainInputRadius,
		},
		textSize:     mainInputTextSize,
		paddingX:     unit.Dp(12),
		gap:          mainInputGap,
		leadingSize:  mainMascotSize,
		leadingInset: unit.Dp(11),
		leading: func(gtx layout.Context, mascot image.Rectangle, visual fieldVisual) {
			mood := mascotNormal
			switch {
			case visual.loading:
				mood = mascotLoading
			case visual.invalid:
				mood = mascotAlert
			}
			shake := float32(0)
			if visual.invalid {
				shake = errorShake(visual.now, m.errorAt)
			}
			offset := op.Offset(image.Pt(errorShakePixels(gtx, shake, mainMascotShakeOffset), 0)).Push(gtx.Ops)
			drawCatgirl(gtx, mascot, visual.now, seasonalMascotHat(visual.now), mood)
			offset.Pop()
		},
		trailing: func(gtx layout.Context, _ int) (int, func(image.Rectangle)) {
			face := font.Font{Typeface: fontFamily, Weight: font.Medium}
			_, ink := m.typo.measure(gtx, "Ctrl + K", face, mainHintTextSize)
			width := ink.Dx() + 2*gtx.Dp(mainHintPaddingX)
			return width, func(rect image.Rectangle) { m.drawHint(gtx, rect) }
		},
		placeholder: func(gtx layout.Context, rect image.Rectangle) {
			m.drawInputPlaceholder(gtx, rect)
		},
		loading:       m.loading,
		errorAt:       m.errorAt,
		animationsRun: m.animationsRun,
		onSubmit: func(gtx layout.Context) {
			if strings.TrimSpace(m.editor.Text()) == "" {
				m.triggerEmptyLink(gtx)
				return
			}
			m.startLoading(gtx, -1, false)
		},
	}
	field.layout(gtx, rect, focused)
}

func (m *mainWindow) drawInputPlaceholder(gtx layout.Context, rect image.Rectangle) {
	if m.editor.Len() != 0 {
		return
	}
	text := m.placeholder.visibleText()
	if text == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, ink := m.typo.measure(gtx, text, face, mainInputTextSize)
	if ink.Empty() {
		return
	}
	m.drawTruncated(
		gtx,
		text,
		face,
		mainInputTextSize,
		mainPlaceholderColor,
		image.Pt(rect.Min.X, rect.Min.Y+(rect.Dy()-ink.Dy())/2),
		rect.Dx(),
	)
}

func (m *mainWindow) drawHint(gtx layout.Context, rect image.Rectangle) {
	progress := m.hintAnim.Progress
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
	labelWidth := 0
	for _, label := range []string{"Очистить всё", "Отменить"} {
		if _, ink := m.typo.measure(gtx, label, clearFace, mainClearSize); ink.Dx() > labelWidth {
			labelWidth = ink.Dx()
		}
	}
	if m.undo.active {
		clearLabel = "Отменить"
	}
	_, clearInk := m.typo.measure(gtx, clearLabel, clearFace, mainClearSize)
	iconSize := gtx.Dp(unit.Dp(20))
	clearGap := gtx.Dp(unit.Dp(6))
	// Reserve the widest label and icon footprint even while "Отменить" is
	// shown, otherwise the status filters jump left as soon as a title is
	// deleted.
	clearWidth := labelWidth + clearGap + iconSize
	clearHeight := rect.Dy()
	clearRect := image.Rect(rect.Max.X-clearWidth, rect.Min.Y, rect.Max.X, rect.Min.Y+clearHeight)
	if clearRect.Empty() {
		return
	}

	filterLeft, filterRight := historyFilterSpan(gtx, rect, titleInk, countInk, clearRect)
	m.layoutHistoryFilters(gtx, filterLeft, filterRight, rect.Min.Y+rect.Dy()/2)

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
	groupWidth := clearInk.Dx() + clearGap + iconSize
	labelX := clearRect.Max.X - groupWidth
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

func (m *mainWindow) layoutHistory(gtx layout.Context, rect image.Rectangle) {
	m.list.Gap = gtx.Dp(mainListGap)
	rowHeight := gtx.Dp(mainRowHeight)
	gap := m.list.Gap
	m.listRect = rect
	m.listRowHeight = rowHeight
	m.listGap = gap
	m.handleHistoryDrag(gtx, rect, rowHeight, gap)
	if len(m.history) == 0 {
		text := "История пока пуста"
		if m.filter != filterAll {
			text = "Ничего не найдено"
		}
		emptyState(gtx, m.typo, rect, text, mainHistoryCountSize, mainMutedColor)
		return
	}

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
func (m *mainWindow) setUserStatus(id int64, status catalog.UserStatus) {
	if id == 0 {
		return
	}
	m.ensureAllHistory()
	target := catalog.NormalizeUserStatus(status)
	previous, found := catalog.UserStatus(0), false
	for index := range m.allHistory {
		if m.allHistory[index].id == id {
			previous = catalog.NormalizeUserStatus(m.allHistory[index].userStatus)
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
		if item.id == id || catalog.NormalizeUserStatus(item.userStatus) != target {
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
