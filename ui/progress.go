package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
)

const (
	episodeProgressHeight  = unit.Dp(6)
	episodeProgressLabelSz = unit.Sp(11)
	episodeProgressGap     = unit.Dp(10)
	// historyProgressGap separates the title line from the progress row in the
	// history card. It matches the space a 52dp poster leaves around the block,
	// so the bar sits low in the card instead of hugging the title.
	historyProgressGap = unit.Dp(18)
)

var (
	episodeProgressTrack    = color.NRGBA{R: 0x27, G: 0x24, B: 0x35, A: 0xff}
	episodeProgressLabelCol = color.NRGBA{R: 0x9b, G: 0x94, B: 0xad, A: 0xff}
)

// episodeProgressValue turns the saved playback position into a 0..1 fill.
// Unknown or finished durations show an empty bar.
func episodeProgressValue(resumeAt, duration float64) float32 {
	if duration <= 0 || resumeAt <= 0 {
		return 0
	}
	value := resumeAt / duration
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return float32(value)
}

// episodeProgressLabel formats the "2 / 28 эп." caption. It returns an empty
// string when the total or the current episode is unknown.
func episodeProgressLabel(current string, total int) string {
	current = strings.TrimSpace(current)
	if current == "" || total <= 0 {
		return ""
	}
	return fmt.Sprintf("%s / %d эп.", current, total)
}

// drawEpisodeProgressBar paints a rounded track and its filled part.
func drawEpisodeProgressBar(gtx layout.Context, rect image.Rectangle, progress float32) {
	if rect.Empty() {
		return
	}
	drawProgressBar(gtx, rect, progress, progressBarStyle{
		track:   episodeProgressTrack,
		fill1:   mainAccentColor1,
		fill2:   mainAccentColor2,
		minFill: rect.Dy(),
	})
}
