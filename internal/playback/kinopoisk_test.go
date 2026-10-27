package playback

import (
	"testing"

	"github.com/arfush/yoanime/internal/kinopoisk"
	"github.com/arfush/yoanime/internal/metadata"
)

// The theatre reports skip ranges as plain seconds. They must become chapters
// whose titles autoSkipTarget understands, otherwise Alloha's skipTime would be
// silently ignored.
func TestKinopoiskChaptersDriveAutoSkip(t *testing.T) {
	chapters := kinopoiskChapters([]kinopoisk.SkipRange{
		{Start: 0, End: 34},
		{Start: 1500, End: 1600},
	})
	if len(chapters) != 4 {
		t.Fatalf("got %d chapters, want 4: %v", len(chapters), chapters)
	}
	enabled := map[metadata.Segment]bool{
		metadata.SegmentIntro: true,
		metadata.SegmentOutro: true,
	}

	target, index, ok := autoSkipTarget(chapters, 5, enabled, map[int]bool{})
	if !ok || target != 34 {
		t.Fatalf("intro skip = %v, %d, %v; want 34", target, index, ok)
	}
	target, _, ok = autoSkipTarget(chapters, 1550, enabled, map[int]bool{index: true})
	if !ok || target != 1600 {
		t.Fatalf("outro skip = %v, %v; want 1600", target, ok)
	}
	if _, _, ok := autoSkipTarget(chapters, 700, enabled, map[int]bool{}); ok {
		t.Fatalf("skipped a position outside every range")
	}

	// A film usually carries a single range, which is its opening.
	film := kinopoiskChapters([]kinopoisk.SkipRange{{Start: 0, End: 34}})
	if len(film) != 2 || film[0].Title != metadata.ChapterTitle(metadata.SegmentIntro) {
		t.Fatalf("film chapters = %v", film)
	}
	if _, ok := metadata.SegmentFromChapterTitle(film[0].Title); !ok {
		t.Fatalf("chapter title %q is not a known segment", film[0].Title)
	}
}
