package ui

import (
	"image"

	"gioui.org/layout"
)

func drawDownloadCard(gtx layout.Context, size image.Point, px func(float32) int) {
	card := image.Rectangle{Max: size}

	// A few very transparent layers give the card a soft edge without needing
	// an offscreen blur. The shadow is deliberately subtle against the dark
	// content surface.
	for i := 10; i >= 2; i -= 2 {
		r := card.Inset(-px(float32(i))).Add(image.Pt(0, px(3)))
		fillRounded(gtx, firstRunShadowColor, r, px(firstRunCardRadius+float32(i)))
	}

	fillRounded(gtx, firstRunCardBorder, card, px(firstRunCardRadius))
	fillRounded(gtx, firstRunCardColor, card.Inset(px(1)), px(firstRunCardRadius-1))
}
