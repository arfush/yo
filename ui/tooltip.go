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
		label = voiceMissingLabel(w.item.voices[w.tooltipIndex])
	}
	if label == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, label, face, titlePageTooltipSize)
	if ink.Empty() {
		return
	}
	padX := gtx.Dp(unit.Dp(10))
	padY := gtx.Dp(unit.Dp(7))
	tipWidth := ink.Dx() + 2*padX
	tipHeight := ink.Dy() + 2*padY
	x := w.tooltipAnchor.X
	y := w.tooltipAnchor.Y - tipHeight/2
	if x+tipWidth > size.X-gtx.Dp(unit.Dp(8)) {
		x = size.X - gtx.Dp(unit.Dp(8)) - tipWidth
	}
	if x < gtx.Dp(unit.Dp(8)) {
		x = gtx.Dp(unit.Dp(8))
	}
	if y < gtx.Dp(unit.Dp(8)) {
		y = gtx.Dp(unit.Dp(8))
	}
	if y+tipHeight > size.Y-gtx.Dp(unit.Dp(8)) {
		y = size.Y - gtx.Dp(unit.Dp(8)) - tipHeight
	}
	rect := image.Rect(x, y, x+tipWidth, y+tipHeight)
	radius := gtx.Dp(unit.Dp(6))
	fillRounded(gtx, titleTooltipBorder, rect, radius)
	fillRounded(gtx, titleTooltipFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
	w.typo.draw(gtx, label, face, titlePageTooltipSize, titleTooltipText, image.Pt(rect.Min.X+padX, rect.Min.Y+padY))
}
