package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
)

const (
	episodeProgressHeight  = unit.Dp(6)
	episodeProgressLabelSz = unit.Sp(10.5)
	episodeProgressGap     = unit.Dp(10)
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
	radius := rect.Dy() / 2
	fillRounded(gtx, episodeProgressTrack, rect, radius)
	if progress <= 0 {
		return
	}
	if progress > 1 {
		progress = 1
	}
	width := int(math.Round(float64(rect.Dx()) * float64(progress)))
	if width < rect.Dy() {
		width = rect.Dy()
	}
	if width > rect.Dx() {
		width = rect.Dx()
	}
	filled := image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+width, rect.Max.Y)
	fillRoundedGradient(gtx, filled, rect, radius, mainAccentColor1, mainAccentColor2)
}
