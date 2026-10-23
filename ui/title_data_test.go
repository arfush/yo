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
