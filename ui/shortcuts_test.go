package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestLetterShortcutsMatchRussianLayout(t *testing.T) {
	for _, name := range []key.Name{"P", "p", "З", "з"} {
		if !isPlayKey(name) {
			t.Fatalf("%q should start playback", name)
		}
	}
	for _, name := range []key.Name{"S", "s", "Ы", "ы"} {
		if !isSaveKey(name) {
			t.Fatalf("%q should save the title", name)
		}
	}
	for _, name := range []key.Name{"A", "Ф", "Enter", ""} {
		if isPlayKey(name) || isSaveKey(name) {
			t.Fatalf("%q should not be a letter shortcut", name)
		}
	}
}

func TestMainArrowKeysCycleHistoryFilters(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	m := &mainScreen{filter: filterAll, selected: -1}
	frame := func() {
		ops.Reset()
		m.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame()
	if m.filter != filterWatching {
		t.Fatalf("right arrow selected %d, want watching", m.filter)
	}

	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if m.filter != filterAll {
		t.Fatalf("left arrow selected %d, want all", m.filter)
	}

	// Left from the first pill wraps to the last one.
	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if m.filter != filterDropped {
		t.Fatalf("left arrow did not wrap, got %d", m.filter)
	}
}

func TestMainPlayShortcutWorksInRussianLayout(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	var played []int64
	m := &mainScreen{
		history:  testHistory(),
		selected: 3,
		filter:   filterAll,
		onPlayHistory: func(item historyTitle) historyPlayResult {
			played = append(played, item.id)
			return historyPlayStarted
		},
	}
	frame := func() {
		ops.Reset()
		m.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	router.Queue(key.Event{Name: "P", State: key.Press})
	frame()
	if len(played) != 1 || played[0] != 4 {
		t.Fatalf("P played %v, want title 4", played)
	}
	if !m.playing || m.playingID != 4 {
		t.Fatalf("playing state = %v id=%d, want started 4", m.playing, m.playingID)
	}

	m.playing = false
	m.playingID = 0
	router.Queue(key.Event{Name: "З", State: key.Press})
	frame()
	if len(played) != 2 || played[1] != 4 {
		t.Fatalf("Russian P played %v, want another title 4", played)
	}

	m.playing = false
	m.playingID = 0
	router.Queue(key.Event{Name: "P", Modifiers: key.ModCtrl, State: key.Press})
	frame()
	if len(played) != 2 {
		t.Fatalf("Ctrl+P played %v, want it left for the system", played)
	}
}

func TestMainDeleteShortcutRemovesSelectedTitle(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	m := &mainScreen{
		history:    testHistory(),
		allHistory: testHistory(),
		selected:   2,
		filter:     filterAll,
	}
	m.rows = make([]historyRow, len(m.history))
	frame := func() {
		ops.Reset()
		m.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	router.Queue(key.Event{Name: key.NameDeleteForward, State: key.Press})
	frame()
	if len(m.history) != 9 {
		t.Fatalf("history length = %d, want 9", len(m.history))
	}
	if m.selected != -1 {
		t.Fatalf("selection = %d, want cleared", m.selected)
	}
	if !m.undo.active {
		t.Fatal("deletion did not start the undo window")
	}
}

func TestTitleArrowKeysMoveEpisodeAndVoice(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	w := &titleScreen{
		item: historyTitle{
			status:   statusOngoing,
			episodes: []string{"1", "2", "3"},
			voices: []voiceover{
				{name: "A"},
				{name: "B"},
				{name: "C"},
			},
		},
		selectedVoice:   0,
		selectedEpisode: 0,
	}
	frame := func() {
		ops.Reset()
		w.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame()
	if w.selectedEpisode != 1 {
		t.Fatalf("right arrow selected episode %d, want 1", w.selectedEpisode)
	}
	router.Queue(key.Event{Name: key.NameRightArrow, State: key.Press})
	frame()
	if w.selectedEpisode != 2 {
		t.Fatalf("right arrow selected episode %d, want 2", w.selectedEpisode)
	}
	router.Queue(key.Event{Name: key.NameLeftArrow, State: key.Press})
	frame()
	if w.selectedEpisode != 1 {
		t.Fatalf("left arrow selected episode %d, want 1", w.selectedEpisode)
	}

	router.Queue(key.Event{Name: key.NameDownArrow, State: key.Press})
	frame()
	if w.selectedVoice != 1 {
		t.Fatalf("down arrow selected voice %d, want 1", w.selectedVoice)
	}
	router.Queue(key.Event{Name: key.NameUpArrow, State: key.Press})
	frame()
	if w.selectedVoice != 0 {
		t.Fatalf("up arrow selected voice %d, want 0", w.selectedVoice)
	}
}

func TestTitleSaveAndWatchShortcutsWorkInRussianLayout(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	saved := 0
	watched := 0
	w := &titleScreen{
		item: historyTitle{
			status:   statusOngoing,
			episodes: []string{"1"},
			voices:   []voiceover{{name: "A"}},
		},
		selectedQuality: 1,
		onSave: func(voice, episode string) error {
			saved++
			return nil
		},
		onWatch: func(voice, episode string, quality int) error {
			watched++
			return nil
		},
	}
	frame := func() {
		ops.Reset()
		w.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	router.Queue(key.Event{Name: "S", State: key.Press})
	frame()
	if saved != 1 {
		t.Fatalf("S saved %d times, want 1", saved)
	}
	router.Queue(key.Event{Name: "Ы", State: key.Press})
	frame()
	if saved != 2 {
		t.Fatalf("Russian S saved %d times, want 2", saved)
	}

	router.Queue(key.Event{Name: "S", Modifiers: key.ModCtrl, State: key.Press})
	frame()
	if saved != 2 {
		t.Fatalf("Ctrl+S saved %d times, want it left for the system", saved)
	}

	router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	frame()
	if watched != 1 || !w.watching {
		t.Fatalf("Enter watched=%d watching=%v, want started", watched, w.watching)
	}
	w.watching = false
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame()
	if watched != 2 || !w.watching {
		t.Fatalf("keypad Enter watched=%d watching=%v, want started", watched, w.watching)
	}
}

func TestShortcutInfoIsPageSpecific(t *testing.T) {
	mainInfo := (&mainScreen{}).shortcutInfo()
	if mainInfo.page != "Главная" {
		t.Fatalf("main page = %q", mainInfo.page)
	}
	if !hasShortcut(mainInfo, "P") || !hasShortcut(mainInfo, "Del") {
		t.Fatal("main summary is missing playback or deletion")
	}
	if hasShortcut(mainInfo, "S") {
		t.Fatal("main summary leaked the title-page save key")
	}

	titleInfo := (&titleScreen{
		item: historyTitle{
			status:   statusOngoing,
			episodes: []string{"1"},
			voices:   []voiceover{{name: "A"}},
		},
	}).shortcutInfo()
	if titleInfo.page != "Страница тайтла" {
		t.Fatalf("title page = %q", titleInfo.page)
	}
	if !hasShortcut(titleInfo, "S") || !hasShortcut(titleInfo, "Enter") {
		t.Fatal("title summary is missing save or playback")
	}
	if hasShortcut(titleInfo, "P") || hasShortcut(titleInfo, "Del") {
		t.Fatal("title summary leaked main-page keys")
	}
	if hasShortcut(titleInfo, "← / →") || hasShortcut(titleInfo, "↑ / ↓") {
		t.Fatal("title summary advertised navigation that cannot move")
	}

	multiInfo := (&titleScreen{
		item: historyTitle{
			status:   statusOngoing,
			episodes: []string{"1", "2"},
			voices:   []voiceover{{name: "A"}, {name: "B"}},
		},
	}).shortcutInfo()
	if !hasShortcut(multiInfo, "← / →") || !hasShortcut(multiInfo, "↑ / ↓") {
		t.Fatal("title summary is missing episode or voice navigation")
	}

	announcedInfo := (&titleScreen{
		item: historyTitle{status: statusAnnounced},
	}).shortcutInfo()
	if hasShortcut(announcedInfo, "Enter") {
		t.Fatal("announced title advertised playback")
	}
}

func TestShortcutsBackButtonLivesInTheLeftFooter(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newShortcutsScreen(typo, (&mainScreen{}).shortcutInfo())
	backs := 0
	w.onBack = func() { backs++ }

	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 49)),
	}
	frame := func() {
		ops.Reset()
		w.layoutFooter(gtx)
		router.Frame(ops)
	}
	frame()

	click := func(at image.Point) {
		router.Queue(
			pointer.Event{
				Source:   pointer.Mouse,
				Buttons:  pointer.ButtonPrimary,
				Kind:     pointer.Press,
				Position: f32.Pt(float32(at.X), float32(at.Y)),
			},
			pointer.Event{
				Source:   pointer.Mouse,
				Buttons:  pointer.ButtonPrimary,
				Kind:     pointer.Release,
				Position: f32.Pt(float32(at.X), float32(at.Y)),
			},
		)
		frame()
	}

	click(image.Pt(700, 24))
	if backs != 0 {
		t.Fatal("the right side of the footer still triggers the back action")
	}
	click(image.Pt(88, 24))
	if backs != 1 {
		t.Fatalf("left footer button triggered %d time(s), want 1", backs)
	}
}

func hasShortcut(info shortcutInfo, keys string) bool {
	for _, item := range info.items {
		if item.keys == keys {
			return true
		}
	}
	return false
}
