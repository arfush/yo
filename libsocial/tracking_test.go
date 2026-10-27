package libsocial

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestEpisodeSelectsNewestAndDetectsKodikOnlyVoice(t *testing.T) {
	var detailRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/anime/12--show":
			_, _ = w.Write([]byte(`{"data":{"id":42,"slug_url":"12--show","status":{"id":1,"label":"Онгоинг"}}}`))
		case "/api/episodes":
			_, _ = w.Write([]byte(`{"data":[
				{"id":101,"number":"1"},
				{"id":103,"number":"10"},
				{"id":102,"number":"2"}
			]}`))
		case "/api/episodes/103":
			detailRequests++
			_, _ = w.Write([]byte(`{"data":{"id":103,"number":"10","players":[
				{"player":"Animelib","translation_type":{"id":2},"team":{"id":1,"name":"Озвучка A"}},
				{"player":"Kodik","translation_type":{"id":2},"team":{"id":1,"name":"Озвучка A"}},
				{"player":"Kodik","translation_type":{"id":2},"team":{"id":2,"name":"Озвучка B"}}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("test-token")
	client.baseURL = server.URL

	release, err := client.LatestEpisode(context.Background(), "12--show")
	if err != nil {
		t.Fatal(err)
	}
	if release.TitleID != 42 || release.Slug != "12--show" || release.StatusID != 1 {
		t.Fatalf("unexpected title part: %#v", release)
	}
	if release.Episode != "10" || release.EpisodeID != 103 {
		t.Fatalf("latest episode = %q/%d, want 10/103", release.Episode, release.EpisodeID)
	}
	if release.EpisodeCount != 3 {
		t.Fatalf("episode count = %d, want 3", release.EpisodeCount)
	}
	if detailRequests != 1 {
		t.Fatalf("detail requested %d times, want 1", detailRequests)
	}
	if len(release.Voiceovers) != 2 {
		t.Fatalf("got %d voiceovers, want 2: %#v", len(release.Voiceovers), release.Voiceovers)
	}
	if release.Voiceovers[0].Name != "Озвучка A" || release.Voiceovers[0].KodikOnly {
		t.Fatalf("Animelib + Kodik voice was marked Kodik-only: %#v", release.Voiceovers[0])
	}
	if release.Voiceovers[1].Name != "Озвучка B" || !release.Voiceovers[1].KodikOnly {
		t.Fatalf("Kodik-only voice was not detected: %#v", release.Voiceovers[1])
	}
}

func TestLatestEpisodeWithoutEpisodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/anime/13--announced":
			_, _ = w.Write([]byte(`{"data":{"id":43,"slug_url":"13--announced","status":{"id":3,"label":"Анонс"}}}`))
		case "/api/episodes":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("test-token")
	client.baseURL = server.URL
	release, err := client.LatestEpisode(context.Background(), "13--announced")
	if err != nil {
		t.Fatal(err)
	}
	if release.Episode != "" || release.EpisodeID != 0 || len(release.Voiceovers) != 0 {
		t.Fatalf("announced title resolved an episode: %#v", release)
	}
	if release.StatusID != StatusAnnounced {
		t.Fatalf("status = %d, want announced", release.StatusID)
	}
}

func TestLatestEpisodeIgnoresItemsWithoutIDOrNumber(t *testing.T) {
	item, number, ok := latestEpisode([]EpisodeListItem{
		{ID: 0, Number: "99"},
		{ID: 1, Number: ""},
		{ID: 2, Number: "2"},
		{ID: 3, Number: "10"},
	})
	if !ok {
		t.Fatal("latest episode was not found")
	}
	if item.ID != 3 || number != "10" {
		t.Fatalf("got id=%d number=%q, want id=3 number=10", item.ID, number)
	}
}

func TestEpisodeNewer(t *testing.T) {
	tests := []struct {
		candidate string
		previous  string
		want      bool
	}{
		{"2", "1", true},
		{"10", "9", true},
		{"10.5", "10", true},
		{"1", "2", false},
		{"2", "2", false},
		{"1", "", true},
		{"", "1", false},
		{"SP", "10", true},
		{"10", "SP", false},
		{"OVA2", "OVA1", true},
	}
	for _, test := range tests {
		if got := EpisodeNewer(test.candidate, test.previous); got != test.want {
			t.Fatalf("EpisodeNewer(%q, %q) = %v, want %v", test.candidate, test.previous, got, test.want)
		}
	}
}

func TestReleaseVoiceoversMergesPlayersForOneTeam(t *testing.T) {
	voices := releaseVoiceovers([]EpisodePlayer{
		{Player: "Kodik", TeamName: "A"},
		{Player: "Animelib", TeamName: "A"},
		{Player: "Kodik", TeamName: "B"},
		{Player: "Animelib", TeamName: "B"},
		{Player: "Kodik", TeamName: "C"},
		{Player: "Other", TeamName: "Ignored"},
		{Player: "Kodik", TeamName: ""},
	})
	if len(voices) != 3 {
		t.Fatalf("got %d voices, want 3: %v", len(voices), fmt.Sprint(voices))
	}
	if voices[0].Name != "A" || voices[0].KodikOnly {
		t.Fatalf("team A = %#v, want normal", voices[0])
	}
	if voices[1].Name != "B" || voices[1].KodikOnly {
		t.Fatalf("team B = %#v, want normal", voices[1])
	}
	if voices[2].Name != "C" || !voices[2].KodikOnly {
		t.Fatalf("team C = %#v, want Kodik-only", voices[2])
	}
}
