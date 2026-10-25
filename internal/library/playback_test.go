package library

import "testing"

func TestNextAvailableIndexForwardOnly(t *testing.T) {
	display := []string{"1", "2", "3"}
	episodes := []PlaybackEpisode{
		{Number: "1"},
		{Number: "2"},
		{Number: "3"},
	}

	if got := nextAvailableIndex(display, episodes, ""); got != 0 {
		t.Fatalf("empty selection should start at the first episode: got %d, want 0", got)
	}
	if got := nextAvailableIndex(display, episodes, "2"); got != 2 {
		t.Fatalf("selected episode should advance to the next one: got %d, want 2", got)
	}
	if got := nextAvailableIndex(display, episodes, "3"); got != -1 {
		t.Fatalf("last episode must not wrap around: got %d, want -1", got)
	}
	if got := nextAvailableIndex(display, episodes, "4"); got != -1 {
		t.Fatalf("episode after the end must not be selected: got %d, want -1", got)
	}
	if got := nextAvailableIndex(display, episodes, "0"); got != -1 {
		t.Fatalf("unknown episode must not restart at episode one: got %d, want -1", got)
	}
}

func TestNextAvailableIndexSkipsMissingEpisodes(t *testing.T) {
	display := []string{"1", "2", "3", "4"}
	episodes := []PlaybackEpisode{
		{Number: "1"},
		{Number: "3"},
		{Number: "4"},
	}

	if got := nextAvailableIndex(display, episodes, "2"); got != 1 {
		t.Fatalf("missing selected episode should move to the next available one: got %d, want 1", got)
	}
	if got := nextAvailableIndex(display, episodes, "3"); got != 2 {
		t.Fatalf("selected episode should advance to the next available one: got %d, want 2", got)
	}
	if got := nextAvailableIndex(display, episodes, "4"); got != -1 {
		t.Fatalf("last episode must not wrap around: got %d, want -1", got)
	}
}

func TestNextAvailableIndexUsesDisplayOrder(t *testing.T) {
	display := []string{"1", "1.5", "2"}
	episodes := []PlaybackEpisode{
		{Number: "1"},
		{Number: "1.5"},
	}

	if got := nextAvailableIndex(display, episodes, "1"); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
	if got := nextAvailableIndex(display, episodes, "1.5"); got != -1 {
		t.Fatalf("got %d, want -1", got)
	}
}

func TestIndexOfEpisode(t *testing.T) {
	episodes := []PlaybackEpisode{
		{Number: "1"},
		{Number: "2"},
		{Number: "3"},
	}

	if got := indexOfEpisode(episodes, "2"); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
	if got := indexOfEpisode(episodes, "02"); got != 1 {
		t.Fatalf("normalized number should match: got %d, want 1", got)
	}
	if got := indexOfEpisode(episodes, "4"); got != -1 {
		t.Fatalf("got %d, want -1", got)
	}
}

func TestPlanCompletesTitle(t *testing.T) {
	episodes := func(numbers ...string) []PlaybackEpisode {
		result := make([]PlaybackEpisode, 0, len(numbers))
		for _, number := range numbers {
			result = append(result, PlaybackEpisode{Number: number})
		}
		return result
	}
	tests := []struct {
		name     string
		title    Title
		episodes []PlaybackEpisode
		want     bool
	}{
		{
			name:     "completed series with the finale",
			title:    Title{Status: StatusCompleted, Episodes: []string{"1", "2", "3"}},
			episodes: episodes("1", "2", "3"),
			want:     true,
		},
		{
			name:     "ongoing series stays watching",
			title:    Title{Status: StatusOngoing, Episodes: []string{"1", "2"}},
			episodes: episodes("1", "2"),
			want:     false,
		},
		{
			name:     "completed series missing the finale",
			title:    Title{Status: StatusCompleted, Episodes: []string{"1", "2", "3"}},
			episodes: episodes("1", "2"),
			want:     false,
		},
		{
			name:     "completed movie",
			title:    Title{Status: StatusCompleted, IsMovie: true},
			episodes: episodes("1"),
			want:     true,
		},
		{
			name:     "announced title",
			title:    Title{Status: StatusAnnounced},
			episodes: episodes("1"),
			want:     false,
		},
		{
			name:     "no playable episodes",
			title:    Title{Status: StatusCompleted, Episodes: []string{"1"}},
			episodes: nil,
			want:     false,
		},
		{
			name:     "cached total confirms a complete voice",
			title:    Title{Status: StatusCompleted, EpisodesTotal: 3},
			episodes: episodes("1", "2", "3"),
			want:     true,
		},
		{
			name:     "cached total rejects an incomplete voice",
			title:    Title{Status: StatusCompleted, EpisodesTotal: 3},
			episodes: episodes("1", "2"),
			want:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := planCompletesTitle(test.title, test.episodes); got != test.want {
				t.Fatalf("planCompletesTitle() = %v, want %v", got, test.want)
			}
		})
	}
}
