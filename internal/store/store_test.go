package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestStorePersistsTitlesAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yonime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{
		Slug:    "12--one-piece-anime",
		EngName: "ONE PIECE",
		RusName: "Ван-Пис",
		Status:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("empty title id")
	}
	entry, err := db.EnsureHistory(id, "1", "AniLibria.TV")
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("history entry was not created")
	}

	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("got %d history entries, want 1", len(history))
	}
	if history[0].RusName != "Ван-Пис" {
		t.Fatalf("history title was not decoded: %#v", history[0])
	}

	upserted, err := db.UpsertTitle(Title{Slug: "12--one-piece-anime", EpisodesTotal: 1100})
	if err != nil {
		t.Fatal(err)
	}
	if upserted != id {
		t.Fatalf("upsert id = %d, want %d", upserted, id)
	}
	history, err = db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].EpisodesTotal != 1100 {
		t.Fatalf("episode total after upsert = %d, want 1100", history[0].EpisodesTotal)
	}
	if err := db.UpdateEpisodesTotal(id, 1101); err != nil {
		t.Fatal(err)
	}
	history, err = db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].EpisodesTotal != 1101 {
		t.Fatalf("episode total = %d, want 1101", history[0].EpisodesTotal)
	}

	if err := db.SaveSelection(id, "2", "AniLibria.TV"); err != nil {
		t.Fatal(err)
	}
	saved, err := db.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Episode != "2" {
		t.Fatalf("saved history = %#v, want episode 2", saved)
	}

	if err := db.ReorderHistory([]int64{id}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteHistory(id); err != nil {
		t.Fatal(err)
	}
	history, err = db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history was not cleared: %#v", history)
	}
}

// TestTrackViewingKeepsProgress covers a title that is watched before it is
// saved: tracking has to create the row, and tracking it again must keep the
// saved position while following the new voiceover.
func TestTrackViewingKeepsProgress(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "42--tracked", RusName: "Отслеживаемый"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.TrackViewing(id, "3", "AniLibria.TV"); err != nil {
		t.Fatal(err)
	}
	entry, err := db.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("tracking did not create a history row")
	}
	if entry.Episode != "3" || entry.Voiceover != "AniLibria.TV" {
		t.Fatalf("tracked history = %#v, want episode 3 with AniLibria.TV", entry)
	}

	if err := db.SaveProgress(id, "3", 120, 1440); err != nil {
		t.Fatal(err)
	}
	// A watch-party guest keeps the host's voiceover without losing its own
	// saved position.
	if err := db.TrackViewing(id, "4", "Studio Band"); err != nil {
		t.Fatal(err)
	}
	entry, err = db.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Voiceover != "Studio Band" {
		t.Fatalf("voiceover = %q, want Studio Band", entry.Voiceover)
	}
	if entry.ResumeAt != 120 || entry.Duration != 1440 {
		t.Fatalf("position = %v/%v, want 120/1440", entry.ResumeAt, entry.Duration)
	}
	if entry.Episode != "3" {
		t.Fatalf("episode = %q, tracking must not rewrite it", entry.Episode)
	}
}

func TestMovieFlagIsPersistedInHistory(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "film", RusName: "Фильм", EpisodesTotal: 1, IsMovie: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureHistory(id, "1", "Озвучка"); err != nil {
		t.Fatal(err)
	}
	title, err := db.GetTitle("film")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || !title.IsMovie {
		t.Fatalf("title movie flag = %#v, want true", title)
	}
	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || !history[0].IsMovie {
		t.Fatalf("history movie flag was not loaded: %#v", history)
	}
}

func TestUpdateNamesPreservesEmptyValues(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "names-test", EngName: "Old", RusName: "Старый"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateNames(id, "", ""); err != nil {
		t.Fatal(err)
	}
	title, err := db.GetTitle("names-test")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || title.EngName != "Old" || title.RusName != "Старый" {
		t.Fatalf("empty update changed names: %#v", title)
	}

	if err := db.UpdateNames(id, "New", "Новый"); err != nil {
		t.Fatal(err)
	}
	title, err = db.GetTitle("names-test")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || title.EngName != "New" || title.RusName != "Новый" {
		t.Fatalf("names were not updated: %#v", title)
	}
}

func TestCoverCache(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "cache-test", RusName: "Кэш"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveCoverFailure(id); err != nil {
		t.Fatal(err)
	}
	cover, err := db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.FailedAt == 0 || cover.Filename != "" {
		t.Fatalf("failure was not recorded: %#v", cover)
	}
	if err := db.SaveCoverSuccess(id, "cover.jpg"); err != nil {
		t.Fatal(err)
	}
	cover, err = db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.Filename != "cover.jpg" || cover.FailedAt != 0 {
		t.Fatalf("success was not recorded: %#v", cover)
	}
	// A failed refresh must keep the previous file so history keeps showing the
	// old cover until a replacement has been downloaded.
	if err := db.SaveCoverFailure(id); err != nil {
		t.Fatal(err)
	}
	cover, err = db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.Filename != "cover.jpg" || cover.FailedAt == 0 {
		t.Fatalf("failure removed the cached filename: %#v", cover)
	}
	if err := db.DeleteCoverCache(id); err != nil {
		t.Fatal(err)
	}
	cover, err = db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover != nil {
		t.Fatalf("cover cache was not deleted: %#v", cover)
	}
	if err := db.DeleteCoverCache(id); err != nil {
		t.Fatalf("deleting a missing cover cache failed: %v", err)
	}
}

func TestEpisodeTrackingLifecycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tid, err := db.UpsertTitle(Title{Slug: "tracking", RusName: "Трекинг", Status: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureHistory(tid, "1", "Озвучка"); err != nil {
		t.Fatal(err)
	}

	watches, err := db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	if len(watches) != 1 || watches[0].Tracked {
		t.Fatalf("fresh history unexpectedly has tracking: %#v", watches)
	}

	if err := db.SeedEpisodeTracking(tid, "4"); err != nil {
		t.Fatal(err)
	}
	if err := db.SeedEpisodeTracking(tid, "9"); err != nil {
		t.Fatal(err)
	}
	watches, err = db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	if !watches[0].Tracked || watches[0].LastEpisode != "4" {
		t.Fatalf("seed was not preserved: %#v", watches[0])
	}

	if err := db.SaveEpisodeTracking(tid, EpisodeTracking{
		LastEpisode:            "5",
		AnyVoiceNotified:       true,
		PreferredVoiceNotified: true,
	}); err != nil {
		t.Fatal(err)
	}
	watches, err = db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	watch := watches[0]
	if watch.LastEpisode != "5" || !watch.AnyVoiceNotified || !watch.PreferredVoiceNotified {
		t.Fatalf("tracking flags were not saved: %#v", watch)
	}

	if err := db.UpdateTitleStatus(tid, 0); err != nil {
		t.Fatal(err)
	}
	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].Status != 0 {
		t.Fatalf("cached status = %d, want 0", history[0].Status)
	}

	// Removing the title from history must remove its notification state too.
	if err := db.DeleteHistory(tid); err != nil {
		t.Fatal(err)
	}
	watches, err = db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	if len(watches) != 0 {
		t.Fatalf("deleted history still has watches: %#v", watches)
	}
	if _, err := db.EnsureHistory(tid, "5", "Озвучка"); err != nil {
		t.Fatal(err)
	}
	watches, err = db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	if watches[0].Tracked {
		t.Fatalf("re-added title reused deleted tracking: %#v", watches[0])
	}
}

func TestClearHistoryDropsEpisodeTracking(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tid, err := db.UpsertTitle(Title{Slug: "clear-tracking", Status: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureHistory(tid, "2", "Озвучка"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveEpisodeTracking(tid, EpisodeTracking{LastEpisode: "2", AnyVoiceNotified: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	watches, err := db.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	if len(watches) != 0 {
		t.Fatalf("cleared history still has watches: %#v", watches)
	}
}

func TestEpisodeNotificationsDefaultOnAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yonime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !db.EpisodeNotificationsEnabled() {
		t.Fatal("notifications must be enabled by default")
	}
	if err := db.SetEpisodeNotificationsEnabled(false); err != nil {
		t.Fatal(err)
	}
	if db.EpisodeNotificationsEnabled() {
		t.Fatal("disabled notification setting was not stored")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if db.EpisodeNotificationsEnabled() {
		t.Fatal("disabled notification setting did not survive reopen")
	}
	if err := db.SetEpisodeNotificationsEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !db.EpisodeNotificationsEnabled() {
		t.Fatal("enabled notification setting was not stored")
	}
}

func TestUpdateSettingsDefaultOffAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yonime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if db.AutoUpdateEnabled() {
		t.Fatal("auto-update must be disabled by default")
	}
	if db.BetaUpdatesEnabled() {
		t.Fatal("beta updates must be disabled by default")
	}
	if err := db.SetAutoUpdateEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBetaUpdatesEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if !db.AutoUpdateEnabled() {
		t.Fatal("enabled auto-update did not survive reopen")
	}
	if !db.BetaUpdatesEnabled() {
		t.Fatal("enabled beta updates did not survive reopen")
	}
	if err := db.SetAutoUpdateEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := db.SetBetaUpdatesEnabled(false); err != nil {
		t.Fatal(err)
	}
	if db.AutoUpdateEnabled() || db.BetaUpdatesEnabled() {
		t.Fatal("disabled update settings were not stored")
	}
}

func TestSharedControlDefaultsOnAndProviderPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yonime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !db.SharedControlEnabled() {
		t.Fatal("shared control must be enabled by default")
	}
	if got := db.WatchPartyProvider(); got != "" {
		t.Fatalf("provider default = %q, want an empty auto choice", got)
	}
	if err := db.SetSharedControlEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := db.SetWatchPartyProvider("supabase"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if db.SharedControlEnabled() {
		t.Fatal("disabled shared control did not survive reopen")
	}
	if got := db.WatchPartyProvider(); got != "supabase" {
		t.Fatalf("stored provider = %q, want supabase", got)
	}
}

func TestHistoryKeepsNewestFirst(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var ids []int64
	for _, slug := range []string{"first", "second", "third"} {
		id, err := db.UpsertTitle(Title{Slug: slug, RusName: slug})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if _, err := db.EnsureHistory(id, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("got %d entries, want 3", len(history))
	}
	if history[0].RusName != "third" || history[2].RusName != "first" {
		t.Fatalf("unexpected order: %s, %s, %s", history[0].RusName, history[1].RusName, history[2].RusName)
	}
}

func TestOpenAddsEpisodesTotalToLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE titles (
		id INTEGER PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
		eng_name TEXT NOT NULL DEFAULT '',
		rus_name TEXT NOT NULL DEFAULT '',
		imdb_id TEXT NOT NULL DEFAULT '',
		season INTEGER NOT NULL DEFAULT 0,
		status INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO titles (slug, rus_name) VALUES ('legacy', 'Старое')"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	title, err := db.GetTitle("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || title.RusName != "Старое" || title.EpisodesTotal != 0 || title.IsMovie {
		t.Fatalf("legacy title = %#v", title)
	}
	if err := db.UpdateEpisodesTotal(title.ID, 13); err != nil {
		t.Fatal(err)
	}
	updated, err := db.GetTitle("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil || updated.EpisodesTotal != 13 {
		t.Fatalf("episode total = %#v, want 13", updated)
	}
}
