package library

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/arfush/yoanime/animelib"
	"github.com/arfush/yoanime/store"
)

func openEpisodeTestLibrary(t *testing.T) *Library {
	t.Helper()
	dir := t.TempDir()
	lib, err := Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

func addEpisodeTestTitle(t *testing.T, lib *Library, title store.Title, episode, voice string) int64 {
	t.Helper()
	tid, err := lib.store.UpsertTitle(title)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.store.EnsureHistory(tid, episode, voice); err != nil {
		t.Fatal(err)
	}
	return tid
}

func episodeTestWatch(t *testing.T, lib *Library, tid int64) store.EpisodeWatch {
	t.Helper()
	watches, err := lib.store.ListEpisodeWatches()
	if err != nil {
		t.Fatal(err)
	}
	for _, watch := range watches {
		if watch.TitleID == tid {
			return watch
		}
	}
	t.Fatalf("title %d is missing from watches", tid)
	return store.EpisodeWatch{}
}

func ongoingEpisodeTitle(slug, russian string) store.Title {
	return store.Title{
		Slug:    slug,
		RusName: russian,
		Status:  int(StatusOngoing),
	}
}

func fixedEpisodeFetcher(release *animelib.EpisodeRelease, calls *int) episodeFetcher {
	return func(context.Context, string) (*animelib.EpisodeRelease, error) {
		if calls != nil {
			*calls++
		}
		return release, nil
	}
}

func TestCheckEpisodeNotificationsSeedsExistingHistorySilently(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("seed", "Сид"), "1", "Озвучка")

	release := &animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "5",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Озвучка"},
		},
	}
	lib.episodeFetcher = fixedEpisodeFetcher(release, nil)

	var events int
	err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		events++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("seed pass sent %d notifications, want 0", events)
	}
	watch := episodeTestWatch(t, lib, tid)
	if !watch.Tracked || watch.LastEpisode != "5" || watch.AnyVoiceNotified || watch.PreferredVoiceNotified {
		t.Fatalf("unexpected seeded tracking: %#v", watch)
	}
}

func TestCheckEpisodeNotificationsTwoStageDelivery(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("two-stage", "Две стадии"), "1", "Моя озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "1"); err != nil {
		t.Fatal(err)
	}

	var events []EpisodeNotification
	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "2",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Другая озвучка", KodikOnly: true},
		},
	}, nil)
	if err := lib.CheckEpisodeNotifications(context.Background(), func(event EpisodeNotification) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("first pass sent %d notifications, want 1", len(events))
	}
	if events[0].Preferred || events[0].TitleReleased || len(events[0].Voiceovers) != 1 || !events[0].Voiceovers[0].KodikOnly {
		t.Fatalf("unexpected first notification: %#v", events[0])
	}
	watch := episodeTestWatch(t, lib, tid)
	if !watch.AnyVoiceNotified || watch.PreferredVoiceNotified || watch.LastEpisode != "2" {
		t.Fatalf("unexpected tracking after first notification: %#v", watch)
	}

	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "2",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Другая озвучка"},
			{Name: "Моя озвучка"},
		},
	}, nil)
	if err := lib.CheckEpisodeNotifications(context.Background(), func(event EpisodeNotification) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("second pass sent %d notifications, want 2 total", len(events))
	}
	second := events[1]
	if !second.Preferred || len(second.Voiceovers) != 1 || second.Voiceovers[0].Name != "Моя озвучка" {
		t.Fatalf("unexpected preferred notification: %#v", second)
	}
	watch = episodeTestWatch(t, lib, tid)
	if !watch.AnyVoiceNotified || !watch.PreferredVoiceNotified || watch.LastEpisode != "2" {
		t.Fatalf("unexpected tracking after preferred notification: %#v", watch)
	}

	if err := lib.CheckEpisodeNotifications(context.Background(), func(event EpisodeNotification) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("repeat pass sent another notification: %#v", events)
	}
}

func TestCheckEpisodeNotificationsIgnoresOlderEpisode(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("rewind", "Назад"), "5", "Озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "5"); err != nil {
		t.Fatal(err)
	}

	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "4",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Озвучка"},
		},
	}, nil)
	events := 0
	if err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		events++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("older episode sent %d notifications", events)
	}
	if watch := episodeTestWatch(t, lib, tid); watch.LastEpisode != "5" {
		t.Fatalf("tracking rewound to %q", watch.LastEpisode)
	}
}

func TestCheckEpisodeNotificationsPreferredVoiceInFirstRelease(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("preferred-first", "Сразу моя"), "1", "Моя озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "1"); err != nil {
		t.Fatal(err)
	}

	var events []EpisodeNotification
	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "2",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Моя озвучка"},
			{Name: "Другая озвучка"},
		},
	}, nil)
	for pass := 0; pass < 2; pass++ {
		if err := lib.CheckEpisodeNotifications(context.Background(), func(event EpisodeNotification) error {
			events = append(events, event)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(events) != 1 {
		t.Fatalf("preferred-first title sent %d notifications, want 1", len(events))
	}
	if events[0].Preferred {
		t.Fatalf("first notification marked as preferred follow-up: %#v", events[0])
	}
	watch := episodeTestWatch(t, lib, tid)
	if !watch.AnyVoiceNotified || !watch.PreferredVoiceNotified {
		t.Fatalf("preferred voice was not marked as covered: %#v", watch)
	}
}

func TestCheckEpisodeNotificationsAnnouncedTitle(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	title := store.Title{
		Slug:    "announced",
		RusName: "Анонс",
		Status:  int(StatusAnnounced),
	}
	tid := addEpisodeTestTitle(t, lib, title, "", "Моя озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, ""); err != nil {
		t.Fatal(err)
	}

	var events []EpisodeNotification
	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "1",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Моя озвучка"},
		},
	}, nil)
	if err := lib.CheckEpisodeNotifications(context.Background(), func(event EpisodeNotification) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].TitleReleased {
		t.Fatalf("announced release was not marked: %#v", events)
	}
	watch := episodeTestWatch(t, lib, tid)
	if watch.Status != int(StatusOngoing) {
		t.Fatalf("cached status = %d, want ongoing", watch.Status)
	}
}

func TestCheckEpisodeNotificationsWaitsForFirstVoiceover(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("waiting", "Ждём озвучку"), "1", "Моя озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "1"); err != nil {
		t.Fatal(err)
	}

	release := &animelib.EpisodeRelease{
		StatusID: animelib.StatusOngoing,
		Episode:  "2",
	}
	lib.episodeFetcher = fixedEpisodeFetcher(release, nil)
	events := 0
	if err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		events++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("flat-list-only episode sent %d notifications", events)
	}
	watch := episodeTestWatch(t, lib, tid)
	if watch.LastEpisode != "1" {
		t.Fatalf("tracking advanced without a publisher: %#v", watch)
	}

	release.Voiceovers = []animelib.ReleaseVoiceover{{Name: "Другая озвучка"}}
	if err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		events++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("published episode sent %d notifications, want 1", events)
	}
}

func TestCheckEpisodeNotificationsCompletedTitleDropsTracking(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("finished", "Завершён"), "1", "Озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "1"); err != nil {
		t.Fatal(err)
	}

	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID: animelib.StatusCompleted,
		Episode:  "12",
		Voiceovers: []animelib.ReleaseVoiceover{
			{Name: "Озвучка"},
		},
	}, nil)
	events := 0
	if err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		events++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("completed title sent %d notifications", events)
	}
	watch := episodeTestWatch(t, lib, tid)
	if watch.Tracked || watch.LastEpisode != "" {
		t.Fatalf("completed title kept tracking: %#v", watch)
	}
	if watch.Status != int(StatusCompleted) {
		t.Fatalf("completed status = %d, want 0", watch.Status)
	}
}

func TestCheckEpisodeNotificationsDisabledSkipsNetwork(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("disabled", "Выключено"), "1", "Озвучка")
	if err := lib.SetEpisodeNotificationsEnabled(false); err != nil {
		t.Fatal(err)
	}

	calls := 0
	lib.episodeFetcher = func(context.Context, string) (*animelib.EpisodeRelease, error) {
		calls++
		return nil, errors.New("must not be called")
	}
	if err := lib.CheckEpisodeNotifications(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("disabled checker made %d network calls", calls)
	}
}

func TestSaveSeedsEpisodeTracking(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	title := Title{
		Slug:     "save-seeds",
		Russian:  "Сид при сохранении",
		Status:   StatusOngoing,
		Episodes: []string{"1", "3", "2"},
		Voiceovers: []Voiceover{
			{Name: "Озвучка", Episodes: []string{"1", "2", "3"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "3")
	if err != nil {
		t.Fatal(err)
	}
	watch := episodeTestWatch(t, lib, saved.ID)
	if !watch.Tracked || watch.LastEpisode != "3" {
		t.Fatalf("save did not seed tracking: %#v", watch)
	}
}

func TestCheckEpisodeNotificationsUpdatesEpisodeTotal(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	tid := addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("total", "Итого"), "1", "Озвучка")
	if err := lib.store.SeedEpisodeTracking(tid, "1"); err != nil {
		t.Fatal(err)
	}
	lib.episodeFetcher = fixedEpisodeFetcher(&animelib.EpisodeRelease{
		StatusID:     animelib.StatusOngoing,
		Episode:      "2",
		EpisodeCount: 12,
		Voiceovers:   []animelib.ReleaseVoiceover{{Name: "Озвучка"}},
	}, nil)

	if err := lib.CheckEpisodeNotifications(context.Background(), func(EpisodeNotification) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].EpisodesTotal != 12 {
		t.Fatalf("cached episode total = %#v, want 12", history)
	}
}

func TestRefreshMissingEpisodeTotalsBackfillsHistory(t *testing.T) {
	lib := openEpisodeTestLibrary(t)
	addEpisodeTestTitle(t, lib, ongoingEpisodeTitle("backfill", "Заполнение"), "1", "Озвучка")

	calls := 0
	lib.episodeCountFetcher = func(context.Context, string) (int, error) {
		calls++
		return 24, nil
	}
	updates := 0
	if err := lib.RefreshMissingEpisodeTotals(context.Background(), func() { updates++ }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || updates != 1 {
		t.Fatalf("calls = %d, updates = %d, want 1/1", calls, updates)
	}
	history, err := lib.History()
	if err != nil {
		t.Fatal(err)
	}
	if history[0].EpisodesTotal != 24 {
		t.Fatalf("episode total = %d, want 24", history[0].EpisodesTotal)
	}

	// A second pass has nothing to fetch.
	if err := lib.RefreshMissingEpisodeTotals(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("backfill fetched cached totals again: %d calls", calls)
	}
}
