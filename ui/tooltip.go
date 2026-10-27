package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func (w *titleScreen) layoutTooltip(gtx layout.Context, size image.Point) {
	if w.tooltipIndex < 0 || w.tooltipIndex >= len(w.item.voices) {
		return
	}
	label := ""
	if w.tooltipKodik {
		label = "Будут серии с Kodik"
	} else {
		missing := w.missingForCurrentSeason(w.item.voices[w.tooltipIndex])
		label = voiceMissingLabelForEpisodes(missing)
	}
	if label == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	margin := gtx.Dp(unit.Dp(8))
	padX := gtx.Dp(unit.Dp(10))
	padY := gtx.Dp(unit.Dp(7))
	maxTextWidth := min(gtx.Dp(unit.Dp(340)), size.X-2*margin-2*padX)
	maxTextWidth = max(maxTextWidth, gtx.Dp(unit.Dp(80)))
	_, ink := w.typo.measureWrapped(gtx, label, face, titlePageTooltipSize, maxTextWidth)
	if ink.Empty() {
		return
	}
	tipWidth := ink.Dx() + 2*padX
	tipHeight := ink.Dy() + 2*padY
	x := w.tooltipAnchor.X
	y := w.tooltipAnchor.Y - tipHeight/2
	if x+tipWidth > size.X-margin {
		x = size.X - margin - tipWidth
	}
	if x < margin {
		x = margin
	}
	if y < margin {
		y = margin
	}
	if y+tipHeight > size.Y-margin {
		y = size.Y - margin - tipHeight
	}
	rect := image.Rect(x, y, x+tipWidth, y+tipHeight)
	radius := gtx.Dp(unit.Dp(6))
	fillRounded(gtx, titleTooltipBorder, rect, radius)
	fillRounded(gtx, titleTooltipFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
	w.typo.drawWrapped(gtx, label, face, titlePageTooltipSize, titleTooltipText, image.Pt(rect.Min.X+padX, rect.Min.Y+padY), maxTextWidth)
}
