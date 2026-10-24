package ui

import (
	"testing"

	"gioui.org/widget"

	"github.com/arfush/yonime/internal/library"
)

func TestVoiceMissingLabelCompressesRanges(t *testing.T) {
	tests := []struct {
		missing []string
		want    string
	}{
		{missing: nil, want: ""},
		{missing: []string{"7"}, want: "Нет серий: 7"},
		{missing: []string{"1", "2", "3", "4", "5"}, want: "Нет серий: 1-5"},
		{missing: []string{"2", "3", "5", "9"}, want: "Нет серий: 2-3, 5, 9"},
		{missing: []string{"5", "3", "4", "3", "1", "2"}, want: "Нет серий: 1-5"},
		{missing: []string{"1.5", "2", "SP1"}, want: "Нет серий: 1.5, 2, SP1"},
	}
	for _, test := range tests {
		if got := voiceMissingLabel(voiceover{missing: test.missing}); got != test.want {
			t.Fatalf("voiceMissingLabel(%v) = %q, want %q", test.missing, got, test.want)
		}
	}
}

func TestVoiceoverKodikFlagSurvivesConversion(t *testing.T) {
	title := titleFromLibrary(library.Title{
		Russian:  "Тайтл",
		Episodes: []string{"1", "2"},
		Voiceovers: []library.Voiceover{
			{Name: "Kodik", Episodes: []string{"1"}, Missing: []string{"2"}, HasKodik: true},
		},
	})
	if len(title.voices) != 1 || !title.voices[0].hasKodik {
		t.Fatalf("hasKodik was lost: %#v", title.voices)
	}
	back := libraryTitleFromHistory(title)
	if len(back.Voiceovers) != 1 || !back.Voiceovers[0].HasKodik {
		t.Fatalf("hasKodik was lost on the way back: %#v", back.Voiceovers)
	}
}

func TestMovieFlagSurvivesConversion(t *testing.T) {
	title := titleFromLibrary(library.Title{Russian: "Фильм", IsMovie: true})
	if !title.movie {
		t.Fatal("movie flag was lost in titleFromLibrary")
	}
	if back := libraryTitleFromHistory(title); !back.IsMovie {
		t.Fatal("movie flag was lost on the way back")
	}
}

func TestEpisodeRangeLabel(t *testing.T) {
	tests := []struct {
		episodes []string
		want     string
	}{
		{episodes: nil, want: "СЕРИИ"},
		{episodes: []string{"1", "2", "3"}, want: "СЕРИИ (1-3)"},
		{episodes: []string{"1", "3", "4"}, want: "СЕРИИ (3)"},
		{episodes: []string{"1.5", "2"}, want: "СЕРИИ (2)"},
	}
	for _, test := range tests {
		if got := episodeRangeLabel(test.episodes); got != test.want {
			t.Fatalf("episodeRangeLabel(%v) = %q, want %q", test.episodes, got, test.want)
		}
	}
}

func TestPosterInitialUsesFirstLetter(t *testing.T) {
	if got := posterInitial("провожающая в последний путь фрирен"); got != "П" {
		t.Fatalf("posterInitial = %q, want %q", got, "П")
	}
	if got := posterInitial("  "); got != "?" {
		t.Fatalf("posterInitial for spaces = %q, want %q", got, "?")
	}
}

func TestEpisodeProgressLabelAndValue(t *testing.T) {
	if got := episodeProgressLabel("2", 28); got != "2 / 28 эп." {
		t.Fatalf("episodeProgressLabel = %q", got)
	}
	if got := episodeProgressLabel("", 28); got != "" {
		t.Fatalf("label without an episode = %q, want empty", got)
	}
	if got := episodeProgressLabel("2", 0); got != "" {
		t.Fatalf("label without a total = %q, want empty", got)
	}
	if got := episodeProgressValue(300, 1200); got < 0.24 || got > 0.26 {
		t.Fatalf("progress = %v, want 0.25", got)
	}
	if got := episodeProgressValue(0, 1200); got != 0 {
		t.Fatalf("unwatched progress = %v, want 0", got)
	}
	if got := episodeProgressValue(1200, 0); got != 0 {
		t.Fatalf("unknown duration progress = %v, want 0", got)
	}
	if got := episodeProgressValue(1800, 1200); got != 1 {
		t.Fatalf("progress past the end = %v, want 1", got)
	}
}

func TestHistoryTitleEpisodeProgressSource(t *testing.T) {
	loaded := historyTitle{
		episodes:        []string{"1", "2", "3"},
		episodesTotal:   12,
		selectedEpisode: 1,
		episodeLabel:    "1",
	}
	if got := loaded.totalEpisodes(); got != 3 {
		t.Fatalf("loaded total = %d, want the fresh list length", got)
	}
	if got := loaded.currentEpisodeLabel(); got != "2" {
		t.Fatalf("loaded current = %q, want 2", got)
	}

	cached := historyTitle{episodesTotal: 28, episodeLabel: "5", selectedEpisode: -1}
	if got := cached.totalEpisodes(); got != 28 {
		t.Fatalf("cached total = %d, want 28", got)
	}
	if got := cached.currentEpisodeLabel(); got != "5" {
		t.Fatalf("cached current = %q, want 5", got)
	}
}

func TestMovieHidesEpisodeProgressLabel(t *testing.T) {
	film := historyTitle{
		movie:           true,
		episodes:        []string{"1"},
		episodesTotal:   1,
		selectedEpisode: 0,
	}
	if got := film.progressLabel(); got != "" {
		t.Fatalf("film progress label = %q, want empty", got)
	}
	series := historyTitle{
		episodes:        []string{"1", "2", "3"},
		episodesTotal:   3,
		selectedEpisode: 1,
	}
	if got := series.progressLabel(); got != "2 / 3 эп." {
		t.Fatalf("series progress label = %q, want 2 / 3 эп.", got)
	}
}

func TestDeleteEditorWord(t *testing.T) {
	var editor widget.Editor
	editor.SetText("hello brave world")
	editor.SetCaret(17, 17)
	deleteEditorWord(&editor, true)
	if got := editor.Text(); got != "hello brave " {
		t.Fatalf("backward deletion = %q, want %q", got, "hello brave ")
	}

	editor.SetText("hello brave world")
	editor.SetCaret(6, 6)
	deleteEditorWord(&editor, false)
	if got := editor.Text(); got != "hello  world" {
		t.Fatalf("forward deletion = %q, want %q", got, "hello  world")
	}

	editor.SetText("hello brave world")
	editor.SetCaret(0, 5)
	deleteEditorWord(&editor, true)
	if got := editor.Text(); got != " brave world" {
		t.Fatalf("selection deletion = %q, want %q", got, " brave world")
	}
}
