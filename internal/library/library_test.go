package library

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/arfush/yonime/internal/animelib"
	"github.com/arfush/yonime/internal/metadata"
	"github.com/arfush/yonime/internal/store"
)

func TestChoiceFallsBackToCompleteVoiceAndEpisode(t *testing.T) {
	title := Title{
		Episodes: []string{"1", "2", "3"},
		Voiceovers: []Voiceover{
			{Name: "Missing first", Missing: []string{"1", "2"}},
			{Name: "Complete", Missing: nil},
		},
	}
	voice, episode := chooseSelection(title, "Complete", "2")
	if voice != "Complete" || episode != "2" {
		t.Fatalf("got %q/%q, want Complete/2", voice, episode)
	}

	voice, episode = chooseSelection(title, "Unknown", "3")
	if voice != "Missing first" || episode != "3" {
		t.Fatalf("fallback got %q/%q, want Missing first/3", voice, episode)
	}
}

// TestAutoSkipSegmentsOptInAndPersist keeps the per-type switches working:
// every segment starts off, and each choice is stored by segment key.
func TestAutoSkipSegmentsOptInAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yonime.sqlite")
	lib, err := Open(path, filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	segments := lib.AutoSkipSegments()
	for _, segment := range metadata.Segments() {
		if segments[segment] {
			t.Fatalf("%s must be disabled by default: %v", segment, segments)
		}
	}
	if err := lib.SetAutoSkipSegment(metadata.SegmentIntro, true); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}

	lib, err = Open(path, filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	if !lib.AutoSkipSegments()[metadata.SegmentIntro] {
		t.Fatal("enabled intro did not survive reopen")
	}
}

// TestTrackViewingTracksUnseenTitles covers a watch-party guest: the title may
// be new for them and only known from the room, but it still has to land in
// their history with the host's voice and keep the progress mpv writes.
func TestTrackViewingTracksUnseenTitles(t *testing.T) {
	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	id, err := lib.TrackViewing(Title{Slug: "party-title", Russian: "Парти-тайтл"}, "AniLibria.TV", "5")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("tracking did not persist the title")
	}
	entry, err := lib.store.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("tracking did not create the history row")
	}
	if entry.Voiceover != "AniLibria.TV" || entry.Episode != "5" {
		t.Fatalf("history = %#v, want the host's voice and episode", entry)
	}

	if err := lib.SaveProgress(id, "5", 300, 1440); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.TrackViewing(Title{Slug: "party-title", Russian: "Парти-тайтл"}, "Studio Band", "6"); err != nil {
		t.Fatal(err)
	}
	episode, resumeAt, _ := lib.Progress(id)
	if resumeAt != 300 {
		t.Fatalf("resume = %v, want the saved 300", resumeAt)
	}
	if episode != "5" {
		t.Fatalf("episode = %q, tracking must not drop the saved episode", episode)
	}
	entry, err = lib.store.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Voiceover != "Studio Band" {
		t.Fatalf("voiceover = %q, want the newly watched one", entry.Voiceover)
	}
}

func TestCoverCacheFailureIsRetriedAndThenReused(t *testing.T) {
	var requests int32
	var failing atomic.Bool
	failing.Store(true)

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: 0x80, G: 0x40, B: 0xc0, A: 0xff})
		}
	}
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if failing.Load() {
			http.Error(w, "not yet", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData.Bytes())
	}))
	defer server.Close()

	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	tid, err := lib.store.UpsertTitle(store.Title{Slug: "cover-test"})
	if err != nil {
		t.Fatal(err)
	}
	coverURL := server.URL + "/cover.png"
	if _, err := lib.RefreshCover(context.Background(), tid, coverURL); err == nil {
		t.Fatal("failed download was reported as success")
	}
	cache, err := lib.store.GetCoverCache(tid)
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil || cache.FailedAt == 0 || cache.Filename != "" {
		t.Fatalf("failure marker missing: %#v", cache)
	}

	failing.Store(false)
	path, err := lib.RefreshCover(context.Background(), tid, coverURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cover file was not written: %v", err)
	}
	cache, err = lib.store.GetCoverCache(tid)
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil || cache.Filename == "" || cache.FailedAt != 0 {
		t.Fatalf("success was not recorded: %#v", cache)
	}

	before := atomic.LoadInt32(&requests)
	again, err := lib.RefreshCover(context.Background(), tid, coverURL)
	if err != nil {
		t.Fatal(err)
	}
	if again != path {
		t.Fatalf("cached path = %q, want %q", again, path)
	}
	if got := atomic.LoadInt32(&requests); got != before {
		t.Fatalf("cached cover made another HTTP request: %d -> %d", before, got)
	}
}

func TestDeleteHistoryRemovesCachedCover(t *testing.T) {
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
		}
	}
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData.Bytes())
	}))
	defer server.Close()

	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	tid, err := lib.store.UpsertTitle(store.Title{Slug: "delete-cover"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.store.EnsureHistory(tid, "1", "Озвучка"); err != nil {
		t.Fatal(err)
	}
	coverPath, err := lib.RefreshCover(context.Background(), tid, server.URL+"/cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(coverPath); err != nil {
		t.Fatalf("cover was not written: %v", err)
	}

	if err := lib.DeleteHistory(tid); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(coverPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cover file still exists after deletion: %v", err)
	}
	cache, err := lib.store.GetCoverCache(tid)
	if err != nil {
		t.Fatal(err)
	}
	if cache != nil {
		t.Fatalf("cover cache row still exists: %#v", cache)
	}
	history, err := lib.store.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history was not cleared: %#v", history)
	}

	// Undo is not modelled here, but deleting twice must stay harmless: the
	// second call has no cover file or cache row left to remove.
	if err := lib.DeleteHistory(tid); err != nil {
		t.Fatalf("repeated delete failed: %v", err)
	}
}

func TestPreviewCoverIsReusedAndAdoptedOnSave(t *testing.T) {
	var requests int32
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
		}
	}
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData.Bytes())
	}))
	defer server.Close()

	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	coverURL := server.URL + "/cover.png"

	// A title opened from the URL field has no local id yet, but the cover file
	// is already named after the URL and can be reused after Save.
	preview, err := lib.PreviewCover(context.Background(), coverURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(preview); err != nil {
		t.Fatalf("preview cover was not written: %v", err)
	}
	if filepath.Base(preview) != "cover.png" {
		t.Fatalf("preview filename = %q, want cover.png", filepath.Base(preview))
	}
	again, err := lib.PreviewCover(context.Background(), coverURL)
	if err != nil {
		t.Fatal(err)
	}
	if again != preview {
		t.Fatalf("second preview path = %q, want %q", again, preview)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("preview cover was downloaded %d times, want 1", got)
	}

	title := Title{
		Slug:      "preview-test",
		Russian:   "Превью",
		CoverURL:  coverURL,
		CoverPath: preview,
		Episodes:  []string{"1"},
		Voiceovers: []Voiceover{
			{Name: "Озвучка", Episodes: []string{"1"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "1")
	if err != nil {
		t.Fatal(err)
	}
	if saved.CoverPath != preview {
		t.Fatalf("saved cover path = %q, want the preview file %q", saved.CoverPath, preview)
	}
	if _, err := os.Stat(saved.CoverPath); err != nil {
		t.Fatalf("saved cover file is missing: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("saving re-downloaded the cover: %d requests", got)
	}

	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].CoverPath != saved.CoverPath {
		t.Fatalf("history cover = %#v, want %q", history, saved.CoverPath)
	}
}

func TestSavePersistsTitleAndSelectionOnce(t *testing.T) {
	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	title := Title{
		Slug:     "save-test",
		Russian:  "Сохранение",
		Original: "Save Test",
		Status:   StatusCompleted,
		Episodes: []string{"1", "2"},
		Voiceovers: []Voiceover{
			{Name: "Озвучка", Episodes: []string{"1", "2"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "2")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatal("saved title has no local id")
	}

	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history has %d titles, want 1", len(history))
	}
	if history[0].Slug != "save-test" || history[0].SelectedVoice != "Озвучка" || history[0].SelectedEpisode != "2" {
		t.Fatalf("unexpected saved history entry: %#v", history[0])
	}

	// Saving the same title again updates the entry instead of duplicating it.
	again, err := lib.Save(title, "Озвучка", "1")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != saved.ID {
		t.Fatalf("second save id = %d, want %d", again.ID, saved.ID)
	}
	history, err = lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history has %d titles after re-save, want 1", len(history))
	}
	if history[0].SelectedEpisode != "1" {
		t.Fatalf("selected episode = %q, want 1", history[0].SelectedEpisode)
	}

	// Saving is explicit: a status chosen in the title window is written even
	// though UpsertTitle keeps the stored status for background refreshes.
	title.UserStatus = UserStatusFavorite
	if _, err := lib.Save(title, "Озвучка", "1"); err != nil {
		t.Fatal(err)
	}
	history, err = lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].UserStatus != UserStatusFavorite {
		t.Fatalf("saved user status = %d, want %d", history[0].UserStatus, UserStatusFavorite)
	}
}

func TestAnnouncedTitleKeepsPlannedStatus(t *testing.T) {
	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	title := Title{
		Slug:    "announced-status",
		Russian: "Анонс",
		Status:  StatusAnnounced,
	}
	saved, err := lib.Save(title, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.UserStatus != UserStatusPlanned {
		t.Fatalf("announced status = %d, want planned", saved.UserStatus)
	}
	if err := lib.SetUserStatus(saved.ID, UserStatusWatching); err != nil {
		t.Fatal(err)
	}
	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].UserStatus != UserStatusPlanned {
		t.Fatalf("announced title changed to %d, want planned", history[0].UserStatus)
	}
}

func TestTrackViewingPromotesPlannedTitle(t *testing.T) {
	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	title := Title{
		Slug:     "promote-status",
		Russian:  "Промоушен",
		Status:   StatusOngoing,
		Episodes: []string{"1"},
		Voiceovers: []Voiceover{
			{Name: "Озвучка", Episodes: []string{"1"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "1")
	if err != nil {
		t.Fatal(err)
	}
	if saved.UserStatus != UserStatusPlanned {
		t.Fatalf("new status = %d, want planned", saved.UserStatus)
	}
	if _, err := lib.TrackViewing(title, "Озвучка", "1"); err != nil {
		t.Fatal(err)
	}
	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].UserStatus != UserStatusWatching {
		t.Fatalf("status after playback = %d, want watching", history[0].UserStatus)
	}
}

func TestCoverIsReplacedWhenURLFilenameChanges(t *testing.T) {
	var requests int32
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: 0x30, G: 0x70, B: 0xb0, A: 0xff})
		}
	}
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData.Bytes())
	}))
	defer server.Close()

	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yonime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	tid, err := lib.store.UpsertTitle(store.Title{Slug: "cover-replace"})
	if err != nil {
		t.Fatal(err)
	}

	first, err := lib.RefreshCover(context.Background(), tid, server.URL+"/first.png")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "first.png" {
		t.Fatalf("first cover filename = %q", filepath.Base(first))
	}

	second, err := lib.RefreshCover(context.Background(), tid, server.URL+"/second.png")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(second) != "second.png" {
		t.Fatalf("second cover filename = %q", filepath.Base(second))
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("old cover file was not removed: %v", err)
	}

	before := atomic.LoadInt32(&requests)
	again, err := lib.RefreshCover(context.Background(), tid, server.URL+"/second.png")
	if err != nil {
		t.Fatal(err)
	}
	if again != second {
		t.Fatalf("cached path = %q, want %q", again, second)
	}
	if got := atomic.LoadInt32(&requests); got != before {
		t.Fatalf("unchanged filename made another HTTP request: %d -> %d", before, got)
	}
}

func TestCoverNameUsesURLFilename(t *testing.T) {
	tests := []struct {
		url  string
		ext  string
		want string
	}{
		{"https://cover.imglib.info/uploads/anime/1/cover/abc.jpg", ".png", "abc.jpg"},
		{"https://cover.imglib.info/a/b/abc?size=big", ".jpg", "abc.jpg"},
		{"https://cover.imglib.info/", ".png", fallbackNamePrefix("https://cover.imglib.info/") + ".png"},
	}
	for _, test := range tests {
		if got := coverName(test.url, test.ext); got != test.want {
			t.Fatalf("coverName(%q, %q) = %q, want %q", test.url, test.ext, got, test.want)
		}
	}
}

func TestCoverNameMatchesURL(t *testing.T) {
	if !coverNameMatches("abc.jpg", "https://example.com/x/abc.jpg") {
		t.Fatal("exact filename should match")
	}
	if !coverNameMatches("abc.png", "https://example.com/x/abc") {
		t.Fatal("detected extension should match a URL without one")
	}
	if coverNameMatches("other.jpg", "https://example.com/x/abc.jpg") {
		t.Fatal("different filename should not match")
	}
}

func TestTitleFromDataDetectsMovie(t *testing.T) {
	film := &animelib.TitleData{Title: animelib.Title{TypeID: animelib.TypeMovie, TypeLabel: "Фильм"}}
	title := titleFromData(store.Title{}, film)
	if !title.IsMovie {
		t.Fatal("film type was not propagated to the library title")
	}
	if stored := storedTitleFromData(film); !stored.IsMovie {
		t.Fatal("film type was not persisted")
	}
	if restored := titleFromData(store.Title{IsMovie: true}, nil); !restored.IsMovie {
		t.Fatal("film type was not restored from storage")
	}
	series := &animelib.TitleData{Title: animelib.Title{TypeID: 16, TypeLabel: "TV Сериал"}}
	if title := titleFromData(store.Title{}, series); title.IsMovie {
		t.Fatal("TV series was detected as a film")
	}
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		id    int
		label string
		want  Status
	}{
		{id: 1, want: StatusOngoing},
		{id: 2, want: StatusCompleted},
		{id: 3, want: StatusAnnounced},
		{id: 0, label: "Онгоинг", want: StatusOngoing},
		{id: 0, label: "Завершён", want: StatusCompleted},
		{id: 0, label: "Анонс", want: StatusAnnounced},
	}
	for _, test := range tests {
		if got := statusFrom(test.id, test.label); got != test.want {
			t.Fatalf("statusFrom(%d, %q) = %v, want %v", test.id, test.label, got, test.want)
		}
	}
}
