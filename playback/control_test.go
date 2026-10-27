package playback

import (
	"testing"

	"github.com/arfush/yoanime/library"
)

func TestResolveEntryIndex(t *testing.T) {
	episode := func(key string) library.PlaybackEpisode { return library.PlaybackEpisode{Number: key} }
	entries := []*playlistEntry{
		{Episode: episode("S1E1")},
		{Episode: episode("S1E2")},
		{Episode: episode("S1E3")},
	}
	if got := resolveEntryIndex(entries, "S1E2"); got != 1 {
		t.Fatalf("index = %d, want 1", got)
	}
	if got := resolveEntryIndex(entries, "S2E1"); got != -1 {
		t.Fatalf("unresolved episode index = %d, want -1", got)
	}
	if got := resolveEntryIndex(nil, "S1E1"); got != -1 {
		t.Fatalf("empty playlist index = %d, want -1", got)
	}
	if got := resolveEntryIndex([]*playlistEntry{{Episode: episode("S1E1")}, nil}, "S1E1"); got != 0 {
		t.Fatalf("index with nil entry = %d, want 0", got)
	}
}

func TestSessionMatches(t *testing.T) {
	if got := (*Session)(nil).Matches("kinopoisk:685246", "HDrezka Studio. 18+"); got {
		t.Fatal("nil session must not match anything")
	}
	noPlan := &Session{}
	if got := noPlan.Matches("kinopoisk:685246", "HDrezka Studio. 18+"); got {
		t.Fatal("session without a plan must not match")
	}
	session := &Session{plan: &library.PlaybackPlan{Slug: "kinopoisk:685246", Voice: "HDrezka Studio. 18+"}}
	if !session.Matches("kinopoisk:685246", "HDrezka Studio. 18+") {
		t.Fatal("exact slug and voice must match")
	}
	if session.Matches("kinopoisk:88198", "HDrezka Studio. 18+") {
		t.Fatal("different slug must not match")
	}
	if session.Matches("kinopoisk:685246", "TVShows") {
		t.Fatal("different voice must not match")
	}
	// A guest that only knows the slug (no voice) is accepted so the follow
	// jump still works when the mirror renamed the voice.
	if !session.Matches("kinopoisk:685246", "") {
		t.Fatal("empty voice must not block the match")
	}
}
