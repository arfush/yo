package ui

import (
	"reflect"
	"testing"
)

func TestEpisodesSectionLabelUsesSeasons(t *testing.T) {
	series := historyTitle{
		episodes: []string{"S1E1", "S2E1"},
		seasons: []titleSeason{
			{number: 1, episodes: []string{"S1E1"}},
			{number: 2, episodes: []string{"S2E1"}},
		},
	}
	if got := series.episodesSectionLabel(); got != "СЕЗОНЫ (1-2)" {
		t.Fatalf("season label = %q", got)
	}
	anime := historyTitle{episodes: []string{"1", "2", "3"}}
	if got := anime.episodesSectionLabel(); got != "СЕРИИ (1-3)" {
		t.Fatalf("anime label = %q", got)
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
	window := &titleWindow{item: item, selectedSeason: 1}
	if got := window.visibleEpisodeIndices(); !reflect.DeepEqual(got, []int{3, 4}) {
		t.Fatalf("season 2 indices = %v", got)
	}
	window.selectedSeason = 0
	if got := window.visibleEpisodeIndices(); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("season 1 indices = %v", got)
	}

	// A title without seasons keeps the whole flat list.
	anime := &titleWindow{item: historyTitle{episodes: []string{"1", "2"}}}
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
