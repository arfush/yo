package playback

import (
	"context"
	"testing"

	"github.com/arfush/yonime/internal/animelib"
)

func TestPickSubtitleTrackPrefersASS(t *testing.T) {
	tests := []struct {
		name   string
		tracks []animelib.SubtitleTrack
		want   string
		ok     bool
	}{
		{
			name: "ass wins even when vtt comes first",
			tracks: []animelib.SubtitleTrack{
				{Format: "vtt", Src: "https://example.com/sub.vtt"},
				{Format: "ASS", Src: "https://example.com/sub.ass"},
			},
			want: "https://example.com/sub.ass",
			ok:   true,
		},
		{
			name: "vtt is the fallback",
			tracks: []animelib.SubtitleTrack{
				{Format: "vtt", Src: "https://example.com/sub.vtt"},
				{Format: "srt", Src: "https://example.com/sub.srt"},
			},
			want: "https://example.com/sub.vtt",
			ok:   true,
		},
		{
			name: "other formats are ignored",
			tracks: []animelib.SubtitleTrack{
				{Format: "srt", Src: "https://example.com/sub.srt"},
			},
		},
		{
			name: "tracks without a source are ignored",
			tracks: []animelib.SubtitleTrack{
				{Format: "ass", Src: " "},
				{Format: "vtt", Src: "https://example.com/sub.vtt"},
			},
			want: "https://example.com/sub.vtt",
			ok:   true,
		},
		{
			name: "no tracks",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := pickSubtitleTrack(test.tracks)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
			if got.Src != test.want {
				t.Fatalf("src = %q, want %q", got.Src, test.want)
			}
		})
	}
}

func TestResolveRelativeSubtitleUsesPlayerVideoDomain(t *testing.T) {
	bases := []string{
		"https://video1.cdnlibs.org/.%D0%B0s/",
		"https://video2.cdnlibs.org/",
		"https://video1.imglib.info/",
	}

	tests := []struct {
		name   string
		src    string
		domain string
		want   string
	}{
		{
			name:   "absolute source is unchanged",
			src:    "https://api.cdnlibs.org/uploads/subtitles/1/a.ass",
			domain: "secondary_2",
			want:   "https://api.cdnlibs.org/uploads/subtitles/1/a.ass",
		},
		{
			name:   "main server",
			src:    "/uploads/subtitles/1/a.ass",
			domain: "main",
			want:   "https://video1.cdnlibs.org/.%D0%B0s/uploads/subtitles/1/a.ass",
		},
		{
			name:   "secondary server 1",
			src:    "uploads/subtitles/1/a.ass",
			domain: "secondary_1",
			want:   "https://video2.cdnlibs.org/uploads/subtitles/1/a.ass",
		},
		{
			name:   "secondary server 2",
			src:    "/uploads/subtitles/1/a.ass",
			domain: "secondary_2",
			want:   "https://video1.imglib.info/uploads/subtitles/1/a.ass",
		},
		{
			name:   "unknown domain falls back to the first base",
			src:    "/uploads/subtitles/1/a.ass",
			domain: "",
			want:   "https://video1.cdnlibs.org/.%D0%B0s/uploads/subtitles/1/a.ass",
		},
		{
			name:   "empty source",
			src:    " ",
			domain: "main",
			want:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveRelativeSubtitle(test.src, test.domain, bases); got != test.want {
				t.Fatalf("url = %q, want %q", got, test.want)
			}
		})
	}

	if got := resolveRelativeSubtitle("/a.ass", "main", nil); got != "" {
		t.Fatalf("url without bases = %q, want empty", got)
	}
}

func TestSubtitleForPlayerUsesASSAndSelectedServer(t *testing.T) {
	session := &Session{videoBases: []string{
		"https://video1.cdnlibs.org/.%D0%B0s/",
		"https://video2.cdnlibs.org/",
		"https://video1.imglib.info/",
	}}
	player := &animelib.EpisodePlayer{
		VideoDomain: "secondary_2",
		Subtitles: []animelib.SubtitleTrack{
			{Format: "vtt", Src: "/uploads/subtitles/1/a.vtt"},
			{Format: "ass", Src: "/uploads/subtitles/1/a.ass"},
		},
	}

	url, label := session.subtitleForPlayer(context.Background(), player)
	if url != "https://video1.imglib.info/uploads/subtitles/1/a.ass" {
		t.Fatalf("url = %q", url)
	}
	if label != "ASS" {
		t.Fatalf("label = %q, want ASS", label)
	}

	absolute := &animelib.EpisodePlayer{
		Subtitles: []animelib.SubtitleTrack{
			{Format: "vtt", Src: "https://api.cdnlibs.org/a.vtt"},
		},
	}
	url, label = (&Session{}).subtitleForPlayer(context.Background(), absolute)
	if url != "https://api.cdnlibs.org/a.vtt" || label != "VTT" {
		t.Fatalf("absolute subtitle = %q (%s), want vtt URL", url, label)
	}
}

func TestSelectVoicePlayersDoesNotLeakSubtitlesAcrossVoices(t *testing.T) {
	players := []animelib.EpisodePlayer{
		{
			ID:        1,
			Player:    "Animelib",
			TeamName:  "Voice A",
			Qualities: []animelib.QualityLink{{Quality: 1080, Href: "/a.mp4"}},
			Subtitles: []animelib.SubtitleTrack{
				{Format: "ass", Src: "https://example.com/a.ass"},
			},
		},
		{
			ID:        2,
			Player:    "Animelib",
			TeamName:  "Voice B",
			Qualities: []animelib.QualityLink{{Quality: 1080, Href: "/b.mp4"}},
			Subtitles: []animelib.SubtitleTrack{
				{Format: "ass", Src: "https://example.com/b.ass"},
			},
		},
		{
			ID:       3,
			Player:   "Kodik",
			TeamName: "Voice A",
			Src:      "//kodik.example/embed",
		},
	}

	animelibPlayer, kodikPlayer := selectVoicePlayers(players, "Voice A")
	if animelibPlayer == nil || animelibPlayer.ID != 1 {
		t.Fatalf("animelib player = %#v, want Voice A player 1", animelibPlayer)
	}
	if kodikPlayer == nil || kodikPlayer.ID != 3 {
		t.Fatalf("kodik player = %#v, want Voice A player 3", kodikPlayer)
	}
	if len(animelibPlayer.Subtitles) != 1 || animelibPlayer.Subtitles[0].Src != "https://example.com/a.ass" {
		t.Fatalf("selected player must keep only its own subtitles: %#v", animelibPlayer.Subtitles)
	}

	animelibPlayer, _ = selectVoicePlayers(players, "Voice B")
	if animelibPlayer == nil || animelibPlayer.ID != 2 {
		t.Fatalf("animelib player = %#v, want Voice B player 2", animelibPlayer)
	}
	if len(animelibPlayer.Subtitles) != 1 || animelibPlayer.Subtitles[0].Src != "https://example.com/b.ass" {
		t.Fatalf("selected player must keep only its own subtitles: %#v", animelibPlayer.Subtitles)
	}
}
