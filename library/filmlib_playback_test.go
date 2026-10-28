package library

import (
	"reflect"
	"testing"

	"github.com/arfush/yo/filmlib"
)

// Golden coverage for the Filmlib playback plan invariants: a movie uses the
// flat "1" key, a series uses S{season}E{episode} keys in the title's display
// order without duplicates, and the selection walks forward like the AnimeLIB
// plan does. These checks do not touch the network.

func TestFilmlibMoviePlanKeysGolden(t *testing.T) {
	plan := &PlaybackPlan{
		Episodes: []PlaybackEpisode{
			{Number: "1"},
		},
	}
	if len(plan.Episodes) != 1 || plan.Episodes[0].Number != "1" {
		t.Fatalf("movie plan episodes = %+v, want a single flat \"1\" key", plan.Episodes)
	}
	if filmlibEpisodeIndex(plan.Episodes, "1") != 0 {
		t.Fatalf("movie episode \"1\" not found at index 0")
	}
	if filmlibEpisodeIndex(plan.Episodes, "S1E1") != -1 {
		t.Fatalf("movie plan must not accept season-prefixed keys")
	}
}

func TestFilmlibSeriesPlanKeysGolden(t *testing.T) {
	display := []string{"S1E1", "S1E2", "S2E1", "S1E1", "S3E1"}
	episodes := makeEpisodes(display)
	// Duplicated S1E1 must appear once, in display order.
	want := []string{"S1E1", "S1E2", "S2E1", "S3E1"}
	got := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		got = append(got, episode.Number)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series plan episodes = %v, want %v", got, want)
	}
}

func TestFilmlibSelectionWalksForwardGolden(t *testing.T) {
	display := []string{"S1E1", "S1E2", "S1E3", "S2E1"}
	episodes := makeEpisodes([]string{"S1E1", "S1E3", "S2E1"})

	if index := filmlibEpisodeIndex(episodes, "S1E2"); index != -1 {
		t.Fatalf("missing S1E2 resolved to index %d", index)
	}
	// Selected S1E2 is skipped; the next available is S1E3.
	if index := nextFilmlibIndex(display, episodes, "S1E2"); index != 1 {
		t.Fatalf("next after S1E2 = %d, want 1 (S1E3)", index)
	}
	// The last display key must not wrap around.
	if index := nextFilmlibIndex(display, episodes, "S2E1"); index != -1 {
		t.Fatalf("last episode wrapped around: %d", index)
	}
	// An unknown selection must not silently restart from the beginning:
	// the user may have picked a spoiler-free episode that vanished.
	if index := nextFilmlibIndex(display, episodes, "S9E1"); index != -1 {
		t.Fatalf("unknown selection must stop the plan, got %d", index)
	}
}

func TestMatchFilmlibVoiceGolden(t *testing.T) {
	voices := []filmlib.Voice{
		{ID: 1, Name: "HDrezka Studio. 18+"},
		{ID: 2, Name: "Дубляж"},
	}

	if voice, ok := matchFilmlibVoice(voices, "HDrezka Studio. 18+"); !ok || voice.ID != 1 {
		t.Fatalf("exact voice match failed: %+v, %v", voice, ok)
	}
	if voice, ok := matchFilmlibVoice(voices, "hdrezka studio. 18+"); !ok || voice.ID != 1 {
		t.Fatalf("case-insensitive voice match failed: %+v, %v", voice, ok)
	}
	if _, ok := matchFilmlibVoice(voices, "Missing"); ok {
		t.Fatal("unknown voice must not match")
	}
}

func makeEpisodes(keys []string) []PlaybackEpisode {
	episodes := make([]PlaybackEpisode, 0, len(keys))
	seen := make(map[string]bool)
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		episodes = append(episodes, PlaybackEpisode{Number: key})
	}
	return episodes
}
