package libsocial

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "https://animelib.org/anime/18076--tonikaku-kawaii-sns-anime", want: "18076--tonikaku-kawaii-sns-anime", ok: true},
		{input: "https://v5.animelib.org/ru/anime/12--one-piece-anime?foo=1", want: "12--one-piece-anime", ok: true},
		{input: "https://animelib.org/anime/12--one-piece-anime/episode/3", want: "12--one-piece-anime", ok: true},
		{input: "https://animelib.org/anime/12--one-piece-anime/", want: "12--one-piece-anime", ok: true},
		{input: "animelib.org/anime/12--one-piece-anime", want: "12--one-piece-anime", ok: true},
		{input: "https://animelib.mobi/anime/12--one-piece-anime", want: "12--one-piece-anime", ok: true},
		{input: "https://example.com/anime/foo", want: "foo", ok: true},
		{input: "https://example.com/title/foo", want: "foo", ok: true},
		{input: "12--one-piece-anime", want: "12--one-piece-anime", ok: true},
		{input: "", ok: false},
		{input: "https://example.com", ok: false},
		{input: "has spaces", ok: false},
	}
	for _, test := range tests {
		got, err := ParseInput(test.input)
		if test.ok && err != nil {
			t.Fatalf("ParseInput(%q) returned error: %v", test.input, err)
		}
		if !test.ok && err == nil {
			t.Fatalf("ParseInput(%q) succeeded, want error", test.input)
		}
		if test.ok && got != test.want {
			t.Fatalf("ParseInput(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestNormalizeEpisodeNumber(t *testing.T) {
	tests := map[string]string{
		"":     "",
		"01":   "1",
		"1.0":  "1",
		"1.5":  "1.5",
		" 42 ": "42",
		"SP1":  "SP1",
		"0":    "0",
	}
	for input, want := range tests {
		if got := NormalizeEpisodeNumber(input); got != want {
			t.Fatalf("NormalizeEpisodeNumber(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFetchLoadsAllEpisodesAndVoiceAvailabilityEveryCall(t *testing.T) {
	var listRequests, playerRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/anime/refresh-test":
			fmt.Fprint(w, `{"data":{"id":7,"name":"Refresh Test","rus_name":"Обновление","eng_name":"Refresh Test","slug":"refresh-test","slug_url":"refresh-test","status":{"id":1,"label":"Онгоинг"},"type":{"id":16,"label":"TV Сериал"},"cover":{"default":"https://example.com/cover.jpg"},"teams":[{"id":10,"name":"Team A"},{"id":11,"name":"Team K"}]}}`)
		case r.URL.Path == "/api/episodes":
			atomic.AddInt32(&listRequests, 1)
			if got := r.URL.Query().Get("anime_id"); got != "refresh-test" {
				http.Error(w, "bad anime_id: "+got, http.StatusBadRequest)
				return
			}
			// Episode 106 is the regression case: it must be available when
			// the API reports it for Team A.
			fmt.Fprint(w, `{"data":[{"id":101248,"number":"1"},{"id":101353,"number":"106"},{"id":101354,"number":"107"}]}`)
		case r.URL.Path == "/api/players/exists":
			atomic.AddInt32(&playerRequests, 1)
			query := r.URL.Query()
			if got := query.Get("anime_id"); got != "7" {
				http.Error(w, "bad anime_id: "+got, http.StatusBadRequest)
				return
			}
			if got := query.Get("translation_type"); got != "2" {
				http.Error(w, "bad translation_type: "+got, http.StatusBadRequest)
				return
			}
			switch query.Get("team_id") + "/" + query.Get("player") {
			case "10/Animelib":
				fmt.Fprint(w, `{"data":{"101248":1,"101353":1,"101354":1}}`)
			case "10/Kodik":
				fmt.Fprint(w, `{"data":{"101248":1}}`)
			case "11/Animelib":
				fmt.Fprint(w, `{"data":[]}`)
			case "11/Kodik":
				fmt.Fprint(w, `{"data":{"101353":1}}`)
			default:
				fmt.Fprint(w, `{"data":[]}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	first, err := client.Fetch(context.Background(), "refresh-test")
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&listRequests); got != 1 {
		t.Fatalf("first fetch made %d episode-list requests, want 1", got)
	}
	if got := atomic.LoadInt32(&playerRequests); got != 4 {
		t.Fatalf("first fetch made %d player requests, want 4", got)
	}
	if len(first.Episodes) != 3 || first.Episodes[1] != "106" {
		t.Fatalf("unexpected episodes: %#v", first.Episodes)
	}
	if first.TypeID != 16 || first.TypeLabel != "TV Сериал" || first.IsMovie() {
		t.Fatalf("unexpected title type: %#v", first.Title)
	}

	voices := make(map[string]Voiceover, len(first.Voiceovers))
	for _, voice := range first.Voiceovers {
		voices[voice.Name] = voice
	}
	teamA, ok := voices["Team A"]
	if !ok {
		t.Fatalf("Team A is missing: %#v", first.Voiceovers)
	}
	if !containsString(teamA.Episodes, "106") || containsString(teamA.Missing, "106") {
		t.Fatalf("episode 106 must be available for Team A: %#v", teamA)
	}
	if teamA.HasKodik {
		t.Fatalf("Team A has no Kodik-only episodes: %#v", teamA)
	}
	teamK, ok := voices["Team K"]
	if !ok {
		t.Fatalf("Kodik-only team is missing: %#v", first.Voiceovers)
	}
	if !containsString(teamK.Episodes, "106") {
		t.Fatalf("Kodik availability was not merged: %#v", teamK)
	}
	if !teamK.HasKodik {
		t.Fatalf("Team K plays exclusively via Kodik: %#v", teamK)
	}
	if !containsString(teamK.Missing, "1") || !containsString(teamK.Missing, "107") {
		t.Fatalf("unexpected missing episodes for Team K: %#v", teamK)
	}

	second, err := client.Fetch(context.Background(), "refresh-test")
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&listRequests); got != 2 {
		t.Fatalf("second fetch made %d episode-list requests, want 2", got)
	}
	if got := atomic.LoadInt32(&playerRequests); got != 8 {
		t.Fatalf("second fetch made %d player requests, want 8 (nothing may be cached)", got)
	}
	if len(second.Episodes) != 3 {
		t.Fatalf("second fetch returned %d episodes, want 3", len(second.Episodes))
	}
}

func TestFetchSkipsEpisodesForAnnouncedTitle(t *testing.T) {
	var listRequests, playerRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/anime/announced-test":
			fmt.Fprint(w, `{"data":{"id":9,"name":"Announced Test","rus_name":"Анонс","eng_name":"Announced Test","slug":"announced-test","slug_url":"announced-test","status":{"id":3,"label":"Анонс"},"cover":{"default":"https://example.com/cover.jpg"},"teams":[{"id":10,"name":"Team A"}]}}`)
		case r.URL.Path == "/api/episodes":
			atomic.AddInt32(&listRequests, 1)
			fmt.Fprint(w, `{"data":[]}`)
		case r.URL.Path == "/api/players/exists":
			atomic.AddInt32(&playerRequests, 1)
			fmt.Fprint(w, `{"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	data, err := client.Fetch(context.Background(), "announced-test")
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&listRequests); got != 0 {
		t.Fatalf("announced title requested the episode list %d times, want 0", got)
	}
	if got := atomic.LoadInt32(&playerRequests); got != 0 {
		t.Fatalf("announced title requested player availability %d times, want 0", got)
	}
	if len(data.Episodes) != 0 || len(data.Voiceovers) != 0 {
		t.Fatalf("announced title should have no episodes: %#v", data)
	}
}

func TestTitleIsMovie(t *testing.T) {
	tests := []struct {
		typeID int
		label  string
		want   bool
	}{
		{typeID: TypeMovie, label: "Фильм", want: true},
		{typeID: TypeMovie, want: true},
		{label: "Короткометражный фильм", want: true},
		{label: "Movie", want: true},
		{typeID: 16, label: "TV Сериал", want: false},
		{label: "OVA", want: false},
	}
	for _, test := range tests {
		title := Title{TypeID: test.typeID, TypeLabel: test.label}
		if got := title.IsMovie(); got != test.want {
			t.Fatalf("IsMovie(%d, %q) = %v, want %v", test.typeID, test.label, got, test.want)
		}
	}
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
