package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func (w *titleWindow) layoutHeaderProgress(gtx layout.Context, rect image.Rectangle) {
	label := w.item.progressLabel()
	barHeight := gtx.Dp(episodeProgressHeight)
	if rect.Empty() || (!w.item.movie && label == "") {
		return
	}
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	var labelInk image.Rectangle
	if label != "" {
		_, labelInk = w.typo.measure(gtx, label, labelFace, episodeProgressLabelSz)
	}
	requiredHeight := barHeight
	if labelInk.Dy() > requiredHeight {
		requiredHeight = labelInk.Dy()
	}
	if rect.Dy() < requiredHeight {
		return
	}
	gap := gtx.Dp(episodeProgressGap)
	barWidth := rect.Dx() * 28 / 100
	if barWidth < gtx.Dp(unit.Dp(120)) {
		barWidth = gtx.Dp(unit.Dp(120))
	}
	maxBar := rect.Dx()
	if labelInk.Dx() > 0 {
		maxBar -= gap + labelInk.Dx()
	}
	if barWidth > maxBar {
		barWidth = maxBar
	}
	if barWidth < gtx.Dp(unit.Dp(56)) {
		return
	}
	bar := image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+barWidth, rect.Min.Y+barHeight)
	progress := float32(0)
	if w.selectedEpisode == w.progressEpisode {
		progress = episodeProgressValue(w.item.resumeAt, w.item.duration)
	}
	drawEpisodeProgressBar(gtx, bar, progress)
	if label == "" {
		return
	}
	w.typo.draw(
		gtx,
		label,
		labelFace,
		episodeProgressLabelSz,
		episodeProgressLabelCol,
		image.Pt(bar.Max.X+gap, bar.Min.Y+(barHeight-labelInk.Dy())/2),
	)
}

func (w *titleWindow) drawAnnouncement(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	drawTruncatedText(
		gtx,
		w.typo,
		"Тайтл ещё не вышел. Озвучки и серии появятся после премьеры.",
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		titlePageOriginalSize,
		mainMutedColor,
		image.Pt(rect.Min.X, rect.Min.Y+gtx.Dp(unit.Dp(4))),
		rect.Dx(),
	)
}
