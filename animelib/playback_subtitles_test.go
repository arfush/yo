package animelib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEpisodeDetailKeepsSubtitlesOnTheirPlayer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/episodes/42" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{
			"data": {
				"id": 42,
				"number": "2",
				"season": "1",
				"name": "Episode 2",
				"players": [
					{
						"id": 1,
						"player": "Animelib",
						"translation_type": {"id": 2},
						"team": {"id": 10, "name": "Voice A"},
						"video_domain": "secondary_2",
						"subtitles": [
							{"id": 101, "format": "ASS", "name": "a", "filename": "a.ass", "src": "/uploads/subtitles/1/a.ass"},
							{"id": 102, "format": "vtt", "name": "b", "filename": "b.vtt", "src": "https://example.com/b.vtt"},
							{"id": 103, "format": "ass", "name": "empty", "filename": "", "src": " "},
							{"id": 104, "format": "vtt", "name": "d", "filename": "d.vtt", "src": "//api.cdnlibs.org/d.vtt"}
						],
						"video": {"quality": [{"href": "/video-a.mp4", "quality": "1080"}]}
					},
					{
						"id": 2,
						"player": "Animelib",
						"translation_type": {"id": 2},
						"team": {"id": 11, "name": "Voice B"},
						"video_domain": "main",
						"subtitles": [
							{"id": 201, "format": "ass", "name": "c", "filename": "c.ass", "src": "https://example.com/c.ass"}
						],
						"video": {"quality": [{"href": "/video-b.mp4", "quality": 720}]}
					},
					{
						"id": 3,
						"player": "Animelib",
						"translation_type": {"id": 1},
						"team": {"id": 12, "name": "Subtitle team"},
						"subtitles": [
							{"id": 301, "format": "ass", "name": "d", "filename": "d.ass", "src": "https://example.com/d.ass"}
						]
					}
				]
			}
		}`)
	}))
	defer server.Close()

	client := New("token")
	client.baseURL = server.URL
	client.http = server.Client()

	detail, err := client.EpisodeDetail(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Players) != 2 {
		t.Fatalf("players = %d, want 2 voiceover players: %#v", len(detail.Players), detail.Players)
	}

	voiceA := detail.Players[0]
	if voiceA.TeamName != "Voice A" || voiceA.VideoDomain != "secondary_2" {
		t.Fatalf("unexpected voice A: %#v", voiceA)
	}
	if len(voiceA.Subtitles) != 3 {
		t.Fatalf("voice A subtitles = %#v, want 3 non-empty tracks", voiceA.Subtitles)
	}
	if got := voiceA.Subtitles[0].Format; got != "ass" {
		t.Fatalf("format = %q, want lowercase ass", got)
	}
	if got := voiceA.Subtitles[0].Src; got != "/uploads/subtitles/1/a.ass" {
		t.Fatalf("relative src = %q", got)
	}
	if got := voiceA.Subtitles[2].Src; got != "https://api.cdnlibs.org/d.vtt" {
		t.Fatalf("protocol-relative src = %q", got)
	}

	voiceB := detail.Players[1]
	if voiceB.TeamName != "Voice B" {
		t.Fatalf("unexpected voice B: %#v", voiceB)
	}
	if len(voiceB.Subtitles) != 1 || voiceB.Subtitles[0].Src != "https://example.com/c.ass" {
		t.Fatalf("voice B subtitles must stay on their own player: %#v", voiceB.Subtitles)
	}
}
