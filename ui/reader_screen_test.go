package ui

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

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
	}

	gtx := readerTestContext()
	r.pages = []readerPageState{{url: "https://example.test/1.png", loading: true}}
	r.layoutManga(gtx, image.Rect(0, 0, 500, 400))
	gtx.Ops.Reset()

	r.content.Paragraphs = []string{"first paragraph", "second paragraph"}
	r.layoutRanobe(gtx, image.Rect(0, 0, 500, 400))
}

func TestReaderMangaRowsReserveChunkCount(t *testing.T) {
	r := &readerScreen{pages: []readerPageState{{
		width: 760, height: readerMaxTextureSide*2 + 100,
	}}}
	r.rebuildMangaRows()
	if got := len(r.mangaRows); got != 3 {
		t.Fatalf("rows before decode = %d, want 3", got)
	}
	r.pages[0].chunks = splitReaderImage(image.NewNRGBA(image.Rect(0, 0, 760, readerMaxTextureSide*2+100)), readerMaxTextureSide)
	r.rebuildMangaRows()
	if got := len(r.mangaRows); got != 3 {
		t.Fatalf("rows after decode = %d, want 3", got)
	}
}

func TestSplitReaderImageKeepsTexturesUnderLimit(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 9000))
	chunks := splitReaderImage(src, readerMaxTextureSide)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	total := 0
	for index, chunk := range chunks {
		bounds := chunk.Bounds()
		if bounds.Dx() != 100 || bounds.Dy() > readerMaxTextureSide {
			t.Fatalf("chunk %d bounds = %v", index, bounds)
		}
		total += bounds.Dy()
	}
	if total != 9000 {
		t.Fatalf("chunk heights total %d, want 9000", total)
	}
}

func TestReaderRequestFrameCallsInvalidator(t *testing.T) {
	calls := 0
	r := &readerScreen{invalidate: func() { calls++ }}
	r.requestFrame()
	if calls != 1 {
		t.Fatalf("invalidate calls = %d, want 1", calls)
	}
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

type blockingTrackSource struct {
	readerSourceStub
	started chan struct{}
	release chan struct{}
}

func (s *blockingTrackSource) TrackViewing(catalog.Title, string, string) (int64, error) {
	close(s.started)
	<-s.release
	return 1, nil
}

func TestReaderPrefillDoesNotBlockOnTracking(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	source := &blockingTrackSource{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	chapters := []string{"1:1", "1:2"}
	item := historyTitle{
		source:   catalog.SourceMangaLib,
		slug:     "test",
		title:    "Test",
		episodes: chapters,
		voices:   []voiceover{{name: "Team", episodes: chapters}},
	}
	title := catalog.Title{
		Source:          catalog.SourceMangaLib,
		Slug:            "test",
		Russian:         "Test",
		SelectedVoice:   "Team",
		SelectedEpisode: "1:1",
		Voiceovers: []catalog.Voiceover{{
			Name:     "Team",
			Episodes: chapters,
		}},
	}

	done := make(chan struct{})
	go func() {
		r := newReaderScreen(typo, item, source)
		r.prefill(title)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("prefill blocked on TrackViewing")
	}
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("TrackViewing was not started in the background")
	}
	close(source.release)
}

func itoaReaderTest(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

// countingChapterSource blocks every ReadChapter until release is closed and
// records the highest number of chapters fetched at the same time.
type countingChapterSource struct {
	readerSourceStub
	release   chan struct{}
	active    atomic.Int32
	maxActive atomic.Int32
}

func (s *countingChapterSource) ReadChapter(_ context.Context, _ catalog.Title, _ string, key string) (catalog.ReadingContent, error) {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		seen := s.maxActive.Load()
		if active <= seen || s.maxActive.CompareAndSwap(seen, active) {
			break
		}
	}
	<-s.release
	return catalog.ReadingContent{Paragraphs: []string{"chapter " + key}}, nil
}

// TestReaderChapterPrefetchBoundedBySemaphore makes sure the prefetch window
// does not burst the library API: at most chapterFetchConcurrency chapter
// requests may be in flight at once, no matter how wide the window is.
func TestReaderChapterPrefetchBoundedBySemaphore(t *testing.T) {
	source := &countingChapterSource{release: make(chan struct{})}
	r := &readerScreen{
		source:         source,
		chapterSem:     make(chan struct{}, chapterFetchConcurrency),
		chapterResults: make(chan readerChapterResult, 256),
		chapterCache:   make(map[string]catalog.ReadingContent),
		chapterPending: make(map[string]bool),
		contentCtx:     context.Background(),
	}
	for index := 0; index < readerChapterWindow*2+1; index++ {
		key := "1:" + itoaReaderTest(index+1)
		r.requestChapter("Team", key, catalog.Title{})
	}
	time.Sleep(300 * time.Millisecond)
	maxActive := source.maxActive.Load()
	close(source.release)
	time.Sleep(300 * time.Millisecond)
	if maxActive > chapterFetchConcurrency {
		t.Fatalf("max concurrent chapter fetches = %d, want <= %d", maxActive, chapterFetchConcurrency)
	}
	if maxActive == 0 {
		t.Fatal("no chapter fetch started")
	}
}
