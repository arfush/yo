package ui

import (
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

func undoTestWindow() *mainWindow {
	history := testHistory()
	return &mainWindow{
		history:  history,
		rows:     make([]historyRow, len(history)),
		selected: 2,
	}
}

func historyIDs(items []historyTitle) []int64 {
	ids := make([]int64, len(items))
	for index, item := range items {
		ids[index] = item.id
	}
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func TestUndoHistoryDeleteRestoresSingleRow(t *testing.T) {
	m := undoTestWindow()
	now := time.Now()

	if !m.queueHistoryDelete(2, now) {
		t.Fatal("queueHistoryDelete returned false")
	}
	if len(m.history) != 9 {
		t.Fatalf("history length = %d, want 9", len(m.history))
	}
	if m.selected != -1 {
		t.Fatalf("selected = %d, want -1 after deleting the selected row", m.selected)
	}
	if !m.undo.active || m.undo.phrase == "" || len(m.undo.pending) != 1 {
		t.Fatalf("undo state not started: %+v", m.undo)
	}

	m.undoHistoryDelete()
	if len(m.history) != 10 || len(m.rows) != 10 {
		t.Fatalf("history/rows length = %d/%d, want 10/10", len(m.history), len(m.rows))
	}
	want := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := historyIDs(m.history); !equalIDs(got, want) {
		t.Fatalf("restored order = %v, want %v", got, want)
	}
	if m.undo.active || len(m.undo.pending) != 0 {
		t.Fatalf("undo state not cleared: %+v", m.undo)
	}
}

func TestUndoAfterClearAllRestoresRowsInOriginalOrder(t *testing.T) {
	m := undoTestWindow()
	now := time.Now()

	// One row is already pending when the whole history is cleared; undoing
	// must bring back both batches in their original order.
	if !m.queueHistoryDelete(2, now) {
		t.Fatal("queueHistoryDelete returned false")
	}
	if !m.queueClearHistory(now.Add(time.Second)) {
		t.Fatal("queueClearHistory returned false")
	}
	if len(m.history) != 0 || len(m.rows) != 0 {
		t.Fatalf("history/rows length = %d/%d, want 0/0", len(m.history), len(m.rows))
	}
	if len(m.undo.pending) != 10 {
		t.Fatalf("pending rows = %d, want 10", len(m.undo.pending))
	}

	m.undoHistoryDelete()
	want := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := historyIDs(m.history); !equalIDs(got, want) {
		t.Fatalf("restored order = %v, want %v", got, want)
	}
	if len(m.rows) != len(m.history) {
		t.Fatalf("rows length = %d, want %d", len(m.rows), len(m.history))
	}
}

func TestUndoDeleteRestartsCountdownForEachDeletion(t *testing.T) {
	m := undoTestWindow()
	now := time.Now()

	m.queueHistoryDelete(0, now)
	firstDeadline := m.undo.deadline
	if left := m.undo.remaining(now); left != undoDeleteDuration {
		t.Fatalf("remaining = %v, want %v", left, undoDeleteDuration)
	}

	later := now.Add(4 * time.Second)
	m.queueHistoryDelete(0, later)
	if want := firstDeadline.Add(4 * time.Second); !m.undo.deadline.Equal(want) {
		t.Fatalf("deadline = %v, want %v", m.undo.deadline, want)
	}
	if left := m.undo.remaining(later); left != undoDeleteDuration {
		t.Fatalf("remaining after second delete = %v, want %v", left, undoDeleteDuration)
	}
}

func TestVisibleHistorySkipsPendingRows(t *testing.T) {
	m := undoTestWindow()
	if !m.queueHistoryDelete(3, time.Now()) {
		t.Fatal("queueHistoryDelete returned false")
	}

	loaded := testHistory()
	visible := m.visibleHistory(loaded)
	want := []int64{1, 2, 3, 5, 6, 7, 8, 9, 10}
	if got := historyIDs(visible); !equalIDs(got, want) {
		t.Fatalf("visible ids = %v, want %v", got, want)
	}
}

func TestUndoExpiresAndClearsState(t *testing.T) {
	m := undoTestWindow()
	now := time.Now()
	if !m.queueHistoryDelete(0, now) {
		t.Fatal("queueHistoryDelete returned false")
	}

	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: now.Add(undoDeleteDuration + time.Second)}
	m.updateUndoDelete(gtx)
	if m.undo.active || len(m.undo.pending) != 0 {
		t.Fatalf("undo state not finalized: %+v", m.undo)
	}
	if len(m.history) != 9 {
		t.Fatalf("history length = %d, want the row to stay hidden", len(m.history))
	}
}

func TestDeleteKeepsSelectionRelativeToRemovedRow(t *testing.T) {
	m := undoTestWindow()
	m.selected = 4
	if !m.queueHistoryDelete(1, time.Now()) {
		t.Fatal("queueHistoryDelete returned false")
	}
	if m.selected != 3 {
		t.Fatalf("selected = %d, want 3", m.selected)
	}

	m.selected = 1
	if !m.queueHistoryDelete(4, time.Now()) {
		t.Fatal("queueHistoryDelete returned false")
	}
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}
}
