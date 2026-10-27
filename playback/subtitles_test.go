package playback

import (
	"context"
	"testing"

	"github.com/arfush/yo/libsocial"
)

func TestPickSubtitleTrackPrefersASS(t *testing.T) {
	tests := []struct {
		name   string
		tracks []libsocial.SubtitleTrack
		want   string
		ok     bool
	}{
		{
			name: "ass wins even when vtt comes first",
			tracks: []libsocial.SubtitleTrack{
				{Format: "vtt", Src: "https://example.com/sub.vtt"},
				{Format: "ASS", Src: "https://example.com/sub.ass"},
			},
			want: "https://example.com/sub.ass",
			ok:   true,
		},
		{
			name: "vtt is the fallback",
			tracks: []libsocial.SubtitleTrack{
				{Format: "vtt", Src: "https://example.com/sub.vtt"},
				{Format: "srt", Src: "https://example.com/sub.srt"},
			},
			want: "https://example.com/sub.vtt",
			ok:   true,
		},
		{
			name: "other formats are ignored",
			tracks: []libsocial.SubtitleTrack{
				{Format: "srt", Src: "https://example.com/sub.srt"},
			},
		},
		{
			name: "tracks without a source are ignored",
			tracks: []libsocial.SubtitleTrack{
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
	player := &libsocial.EpisodePlayer{
		VideoDomain: "secondary_2",
		Subtitles: []libsocial.SubtitleTrack{
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

	absolute := &libsocial.EpisodePlayer{
		Subtitles: []libsocial.SubtitleTrack{
			{Format: "vtt", Src: "https://api.cdnlibs.org/a.vtt"},
		},
	}
	url, label = (&Session{}).subtitleForPlayer(context.Background(), absolute)
	if url != "https://api.cdnlibs.org/a.vtt" || label != "VTT" {
		t.Fatalf("absolute subtitle = %q (%s), want vtt URL", url, label)
	}
}

func TestSelectVoicePlayersDoesNotLeakSubtitlesAcrossVoices(t *testing.T) {
	players := []libsocial.EpisodePlayer{
		{
			ID:        1,
			Player:    "Animelib",
			TeamName:  "Voice A",
			Qualities: []libsocial.QualityLink{{Quality: 1080, Href: "/a.mp4"}},
			Subtitles: []libsocial.SubtitleTrack{
				{Format: "ass", Src: "https://example.com/a.ass"},
			},
		},
		{
			ID:        2,
			Player:    "Animelib",
			TeamName:  "Voice B",
			Qualities: []libsocial.QualityLink{{Quality: 1080, Href: "/b.mp4"}},
			Subtitles: []libsocial.SubtitleTrack{
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

	libsocialPlayer, kodikPlayer := selectVoicePlayers(players, "Voice A")
	if libsocialPlayer == nil || libsocialPlayer.ID != 1 {
		t.Fatalf("libsocial player = %#v, want Voice A player 1", libsocialPlayer)
	}
	if kodikPlayer == nil || kodikPlayer.ID != 3 {
		t.Fatalf("kodik player = %#v, want Voice A player 3", kodikPlayer)
	}
	if len(libsocialPlayer.Subtitles) != 1 || libsocialPlayer.Subtitles[0].Src != "https://example.com/a.ass" {
		t.Fatalf("selected player must keep only its own subtitles: %#v", libsocialPlayer.Subtitles)
	}

	libsocialPlayer, _ = selectVoicePlayers(players, "Voice B")
	if libsocialPlayer == nil || libsocialPlayer.ID != 2 {
		t.Fatalf("libsocial player = %#v, want Voice B player 2", libsocialPlayer)
	}
	if len(libsocialPlayer.Subtitles) != 1 || libsocialPlayer.Subtitles[0].Src != "https://example.com/b.ass" {
		t.Fatalf("selected player must keep only its own subtitles: %#v", libsocialPlayer.Subtitles)
	}
}
