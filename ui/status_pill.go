package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
)

func statusPillWidth(gtx layout.Context, typo *typography) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	// Release and user statuses share one width so both pills in a row look
	// like a single, stable column even when their labels differ.
	width := 0
	for _, label := range []string{"Завершён", "Онгоинг", "Анонс"} {
		if _, ink := typo.measure(gtx, label, face, mainStatusSize); ink.Dx() > width {
			width = ink.Dx()
		}
	}
	for _, status := range userStatusMenuOptions {
		if _, ink := typo.measure(gtx, userStatusLabel(status), face, mainStatusSize); ink.Dx() > width {
			width = ink.Dx()
		}
	}
	return 2*gtx.Dp(mainStatusPaddingX) + width
}

func (m *mainWindow) drawStatus(gtx layout.Context, rect image.Rectangle, status titleStatus) {
	drawStatusPill(gtx, m.typo, rect, status)
}

func drawStatusPill(gtx layout.Context, typo *typography, rect image.Rectangle, status titleStatus) {
	if rect.Empty() {
		return
	}
	label := "Завершён"
	textColor := mainStatusDoneText
	fillColor := mainStatusDoneFill
	borderColor := mainStatusDoneBorder
	switch status {
	case statusOngoing:
		label = "Онгоинг"
		textColor = mainStatusLiveText
		fillColor = mainStatusLiveFill
		borderColor = mainStatusLiveBorder
	case statusAnnounced:
		label = "Анонс"
		textColor = mainStatusAnnText
		fillColor = mainStatusAnnFill
		borderColor = mainStatusAnnBorder
	}
	drawPill(gtx, typo, rect, label, mainStatusSize, textColor, fillColor, borderColor)
}
