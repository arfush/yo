package playback

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/arfush/yo/library"
)

func TestUnopenedErrorDescribesKodik(t *testing.T) {
	err := unopenedError(&playlistEntry{
		Episode: library.PlaybackEpisode{Number: "7"},
		Kind:    "hls",
	}, "animelib")
	if !errors.Is(err, ErrKodikUnavailable) {
		t.Fatalf("error = %v, want ErrKodikUnavailable", err)
	}
}

func TestUnopenedEntryFallsBackToFirstEntry(t *testing.T) {
	first := &playlistEntry{
		Episode: library.PlaybackEpisode{Number: "S1E1"},
		Kind:    "hls",
	}
	session := &Session{
		entries: []*playlistEntry{first},
		current: 7,
	}
	if got := session.unopenedEntryLocked(7); got != first {
		t.Fatalf("fallback entry = %v, want the first episode", got)
	}
}

func TestUnopenedErrorDescribesFilmlibPlayer(t *testing.T) {
	err := unopenedError(&playlistEntry{
		Episode: library.PlaybackEpisode{Number: "S1E2"},
		Kind:    "hls",
	}, "filmlib")
	if !errors.Is(err, ErrVideoOpen) {
		t.Fatalf("error = %v, want ErrVideoOpen", err)
	}
}

func TestUnopenedErrorFallsBackToGenericText(t *testing.T) {
	err := unopenedError(nil, "")
	if !errors.Is(err, ErrPlaybackFailed) {
		t.Fatalf("error = %v, want ErrPlaybackFailed", err)
	}
}

func TestFinalEOFCompletesTitleOnlyWhenFlagged(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "yoanime.sqlite"), filepath.Join(dir, "token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lib.Close() })

	title := library.Title{
		Slug:     "playback-complete",
		Russian:  "Финальная серия",
		Status:   library.StatusCompleted,
		Episodes: []string{"1"},
		Voiceovers: []library.Voiceover{
			{Name: "Озвучка", Episodes: []string{"1"}},
		},
	}
	saved, err := lib.Save(title, "Озвучка", "1")
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.SetUserStatus(saved.ID, library.UserStatusWatching); err != nil {
		t.Fatal(err)
	}

	newSession := func(complete bool) *Session {
		return &Session{
			service: &Service{lib: lib},
			plan: &library.PlaybackPlan{
				TitleID:          saved.ID,
				Episodes:         []library.PlaybackEpisode{{Number: "1"}},
				CompleteOnFinish: complete,
			},
			entries: []*playlistEntry{{
				EpisodeIndex: 0,
				Episode:      library.PlaybackEpisode{Number: "1"},
			}},
		}
	}

	newSession(false).onEndFile(mpvEvent{Kind: mpvEventEndFile, Reason: "eof"})
	if got := lib.UserStatus(saved.ID); got != library.UserStatusWatching {
		t.Fatalf("status without the completion flag = %d, want watching", got)
	}

	newSession(true).onEndFile(mpvEvent{Kind: mpvEventEndFile, Reason: "eof"})
	if got := lib.UserStatus(saved.ID); got != library.UserStatusCompleted {
		t.Fatalf("status after the final EOF = %d, want completed", got)
	}
}
