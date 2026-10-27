package ui

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
)

func readerTestContext() layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 545)),
	}
}

// TestReaderListsBalanceTheirOperationStacks guards the reader against the
// double Pop that made Gio panic with "unbalanced operation" as soon as a
// manga or ranobe chapter was shown.
func TestReaderListsBalanceTheirOperationStacks(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	r := &readerScreen{
		typo:     typo,
		chapters: []string{"1:1", "1:2"},
		filtered: []string{"1:1", "1:2"},
	}
	r.chapterClick = make([]widget.Clickable, len(r.chapters))

	gtx := readerTestContext()
	r.layoutChapterList(gtx, image.Rect(0, 0, 280, 400))
	gtx.Ops.Reset()

	r.pages = []readerPageState{{url: "https://example.test/1.png", loading: true}}
	r.layoutManga(gtx, image.Rect(0, 0, 500, 400))
	gtx.Ops.Reset()

	r.content.Paragraphs = []string{"first paragraph", "second paragraph"}
	r.layoutRanobe(gtx, image.Rect(0, 0, 500, 400))
}

type readerSourceStub struct {
	mu    sync.Mutex
	reads []string
}

func (s *readerSourceStub) ReadChapter(_ context.Context, _ catalog.Title, _ string, key string) (catalog.ReadingContent, error) {
	s.mu.Lock()
	s.reads = append(s.reads, key)
	s.mu.Unlock()
	return catalog.ReadingContent{Paragraphs: []string{"chapter " + key}}, nil
}

func (s *readerSourceStub) ReadingImage(context.Context, catalog.Title, string) (image.Image, error) {
	return nil, nil
}

func (s *readerSourceStub) TrackViewing(catalog.Title, string, string) (int64, error) {
	return 1, nil
}

func (s *readerSourceStub) SaveSelection(int64, string, string) error { return nil }

func (s *readerSourceStub) readKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.reads...)
}

func TestReaderPrefetchesFiveChaptersAroundSelection(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	chapters := make([]string, 20)
	for index := range chapters {
		chapters[index] = "1:" + itoaReaderTest(index+1)
	}
	source := &readerSourceStub{}
	item := historyTitle{
		id:       1,
		source:   catalog.SourceMangaLib,
		slug:     "test",
		title:    "Test",
		episodes: chapters,
		voices:   []voiceover{{name: "Team", episodes: chapters}},
	}
	title := catalog.Title{
		ID:              1,
		Source:          catalog.SourceMangaLib,
		Slug:            "test",
		Russian:         "Test",
		SelectedVoice:   "Team",
		SelectedEpisode: "1:10",
		Voiceovers: []catalog.Voiceover{{
			Name:     "Team",
			Episodes: chapters,
		}},
	}
	r := newReaderScreen(typo, item, source)
	r.prefill(title)

	deadline := time.Now().Add(3 * time.Second)
	for {
		gtx := readerTestContext()
		r.updateChapterResult(gtx)
		if len(r.chapterPending) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("prefetch did not settle, pending=%d", len(r.chapterPending))
		}
		time.Sleep(time.Millisecond)
	}

	// Current chapter plus five on each side. The first and last positions in
	// the title naturally have a smaller window.
	reads := source.readKeys()
	if len(reads) != readerChapterWindow*2+1 {
		t.Fatalf("prefetched %d chapters, want %d: %v", len(reads), readerChapterWindow*2+1, reads)
	}
	if r.loading {
		t.Fatal("reader stayed in the loading state after the current chapter arrived")
	}
	if r.currentChapterKey() != "1:10" {
		t.Fatalf("current chapter = %q, want 1:10", r.currentChapterKey())
	}

	// Moving to the next chapter is instant because it is already cached.
	r.selectChapter(10)
	if r.loading {
		t.Fatal("cached chapter opened with a spinner")
	}
	if len(r.content.Paragraphs) != 1 || r.content.Paragraphs[0] != "chapter 1:11" {
		t.Fatalf("cached content = %#v, want chapter 1:11", r.content.Paragraphs)
	}
}

func itoaReaderTest(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
