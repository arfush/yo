package playback

import (
	"testing"

	"github.com/arfush/yoanime/metadata"
)

func TestAutoSkipTargetFindsEnabledSegment(t *testing.T) {
	chapters := []metadata.Chapter{
		{Title: "Episode", Time: 0},
		{Title: "Recap", Time: 30},
		{Title: "Opening", Time: 90},
		{Title: "Episode", Time: 180},
		{Title: "Ending", Time: 1300},
		{Title: "Episode", Time: 1380},
	}
	enabled := map[metadata.Segment]bool{metadata.SegmentIntro: true}
	skipped := map[int]bool{}

	if target, index, ok := autoSkipTarget(chapters, 90, enabled, skipped); !ok || target != 180 || index != 2 {
		t.Fatalf("intro target = %v/%d/%v, want 180/2/true", target, index, ok)
	}
	if target, _, ok := autoSkipTarget(chapters, 89.9, enabled, skipped); ok {
		t.Fatalf("position before the intro must not skip to %v", target)
	}
	if target, _, ok := autoSkipTarget(chapters, 179.8, enabled, skipped); ok {
		t.Fatalf("position at the end of the intro must not skip to %v", target)
	}
	if _, _, ok := autoSkipTarget(chapters, 30, enabled, skipped); ok {
		t.Fatal("disabled recap must not be skipped")
	}
}

func TestAutoSkipTargetSkipsEachChapterOnce(t *testing.T) {
	chapters := []metadata.Chapter{
		{Title: "Opening", Time: 10},
		{Title: "Episode", Time: 100},
	}
	enabled := map[metadata.Segment]bool{metadata.SegmentIntro: true}
	skipped := map[int]bool{}

	target, index, ok := autoSkipTarget(chapters, 20, enabled, skipped)
	if !ok || target != 100 {
		t.Fatalf("first skip = %v/%v, want 100/true", target, ok)
	}
	skipped[index] = true
	if target, _, ok := autoSkipTarget(chapters, 20, enabled, skipped); ok {
		t.Fatalf("rewinding into the same chapter skipped again to %v", target)
	}
}

func TestAutoSkipTargetIgnoresPlainChapters(t *testing.T) {
	chapters := []metadata.Chapter{
		{Title: "Episode", Time: 0},
		{Title: "Episode", Time: 100},
	}
	enabled := map[metadata.Segment]bool{
		metadata.SegmentRecap:       true,
		metadata.SegmentIntro:       true,
		metadata.SegmentOutro:       true,
		metadata.SegmentPreview:     true,
		metadata.SegmentCredits:     true,
		metadata.SegmentPostCredits: true,
	}
	if target, _, ok := autoSkipTarget(chapters, 50, enabled, map[int]bool{}); ok {
		t.Fatalf("a plain episode chapter must not be skipped to %v", target)
	}
}
