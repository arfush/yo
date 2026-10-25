package playback

import (
	"context"
	"testing"
)

func TestPickVideoBaseUsesPlayerDomain(t *testing.T) {
	bases := []string{
		"https://video1.cdnlibs.org/.%D0%B0s/",
		"https://video2.cdnlibs.org/",
		"https://video1.imglib.info/",
	}

	tests := []struct {
		domain string
		want   string
	}{
		{"main", "https://video1.cdnlibs.org/.%D0%B0s/"},
		{"secondary_1", "https://video2.cdnlibs.org/"},
		{"secondary_2", "https://video1.imglib.info/"},
		{"", "https://video1.cdnlibs.org/.%D0%B0s/"},
		{"unknown", "https://video1.cdnlibs.org/.%D0%B0s/"},
	}
	for _, test := range tests {
		if got := pickVideoBase(bases, test.domain); got != test.want {
			t.Fatalf("pickVideoBase(%q) = %q, want %q", test.domain, got, test.want)
		}
	}

	if got := pickVideoBase(nil, "secondary_2"); got != "" {
		t.Fatalf("pickVideoBase without bases = %q, want empty", got)
	}
}

// TestJoinAnimelibUsesPlayerDomain guards the title that only plays from the
// secondary_2 host: joining its relative href to the first server returned 404
// and mpv never opened.
func TestJoinAnimelibUsesPlayerDomain(t *testing.T) {
	session := &Session{videoBases: []string{
		"https://video1.cdnlibs.org/.%D0%B0s/",
		"https://video2.cdnlibs.org/",
		"https://video1.imglib.info/",
	}}
	href := "/uploads/converted_videos/anime/20698/players/29031/29031_1080.mp4"

	got, err := session.joinAnimelib(context.Background(), href, "secondary_2")
	if err != nil {
		t.Fatalf("joinAnimelib: %v", err)
	}
	want := "https://video1.imglib.info/uploads/converted_videos/anime/20698/players/29031/29031_1080.mp4"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}

	got, err = session.joinAnimelib(context.Background(), href, "")
	if err != nil {
		t.Fatalf("joinAnimelib without a domain: %v", err)
	}
	want = "https://video1.cdnlibs.org/.%D0%B0s/uploads/converted_videos/anime/20698/players/29031/29031_1080.mp4"
	if got != want {
		t.Fatalf("fallback url = %q, want %q", got, want)
	}

	absolute := "https://cdn.example.com/video.mp4"
	got, err = session.joinAnimelib(context.Background(), absolute, "secondary_2")
	if err != nil || got != absolute {
		t.Fatalf("absolute url = %q (%v), want %q", got, err, absolute)
	}
}
