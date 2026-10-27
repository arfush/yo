package ui

import (
	"image/color"
	"math/rand"
	"sort"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/arfush/yonime/log"
)

// undoDeleteDuration is how long a removed title can be brought back before it
// is deleted from the database for good.
const undoDeleteDuration = 10 * time.Second

var undoCountdownColor = mainDangerColor

// undoUnavailableColor is used for the "Отменить" action. The purple accent
// distinguishes it from the red clear action it temporarily replaces.
var undoActionColor = color.NRGBA{R: 0xb4, G: 0x9c, B: 0xff, A: 0xff}

var undoActionHoverColor = color.NRGBA{R: 0xd9, G: 0xcc, B: 0xff, A: 0xff}

// undoPhrases are shown in the history header while a deletion can still be
// undone. One is picked at random each time a new countdown starts.
var undoPhrases = []string{
	"О нет, не надо!", "Уверен, сэмпай?", "Бака! Стой!", "Нани?! Не так быстро!",
	"Chotto matte!", "Это же жестоко!", "Ватащи-но тайтлы!", "Подумай ещё раз!",
	"Не будь таким холодным!", "Сугои... верни их!", "Yamete kudasai!", "Nani?!",
	"Это же цундере-жестокость!", "Онии-тян, за что?!", "Гомэн, но я против!",
	"Верни моё моэ!", "Кавай же! Зачем?!", "Чиби-тян расстроится!",
	"Ты серьёзно, кун?", "Сумимасуэн, но нет!", "Моя коллекция! Гя-а-а!",
	"Это же не хентай, сэмпай!", "Твоя вайфу расстроится!", "Усо да! Это неправда!",
	"Онии-сама, что ты творишь?!", "Маджидэ? Ты не шутишь?", "Масака! Как ты мог?",
	"Хидой! Это же хидой!", "Аниме-боги не простят!", "Умэ-умэ! Верни!",
	"Ты не сэмпай, ты анти-сэмпай!", "Omae wa mou shindeiru... стой!",
	"Какой же ты цун-цун!", "Хэнтай! Не смей удалять!", "Твоя вайфу-карма утекает!",
	"Не-не-не, стой, тайм-аут!", "Додзё! Я умоляю!", "Ками-сама тебя накажет!",
	"Это же аниме-грех!", "Ксо! Стой!", "Итадакимас... перед удалением!",
	"Джян-джян! Не делай этого!", "Сэмпай, ты чего?!", "Твоя отаку-карма заплачет!",
	"Ара-ара, не так резко!", "Нани сол! Не трогай!", "Это же precious cargo!",
	"Суки-суки! Оставь!", "Ганбатте, undo-о!", "Твой топ аниме утекает!",
}

// pendingHistoryDelete remembers one hidden row together with its neighbours
// at the moment it was removed. The neighbours let the row slide back into its
// old place when the deletion is undone.
type pendingHistoryDelete struct {
	item   historyTitle
	seq    int
	prevID int64
	nextID int64
}

// undoDeleteState tracks the batch of rows removed during the current undo
// window. All deletions within those ten seconds share one countdown.
type undoDeleteState struct {
	active   bool
	phrase   string
	deadline time.Time
	seq      int
	pending  []pendingHistoryDelete
}

func (u *undoDeleteState) remaining(now time.Time) time.Duration {
	if !u.active {
		return 0
	}
	left := u.deadline.Sub(now)
	if left < 0 {
		return 0
	}
	return left
}

// pendingIDs reports the database ids of rows that are only hidden. History
// reloads from the library must skip them until the countdown finishes.
func (u *undoDeleteState) pendingIDs() map[int64]bool {
	if len(u.pending) == 0 {
		return nil
	}
	ids := make(map[int64]bool, len(u.pending))
	for _, pending := range u.pending {
		if pending.item.id != 0 {
			ids[pending.item.id] = true
		}
	}
	return ids
}

// visibleHistory removes hidden rows from a history snapshot loaded from the
// library while an undo countdown is still running, then applies the selected
// status filter. A status view uses its own order; the unfiltered view keeps
// the all-titles order.
func (m *mainWindow) visibleHistory(items []historyTitle) []historyTitle {
	hidden := m.undo.pendingIDs()
	visible := make([]historyTitle, 0, len(items))
	for _, item := range items {
		if item.id != 0 && hidden[item.id] {
			continue
		}
		if !m.filter.matches(item.userStatus) {
			continue
		}
		visible = append(visible, item)
	}
	if m.filter != filterAll {
		sort.SliceStable(visible, func(i, j int) bool {
			return visible[i].statusOrder < visible[j].statusOrder
		})
	}
	return visible
}

// ensureAllHistory repairs the complete-history model for tests and UI state
// created before status filtering existed.
func (m *mainWindow) ensureAllHistory() {
	if len(m.allHistory) == 0 && len(m.history) > 0 {
		m.allHistory = append([]historyTitle(nil), m.history...)
	}
}

// applyFilter rebuilds the visible rows from the complete history. Selection
// and scroll position refer to the filtered list, so both are reset.
func (m *mainWindow) applyFilter() {
	m.history = m.visibleHistory(m.allHistory)
	m.rows = make([]historyRow, len(m.history))
	m.selected = -1
	m.list.Position = layout.Position{}
	m.statusMenu.close()
}

// queueHistoryDelete hides one row and starts (or extends) the undo window.
// The database row is removed only when the countdown expires.
func (m *mainWindow) queueHistoryDelete(index int, now time.Time) bool {
	if m.loading || index < 0 || index >= len(m.history) {
		return false
	}
	m.ensureAllHistory()

	item := m.history[index]
	log.Info("history row queued for deletion", "title", item.id, "name", item.title)
	pending := pendingHistoryDelete{item: item}
	if allIndex := historyIndexByID(m.allHistory, item.id); allIndex >= 0 {
		if allIndex > 0 {
			pending.prevID = m.allHistory[allIndex-1].id
		}
		if allIndex+1 < len(m.allHistory) {
			pending.nextID = m.allHistory[allIndex+1].id
		}
		m.allHistory = append(m.allHistory[:allIndex], m.allHistory[allIndex+1:]...)
	}

	m.history = append(m.history[:index], m.history[index+1:]...)
	if index < len(m.rows) {
		m.rows = append(m.rows[:index], m.rows[index+1:]...)
	}
	switch {
	case m.selected == index:
		m.selected = -1
	case m.selected > index:
		m.selected--
	}
	m.loadError = ""

	m.pushPendingDeletes([]pendingHistoryDelete{pending}, now)
	return true
}

// queueClearHistory hides every visible row at once. Previously hidden rows
// stay in the same undo batch, so one "Отменить" brings all of them back.
func (m *mainWindow) queueClearHistory(now time.Time) bool {
	if m.loading {
		return false
	}
	m.ensureAllHistory()
	if len(m.allHistory) == 0 {
		return false
	}

	log.Info("clearing history", "count", len(m.allHistory))
	pending := make([]pendingHistoryDelete, 0, len(m.allHistory))
	for index := range m.allHistory {
		item := pendingHistoryDelete{item: m.allHistory[index]}
		if index > 0 {
			item.prevID = m.allHistory[index-1].id
		}
		if index+1 < len(m.allHistory) {
			item.nextID = m.allHistory[index+1].id
		}
		pending = append(pending, item)
	}

	m.allHistory = nil
	m.history = nil
	m.rows = nil
	m.selected = -1
	m.loadError = ""
	m.list.Position = layout.Position{}

	m.pushPendingDeletes(pending, now)
	return true
}

func (m *mainWindow) pushPendingDeletes(items []pendingHistoryDelete, now time.Time) {
	if len(items) == 0 {
		return
	}
	for index := range items {
		items[index].seq = m.undo.seq
		m.undo.seq++
	}
	m.undo.pending = append(m.undo.pending, items...)
	if !m.undo.active {
		m.undo.active = true
		m.undo.phrase = undoPhrases[rand.Intn(len(undoPhrases))]
	}
	// Every deletion gets its own full ten seconds, even when it joins rows
	// that were hidden a moment earlier.
	m.undo.deadline = now.Add(undoDeleteDuration)
}

// undoHistoryDelete brings every hidden row back. Rows are reinserted from the
// newest deletion to the oldest: the neighbours recorded at deletion time are
// then already visible when an older row is placed.
func (m *mainWindow) undoHistoryDelete() {
	pending := m.undo.pending
	m.undo = undoDeleteState{}
	if len(pending) == 0 {
		return
	}

	for index := len(pending) - 1; index >= 0; index-- {
		item := pending[index]
		at := m.restoreIndex(item)
		m.allHistory = insertHistoryTitle(m.allHistory, at, item.item)
	}
	log.Info("history deletion undone", "count", len(pending))
	m.applyFilter()
	m.loadError = ""
	m.persistHistoryOrder()
}

func (m *mainWindow) restoreIndex(pending pendingHistoryDelete) int {
	if pending.nextID != 0 {
		for index := range m.allHistory {
			if m.allHistory[index].id == pending.nextID {
				return index
			}
		}
	}
	if pending.prevID != 0 {
		for index := range m.allHistory {
			if m.allHistory[index].id == pending.prevID {
				return index + 1
			}
		}
	}
	return len(m.allHistory)
}

func historyIndexByID(items []historyTitle, id int64) int {
	for index := range items {
		if items[index].id == id {
			return index
		}
	}
	return -1
}

func insertHistoryTitle(list []historyTitle, index int, item historyTitle) []historyTitle {
	if index < 0 {
		index = 0
	}
	if index > len(list) {
		index = len(list)
	}
	list = append(list, historyTitle{})
	copy(list[index+1:], list[index:])
	list[index] = item
	return list
}

func insertHistoryRow(list []historyRow, index int, row historyRow) []historyRow {
	if index < 0 {
		index = 0
	}
	if index > len(list) {
		index = len(list)
	}
	list = append(list, historyRow{})
	copy(list[index+1:], list[index:])
	list[index] = row
	return list
}

// finalizeHistoryDeletes removes the hidden rows from the database once the
// countdown has run out (or the application is closing).
func (m *mainWindow) finalizeHistoryDeletes() {
	pending := m.undo.pending
	m.undo = undoDeleteState{}
	if len(pending) == 0 || m.library == nil {
		return
	}

	var firstErr error
	for _, item := range pending {
		if item.item.id == 0 {
			continue
		}
		if err := m.library.DeleteHistory(item.item.id); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		forgetPoster(item.item.coverPath)
	}
	if firstErr != nil {
		m.loadError = firstErr.Error()
		m.reportError(firstErr)
	}
}

// headerNotice exposes the undo countdown to the shell header. The history
// header keeps the "Отменить" action and the filters; only the transient
// sentence moves up.
func (m *mainWindow) headerNotice(now time.Time) headerNotice {
	remaining := m.undo.remaining(now)
	if !m.undo.active || remaining <= 0 {
		return headerNotice{}
	}
	return headerNotice{
		text: m.undo.phrase + " · " + rateLimitWaitLabel(remaining),
		tone: headerNoticeUndo,
	}
}

// updateUndoDelete commits expired deletions and keeps repainting while the
// countdown is visible so the seconds tick down promptly.
func (m *mainWindow) updateUndoDelete(gtx layout.Context) {
	if !m.undo.active {
		return
	}
	if m.undo.remaining(gtx.Now) <= 0 {
		m.finalizeHistoryDeletes()
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(100 * time.Millisecond)})
}
