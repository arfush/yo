package ui

import (
	"fmt"
	"image"
	"reflect"
	"testing"
)

func TestSectionLabelsSplitSeasonsAndEpisodes(t *testing.T) {
	season1 := make([]string, 0, 11)
	for episode := 1; episode <= 11; episode++ {
		season1 = append(season1, fmt.Sprintf("S1E%d", episode))
	}
	season2 := []string{"S2E1", "S2E2", "S2E3"}
	series := historyTitle{
		episodes: append(append([]string(nil), season1...), season2...),
		seasons: []titleSeason{
			{number: 1, episodes: season1},
			{number: 2, episodes: season2},
		},
	}
	if got := series.seasonsSectionLabel(); got != "СЕЗОНЫ (1-2)" {
		t.Fatalf("season label = %q", got)
	}
	if got := series.episodesSectionLabel(0); got != "СЕРИИ (1-11)" {
		t.Fatalf("season 1 episode label = %q", got)
	}
	if got := series.episodesSectionLabel(1); got != "СЕРИИ (1-3)" {
		t.Fatalf("season 2 episode label = %q", got)
	}
	if got := series.episodesSectionLabel(99); got != "СЕРИИ (1-11)" {
		t.Fatalf("out-of-range season label = %q", got)
	}
	anime := historyTitle{episodes: []string{"1", "2", "3"}}
	if got := anime.seasonsSectionLabel(); got != "" {
		t.Fatalf("anime season label = %q", got)
	}
	if got := anime.episodesSectionLabel(0); got != "СЕРИИ (1-3)" {
		t.Fatalf("anime label = %q", got)
	}
}

func TestEpisodeGridColumnsKeepEpisodeShape(t *testing.T) {
	if got := episodeGridColumns(500, 6, 44); got != 7 {
		t.Fatalf("wide grid columns = %d", got)
	}
	if got := episodeGridColumns(200, 6, 44); got != 4 {
		t.Fatalf("narrow grid columns = %d", got)
	}
	if got := episodeGridColumns(40, 6, 44); got != 1 {
		t.Fatalf("tiny grid columns = %d", got)
	}
}

func TestGridBlockSizeCapsAtThreeRows(t *testing.T) {
	tall := image.Rect(0, 0, 500, 1000)
	if rows, height := gridBlockSize(tall, 10, 42, 6); rows != titleGridMaxRows || height != 3*42+2*6 {
		t.Fatalf("tall block = %d rows, %d px; want %d rows, 138 px", rows, height, titleGridMaxRows)
	}
	if rows, height := gridBlockSize(tall, 2, 42, 6); rows != 2 || height != 2*42+6 {
		t.Fatalf("short block = %d rows, %d px; want 2 rows, 90 px", rows, height)
	}
	medium := image.Rect(0, 0, 500, 100)
	if rows, height := gridBlockSize(medium, 10, 42, 6); rows != 2 || height != 90 {
		t.Fatalf("medium block = %d rows, %d px; want 2 rows, 90 px", rows, height)
	}
	tiny := image.Rect(0, 0, 500, 20)
	if rows, height := gridBlockSize(tiny, 10, 42, 6); rows != 1 || height != 20 {
		t.Fatalf("tiny block = %d rows, %d px; want 1 clipped row, 20 px", rows, height)
	}
}

func TestEpisodeLabels(t *testing.T) {
	if got := episodeDisplayLabel("S2E5"); got != "5" {
		t.Fatalf("season episode label = %q", got)
	}
	if got := episodeDisplayLabel("5"); got != "5" {
		t.Fatalf("plain episode label = %q", got)
	}
	if got := episodeRangeLabel([]string{"S1E1", "S1E2"}); got != "СЕРИИ (1-2)" {
		t.Fatalf("kinopoisk single season range = %q", got)
	}
}

func TestVisibleEpisodeIndicesFollowSeason(t *testing.T) {
	item := historyTitle{
		episodes: []string{"S1E1", "S1E2", "S1E3", "S2E1", "S2E2"},
		seasons: []titleSeason{
			{number: 1, episodes: []string{"S1E1", "S1E2", "S1E3"}},
			{number: 2, episodes: []string{"S2E1", "S2E2"}},
		},
	}
	window := &titleScreen{item: item, selectedSeason: 1}
	if got := window.visibleEpisodeIndices(); !reflect.DeepEqual(got, []int{3, 4}) {
		t.Fatalf("season 2 indices = %v", got)
	}
	window.selectedSeason = 0
	if got := window.visibleEpisodeIndices(); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("season 1 indices = %v", got)
	}

	// A title without seasons keeps the whole flat list.
	anime := &titleScreen{item: historyTitle{episodes: []string{"1", "2"}}}
	if got := anime.visibleEpisodeIndices(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("flat indices = %v", got)
	}
}

func TestSeasonIndexForEpisode(t *testing.T) {
	item := historyTitle{
		episodes: []string{"S1E1", "S1E2", "S2E1"},
		seasons: []titleSeason{
			{number: 1, episodes: []string{"S1E1", "S1E2"}},
			{number: 2, episodes: []string{"S2E1"}},
		},
	}
	if got := item.seasonIndexForEpisode(2); got != 1 {
		t.Fatalf("season index = %d", got)
	}
	if got := item.seasonIndexForEpisode(-1); got != 0 {
		t.Fatalf("out of range season index = %d", got)
	}
}
