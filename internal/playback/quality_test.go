package playback

import (
	"testing"

	"github.com/arfush/yoanime/internal/animelib"
)

func TestPickAnimelibQuality(t *testing.T) {
	links := []animelib.QualityLink{
		{Quality: 720, Href: "720"},
		{Quality: 480, Href: "480"},
		{Quality: 1080, Href: "1080"},
	}

	tests := []struct {
		name      string
		preferred int
		want      int
	}{
		{name: "exact", preferred: 1080, want: 1080},
		{name: "nearest lower", preferred: 900, want: 720},
		{name: "nearest higher", preferred: 360, want: 480},
		{name: "default is 1080", preferred: 0, want: 1080},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pickAnimelibQuality(links, test.preferred)
			if got.Quality != test.want {
				t.Fatalf("quality = %d, want %d", got.Quality, test.want)
			}
		})
	}

	if got := pickAnimelibQuality(nil, 1080); got.Href != "" {
		t.Fatalf("empty links should stay empty, got %+v", got)
	}
}
