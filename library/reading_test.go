package library

import (
	"testing"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/libsocial"
)

func TestTitleFromReadingDataGroupsByTranslation(t *testing.T) {
	data := &libsocial.ReadingTitleData{
		SiteID:   libsocial.SiteMangaLib,
		ID:       42,
		Name:     "Test Manga",
		SlugURL:  "42--test-manga",
		StatusID: 1,
		Chapters: []libsocial.ReadingChapterData{
			{Volume: "1", Number: "0", Translation: "Team A", BranchID: 101},
			{Volume: "1", Number: "0", Translation: "Team B", BranchID: 201},
			{Volume: "1", Number: "0.5", Translation: "Team B", BranchID: 202},
			{Volume: "1", Number: "1", Translation: "Team B", BranchID: 203},
		},
	}
	title := titleFromReadingData(data)
	if title.Source != catalog.SourceMangaLib {
		t.Fatalf("source = %q, want mangalib", title.Source)
	}
	if len(title.Episodes) != 3 {
		t.Fatalf("episodes = %v, want 3 unique keys", title.Episodes)
	}
	if len(title.Voiceovers) != 2 {
		t.Fatalf("voiceovers = %d, want 2", len(title.Voiceovers))
	}
	// The team with more chapters leads.
	if title.Voiceovers[0].Name != "Team B" {
		t.Fatalf("first voice = %q, want Team B (3 chapters)", title.Voiceovers[0].Name)
	}
	if got := len(title.Voiceovers[0].Episodes); got != 3 {
		t.Fatalf("Team B episodes = %d, want 3", got)
	}
	if got := len(title.Voiceovers[1].Missing); got != 2 {
		t.Fatalf("Team A missing = %d, want 2 (chapters 0.5 and 1)", got)
	}
	chapter, ok := findReadingChapter(title, "Team B", "1:1")
	if !ok || chapter.BranchID != 203 {
		t.Fatalf("Team B chapter 1:1 = %+v ok=%v, want branch 203", chapter, ok)
	}
	if _, ok := findReadingChapter(title, "Team A", "1:0.5"); ok {
		t.Fatal("Team A must not have chapter 0.5")
	}
}

func TestParseRanobeParagraphs(t *testing.T) {
	raw := `<p data-paragraph-index="0">Глава первая.</p>
<div class="block"><p>Второй &laquo;абзац&raquo; с <b>разметкой</b>.<br>И переносом.</p></div>
<p></p>
<p data-paragraph-index="2">Третий.</p>`
	got := parseRanobeParagraphs(raw)
	// The empty <p> between the blocks is kept as a blank-line scene break.
	want := []string{"Глава первая.", "Второй «абзац» с разметкой.\nИ переносом.", "", "Третий."}
	if len(got) != len(want) {
		t.Fatalf("paragraphs = %#v, want %d", got, len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("paragraph %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestParseRanobeParagraphsFallsBackWithoutPTags(t *testing.T) {
	got := parseRanobeParagraphs(`<div>Строка один</div><div>Строка два</div>`)
	if len(got) != 1 || got[0] != "Строка один\nСтрока два" {
		t.Fatalf("fallback paragraphs = %#v", got)
	}
}

func TestReadingChapterKeysSortNumerically(t *testing.T) {
	keys := []string{"10:5", "2:0", "1:0", "10:1", "2:1", "1:10"}
	sortReadingChapterKeys(keys)
	want := []string{"1:0", "1:10", "2:0", "2:1", "10:1", "10:5"}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("sorted keys = %v, want %v", keys, want)
		}
	}
}
