package ui

import (
	"testing"

	"github.com/arfush/yo/catalog"
)

func TestReadingTitleScreenTreeAndReadTransition(t *testing.T) {
	item := historyTitle{
		source:   catalog.SourceMangaLib,
		title:    "Title",
		episodes: []string{"1:1", "1:2", "2:1"},
		chapters: []catalog.ReadingChapter{
			{Key: "1:1", Volume: "1", Number: "1"},
			{Key: "1:2", Volume: "1", Number: "2"},
			{Key: "2:1", Volume: "2", Number: "1"},
		},
		voices: []voiceover{{
			name:     "Team",
			episodes: []string{"1:1", "2:1"},
		}},
		selectedVoice:   0,
		selectedEpisode: 0,
	}
	screen := newReadingTitleScreen(nil, item)
	if !screen.loaded {
		t.Fatal("a loaded title must not start another network request")
	}
	if got := screen.currentChapter(); got != "1:1" {
		t.Fatalf("selected chapter = %q, want 1:1", got)
	}
	foundMissing := false
	for _, row := range screen.treeRows {
		if row.chapter == "1:2" && row.missing {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatal("tree must show the chapter missing from the selected translation")
	}

	var gotVoice, gotChapter string
	screen.onRead = func(_ historyTitle, _ catalog.Title, voice, chapter string) {
		gotVoice, gotChapter = voice, chapter
	}
	screen.read()
	if gotVoice != "Team" || gotChapter != "1:1" {
		t.Fatalf("read transition = %q/%q, want Team/1:1", gotVoice, gotChapter)
	}

	screen.selectedChapter = 1
	screen.read()
	if gotChapter != "2:1" {
		t.Fatalf("read transition chapter = %q, want 2:1", gotChapter)
	}
}

func TestReadingTitleScreenVoiceSwitchKeepsNearestChapter(t *testing.T) {
	item := historyTitle{
		source:   catalog.SourceRanobeLib,
		title:    "Title",
		episodes: []string{"1:1", "1:2", "2:1"},
		voices: []voiceover{
			{name: "A", episodes: []string{"1:1", "2:1"}},
			{name: "B", episodes: []string{"1:1", "1:2", "2:1"}},
		},
		selectedVoice:   0,
		selectedEpisode: 0,
	}
	screen := newReadingTitleScreen(nil, item)
	screen.selectedChapter = 0
	screen.selectVoice(1)
	if got := screen.currentChapter(); got != "1:1" {
		t.Fatalf("chapter after voice switch = %q, want 1:1", got)
	}
}
