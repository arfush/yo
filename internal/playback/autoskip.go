package playback

import "github.com/arfush/yoanime/internal/metadata"

// autoSkipMargin keeps the target a little before the exact end of a segment.
// mpv reports time-pos a few tenths behind the seek target, so without the
// margin a segment that ends at the end of the file would be "entered" again
// right after the jump.
const autoSkipMargin = 0.5

// autoSkipTarget returns the end of the chapter containing position when that
// chapter is an enabled introdb segment. Each chapter index is reported once,
// so a viewer who rewinds into the opening is not pushed forward again.
func autoSkipTarget(
	chapters []metadata.Chapter,
	position float64,
	enabled map[metadata.Segment]bool,
	skipped map[int]bool,
) (float64, int, bool) {
	for index := 0; index+1 < len(chapters); index++ {
		segment, ok := metadata.SegmentFromChapterTitle(chapters[index].Title)
		if !ok || !enabled[segment] || skipped[index] {
			continue
		}
		start := chapters[index].Time
		end := chapters[index+1].Time
		if end-start <= autoSkipMargin*2 {
			continue
		}
		if position < start || position >= end-autoSkipMargin {
			continue
		}
		return end, index, true
	}
	return 0, 0, false
}
