package ui

import (
	"path/filepath"
	"testing"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/storage"
)

func TestUserStatusFilterStatus(t *testing.T) {
	if _, ok := filterAll.userStatus(); ok {
		t.Fatal("filterAll must not report a library status")
	}
	if status, ok := filterWatching.userStatus(); !ok || status != library.UserStatusWatching {
		t.Fatalf("watching filter status = %d/%v, want watching", status, ok)
	}
	if _, ok := userStatusFilter(99).userStatus(); ok {
		t.Fatal("unknown filter reported a library status")
	}
}

func TestVisibleHistoryUsesStatusOrderForFilteredViews(t *testing.T) {
	items := []historyTitle{
		{id: 1, userStatus: library.UserStatusWatching, statusOrder: 20},
		{id: 2, userStatus: library.UserStatusWatching, statusOrder: -10},
		{id: 3, userStatus: library.UserStatusPlanned, statusOrder: -100},
		{id: 4, userStatus: library.UserStatusWatching, statusOrder: 10},
	}

	m := &mainScreen{filter: filterWatching}
	if got := historyIDs(m.visibleHistory(items)); !equalIDs(got, []int64{2, 4, 1}) {
		t.Fatalf("filtered order = %v, want [2 4 1]", got)
	}

	m.filter = filterAll
	if got := historyIDs(m.visibleHistory(items)); !equalIDs(got, []int64{1, 2, 3, 4}) {
		t.Fatalf("all-titles order = %v, want the original [1 2 3 4]", got)
	}
}

func TestMoveFilteredHistoryUpdatesOnlyItsStatusOrder(t *testing.T) {
	all := []historyTitle{
		{id: 1, userStatus: library.UserStatusPlanned, statusOrder: 0},
		{id: 2, userStatus: library.UserStatusWatching, statusOrder: 10},
		{id: 3, userStatus: library.UserStatusPlanned, statusOrder: -10},
		{id: 4, userStatus: library.UserStatusWatching, statusOrder: 20},
		{id: 5, userStatus: library.UserStatusWatching, statusOrder: 30},
	}
	m := &mainScreen{
		allHistory: append([]historyTitle(nil), all...),
		filter:     filterWatching,
	}
	m.history = m.visibleHistory(m.allHistory)
	if got := historyIDs(m.history); !equalIDs(got, []int64{2, 4, 5}) {
		t.Fatalf("initial filtered order = %v, want [2 4 5]", got)
	}

	m.moveHistoryItem(0, 2)
	if got := historyIDs(m.history); !equalIDs(got, []int64{4, 5, 2}) {
		t.Fatalf("moved filtered order = %v, want [4 5 2]", got)
	}

	wantOrders := map[int64]int64{2: 2, 4: 0, 5: 1}
	for _, item := range m.allHistory {
		if order, ok := wantOrders[item.id]; ok && item.statusOrder != order {
			t.Fatalf("watching title %d order = %d, want %d", item.id, item.statusOrder, order)
		}
		if item.userStatus == library.UserStatusPlanned && item.statusOrder != all[item.id-1].statusOrder {
			t.Fatalf("planned title %d order changed to %d", item.id, item.statusOrder)
		}
	}

	// The unfiltered list keeps the all-titles order; only the status-specific
	// order changed.
	m.filter = filterAll
	if got := historyIDs(m.visibleHistory(m.allHistory)); !equalIDs(got, []int64{1, 2, 3, 4, 5}) {
		t.Fatalf("all-titles order changed: %v", got)
	}
}

func TestEachSectionKeepsItsOwnHistory(t *testing.T) {
	all := []historyTitle{
		{id: 1, source: catalog.SourceLibSocial},
		{id: 2, source: catalog.SourceMangaLib},
		{id: 3, source: catalog.SourceRanobeLib},
		{id: 4, source: catalog.SourceYaoiLib},
	}
	m := &mainScreen{
		allHistory: append([]historyTitle(nil), all...),
		filter:     filterAll,
		section:    sectionManga,
	}
	m.applyFilter()
	if got := historyIDs(m.history); !equalIDs(got, []int64{2, 4}) {
		t.Fatalf("manga history = %v, want [2 4]", got)
	}

	// The same call the startup path now uses must rebuild the list even when
	// the stored section already matches the one from newMainScreen.
	m.setSection(sectionRanobe)
	if got := historyIDs(m.history); !equalIDs(got, []int64{3}) {
		t.Fatalf("ranobe history = %v, want [3]", got)
	}
	m.setSection(sectionAnime)
	if got := historyIDs(m.history); !equalIDs(got, []int64{1}) {
		t.Fatalf("anime history = %v, want [1]", got)
	}
}

func TestRepeatedSectionSwitchesKeepAllHistory(t *testing.T) {
	all := []historyTitle{
		{id: 1, source: catalog.SourceLibSocial},
		{id: 2, source: catalog.SourceMangaLib},
		{id: 3, source: catalog.SourceRanobeLib},
		{id: 4, source: catalog.SourceMangaLib},
	}
	want := map[librarySection][]int64{
		sectionAnime:  {1},
		sectionManga:  {2, 4},
		sectionRanobe: {3},
	}
	m := &mainScreen{allHistory: append([]historyTitle(nil), all...), filter: filterAll}
	for round := 0; round < 100; round++ {
		for _, section := range []librarySection{sectionManga, sectionRanobe, sectionAnime} {
			m.setSection(section)
			if got := historyIDs(m.history); !equalIDs(got, want[section]) {
				t.Fatalf("round %d section %s history = %v, want %v", round, section.key(), got, want[section])
			}
		}
	}
}

func TestMovingRowsInOneSectionPreservesOtherSections(t *testing.T) {
	all := []historyTitle{
		{id: 1, source: catalog.SourceLibSocial},
		{id: 2, source: catalog.SourceMangaLib},
		{id: 3, source: catalog.SourceRanobeLib},
		{id: 4, source: catalog.SourceMangaLib},
	}
	m := &mainScreen{
		allHistory: append([]historyTitle(nil), all...),
		filter:     filterAll,
		section:    sectionManga,
	}
	m.history = m.visibleHistory(m.allHistory)
	m.moveHistoryItem(0, 1)

	if got := historyIDs(m.allHistory); !equalIDs(got, []int64{1, 4, 3, 2}) {
		t.Fatalf("complete history after drag = %v, want [1 4 3 2]", got)
	}
	m.setSection(sectionAnime)
	if got := historyIDs(m.history); !equalIDs(got, []int64{1}) {
		t.Fatalf("anime history was lost: %v", got)
	}
	m.setSection(sectionRanobe)
	if got := historyIDs(m.history); !equalIDs(got, []int64{3}) {
		t.Fatalf("ranobe history was lost: %v", got)
	}
}

func TestSetUserStatusMovesTitleToTopOfNewStatus(t *testing.T) {
	all := []historyTitle{
		{id: 1, userStatus: library.UserStatusWatching, statusOrder: 5},
		{id: 2, userStatus: library.UserStatusWatching, statusOrder: 15},
		{id: 3, userStatus: library.UserStatusPlanned, statusOrder: -20},
	}
	m := &mainScreen{
		allHistory: append([]historyTitle(nil), all...),
		history:    append([]historyTitle(nil), all...),
		filter:     filterAll,
	}

	m.setUserStatus(3, library.UserStatusWatching)
	item := m.allHistory[2]
	if item.userStatus != library.UserStatusWatching || item.statusOrder != -995 {
		t.Fatalf("promoted item = status %d order %d, want watching/-995", item.userStatus, item.statusOrder)
	}

	// Choosing the same status again must preserve the manual position.
	m.allHistory[2].statusOrder = 0
	m.history[2].statusOrder = 0
	m.setUserStatus(3, library.UserStatusWatching)
	if got := m.allHistory[2].statusOrder; got != 0 {
		t.Fatalf("same status changed order to %d, want 0", got)
	}
}

func TestSetUserStatusRemovesRowFromCurrentFilter(t *testing.T) {
	all := []historyTitle{
		{id: 1, userStatus: library.UserStatusWatching, statusOrder: 0},
		{id: 2, userStatus: library.UserStatusWatching, statusOrder: 10},
		{id: 3, userStatus: library.UserStatusPlanned, statusOrder: 0},
	}
	m := &mainScreen{
		allHistory: append([]historyTitle(nil), all...),
		filter:     filterWatching,
	}
	m.history = m.visibleHistory(m.allHistory)
	m.rows = make([]historyRow, len(m.history))

	m.setUserStatus(1, library.UserStatusPlanned)
	if got := historyIDs(m.history); !equalIDs(got, []int64{2}) {
		t.Fatalf("filtered rows after status change = %v, want [2]", got)
	}
	if len(m.rows) != len(m.history) {
		t.Fatalf("row cache length = %d, want %d", len(m.rows), len(m.history))
	}
}

func TestPersistHistoryOrderWritesOnlyTheActiveStatus(t *testing.T) {
	dir := t.TempDir()
	databasePath := filepath.Join(dir, "yoanime.sqlite")
	db, err := storage.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}

	specs := []struct {
		slug   string
		status int
	}{
		{"planned-a", 2},
		{"watching-a", 1},
		{"planned-b", 2},
		{"watching-b", 1},
		{"watching-c", 1},
	}
	ids := make([]int64, len(specs))
	for index, spec := range specs {
		id, err := db.UpsertTitle(storage.Title{Slug: spec.slug, UserStatus: spec.status})
		if err != nil {
			t.Fatal(err)
		}
		ids[index] = id
		if _, err := db.EnsureHistory(id, "1", "voice"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	lib, err := library.Open(databasePath, filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	all := make([]historyTitle, 0, len(history))
	for _, item := range history {
		all = append(all, titleFromLibrary(item))
	}
	m := &mainScreen{
		titles:     lib,
		allHistory: all,
		filter:     filterWatching,
	}
	m.history = m.visibleHistory(m.allHistory)
	if got := historyIDs(m.history); !equalIDs(got, []int64{ids[4], ids[3], ids[1]}) {
		t.Fatalf("initial watching order = %v, want [%d %d %d]", got, ids[4], ids[3], ids[1])
	}

	m.moveHistoryItem(0, 2)
	m.persistHistoryOrder()

	history, err = lib.History()
	if err != nil {
		t.Fatal(err)
	}
	orders := make(map[int64]int64, len(history))
	for _, item := range history {
		orders[item.ID] = item.StatusOrder
	}
	if orders[ids[3]] != 0 || orders[ids[1]] != 1 || orders[ids[4]] != 2 {
		t.Fatalf("saved watching order = %d/%d/%d, want 0/1/2", orders[ids[3]], orders[ids[1]], orders[ids[4]])
	}
	if orders[ids[0]] != 0 || orders[ids[2]] != -1000 {
		t.Fatalf("planned order changed: %d/%d", orders[ids[0]], orders[ids[2]])
	}
}
