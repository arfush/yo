package ui

import (
	"image"
	"testing"

	"gioui.org/io/key"
)

func testHistory() []historyTitle {
	history := make([]historyTitle, 10)
	for index := range history {
		history[index] = historyTitle{
			id:       int64(index + 1),
			slug:     "title-" + string(rune('a'+index)),
			title:    "Тайтл " + string(rune('А'+index)),
			original: "Title " + string(rune('A'+index)),
			status:   titleStatus(index % 3),
			initial:  string(rune('А' + index)),
			episodes: []string{"1", "2", "3"},
			voices:   []voiceover{{name: "Озвучка"}},
		}
	}
	return history
}

func TestSetInputFocusClearsSelectionWhenEnteringInput(t *testing.T) {
	m := &mainWindow{selected: 4}

	m.setInputFocused(true)
	if !m.inputFocused || m.selected != -1 {
		t.Fatalf("focusing input should clear selection, got focused=%v selected=%d", m.inputFocused, m.selected)
	}

	m.selected = 2
	m.setInputFocused(false)
	if m.inputFocused || m.selected != 2 {
		t.Fatalf("leaving input should keep selection, got focused=%v selected=%d", m.inputFocused, m.selected)
	}
}

func TestNoteWindowPressClearsSelectionOutsideCard(t *testing.T) {
	m := &mainWindow{
		history:       testHistory(),
		selected:      1,
		listRect:      image.Rect(20, 100, 800, 400),
		listRowHeight: 60,
		listGap:       10,
	}
	// Row 1 occupies y=170..230 in list viewport coordinates.
	m.noteWindowPress(image.Pt(400, 200))
	if m.selected != 1 {
		t.Fatalf("point inside the selected card should keep selection, got %d", m.selected)
	}
	for _, point := range []image.Point{
		image.Pt(400, 155), // header/input area
		image.Pt(400, 235), // gap below the card
		image.Pt(-1, -1),   // non-client header fallback
	} {
		m.selected = 1
		m.noteWindowPress(point)
		if m.selected != -1 {
			t.Fatalf("press at %v should clear selection, got %d", point, m.selected)
		}
	}

	// A selected row scrolled completely out of the list has no visible card.
	m.selected = 1
	m.list.Position.First = 2
	m.list.Position.Offset = 10
	m.noteWindowPress(image.Pt(400, 200))
	if m.selected != -1 {
		t.Fatalf("press with an off-screen selected row should clear selection, got %d", m.selected)
	}

	// Presses during a drag must not disturb the drag state.
	m.selected = 1
	m.list.Position.First = 0
	m.list.Position.Offset = 0
	m.drag.active = true
	m.noteWindowPress(image.Pt(400, 155))
	if m.selected != 1 {
		t.Fatalf("drag press should keep selection, got %d", m.selected)
	}
}

func TestShortcutMatchesLatinAndCyrillicK(t *testing.T) {
	for _, name := range []key.Name{"K", "k", "Л", "л"} {
		if !isCtrlK(name) {
			t.Fatalf("Ctrl+%q should focus the input", name)
		}
	}
	for _, name := range []key.Name{"L", "Д", "J", ""} {
		if isCtrlK(name) {
			t.Fatalf("Ctrl+%q should not focus the input", name)
		}
	}
}

func TestNextSelectionStartsAndClamps(t *testing.T) {
	tests := []struct {
		name    string
		current int
		delta   int
		count   int
		want    int
	}{
		{name: "first down", current: -1, delta: 1, count: 10, want: 0},
		{name: "first up wraps to last", current: -1, delta: -1, count: 10, want: 9},
		{name: "down moves", current: 3, delta: 1, count: 10, want: 4},
		{name: "up moves", current: 3, delta: -1, count: 10, want: 2},
		{name: "clamps at top", current: 0, delta: -1, count: 10, want: 0},
		{name: "clamps at bottom", current: 9, delta: 1, count: 10, want: 9},
		{name: "empty history", current: 4, delta: 1, count: 0, want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextSelection(tt.current, tt.delta, tt.count); got != tt.want {
				t.Fatalf("nextSelection(%d, %d, %d) = %d, want %d", tt.current, tt.delta, tt.count, got, tt.want)
			}
		})
	}
}

func TestMoveHistoryItemKeepsSelectionAndLoadingWithTitle(t *testing.T) {
	history := testHistory()
	selectedTitle := history[2].title
	loadingTitle := history[4].title
	m := &mainWindow{
		history:       append([]historyTitle(nil), history...),
		selected:      2,
		loadingTarget: 4,
	}

	m.moveHistoryItem(2, 5)
	if got := m.history[5].title; got != selectedTitle {
		t.Fatalf("moved title = %q, want %q", got, selectedTitle)
	}
	if m.selected != 5 {
		t.Fatalf("selection = %d, want 5", m.selected)
	}
	if m.loadingTarget != 3 {
		t.Fatalf("loading target = %d, want 3", m.loadingTarget)
	}
	if got := m.history[3].title; got != loadingTitle {
		t.Fatalf("loading title = %q, want %q", got, loadingTitle)
	}
}

func TestReorderIndexFollowsMovedItem(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		from, to int
		want     int
	}{
		{name: "moved item", index: 2, from: 2, to: 5, want: 5},
		{name: "shift down", index: 4, from: 2, to: 5, want: 3},
		{name: "outside down move", index: 1, from: 2, to: 5, want: 1},
		{name: "shift up", index: 3, from: 5, to: 2, want: 4},
		{name: "outside up move", index: 1, from: 5, to: 2, want: 1},
		{name: "unmoved", index: 3, from: 2, to: 2, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reorderIndex(tt.index, tt.from, tt.to); got != tt.want {
				t.Fatalf("reorderIndex(%d, %d, %d) = %d, want %d", tt.index, tt.from, tt.to, got, tt.want)
			}
		})
	}
}
