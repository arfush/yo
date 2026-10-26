package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func (w *titleWindow) layoutQualities(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	gap := gtx.Dp(unit.Dp(8))
	n := len(w.qualities)
	if n == 0 {
		return
	}
	width := (rect.Dx() - gap*(n-1)) / n
	for i := range w.qualities {
		left := rect.Min.X + i*(width+gap)
		cell := image.Rect(left, rect.Min.Y, left+width, rect.Max.Y)
		if cell.Empty() {
			continue
		}
		click := &w.qualities[i]
		_, clicked := click.Update(gtx)
		if clicked {
			w.selectedQuality = i
			if w.onQuality != nil {
				if err := w.onQuality(i); err != nil {
					w.watchError = err.Error()
					if w.onError != nil {
						w.onError(err)
					}
				}
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		hovered := click.Hovered()
		w.layoutClickableDraw(gtx, cell, click, func(gtx layout.Context) {
			local := image.Rect(0, 0, cell.Dx(), cell.Dy())
			if i == w.selectedQuality {
				fillRoundedGradient(gtx, local, local, gtx.Dp(unit.Dp(8)), mainAccentColor1, mainAccentColor2)
			} else {
				fill := titleQualityFill
				border := titleQualityBorder
				if hovered {
					fill = titleQualityHover
					border = titleVoiceHoverBorder
				}
				fillRounded(gtx, border, local, gtx.Dp(unit.Dp(8)))
				fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(7)))
			}
			textColor := titleQualityText
			if i == w.selectedQuality {
				textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
			}
			w.typo.drawCentered(
				gtx,
				titleQualityLabels[i],
				font.Font{Typeface: fontFamily, Weight: font.Medium},
				titlePageQualitySize,
				textColor,
				centerOf(local),
			)
		})
	}
}
