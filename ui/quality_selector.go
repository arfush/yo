package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// layoutQualities draws the quality block. A step the selected Kinopoisk voice
// cannot deliver is painted muted and ignores clicks instead of silently
// falling back during playback.
func (w *titleWindow) layoutQualities(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	n := len(w.qualityOptions)
	if n == 0 {
		return
	}
	if len(w.qualities) != n {
		w.qualities = make([]widget.Clickable, n)
	}
	gap := gtx.Dp(unit.Dp(8))
	width := (rect.Dx() - gap*(n-1)) / n
	for i, option := range w.qualityOptions {
		left := rect.Min.X + i*(width+gap)
		cell := image.Rect(left, rect.Min.Y, left+width, rect.Max.Y)
		if cell.Empty() {
			continue
		}
		click := &w.qualities[i]
		_, clicked := click.Update(gtx)
		if !option.enabled {
			w.layoutHitArea(gtx, cell, click, false, func(gtx layout.Context) {
				w.drawQualityButton(gtx, image.Rect(0, 0, cell.Dx(), cell.Dy()), option, false, false)
			})
			continue
		}
		if clicked {
			w.selectedQuality = i
			if w.onQuality != nil {
				if err := w.onQuality(option.value); err != nil {
					w.watchError = err.Error()
					if w.onError != nil {
						w.onError(err)
					}
				}
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		hovered := click.Hovered()
		selected := i == w.selectedQuality
		w.layoutClickableDraw(gtx, cell, click, func(gtx layout.Context) {
			w.drawQualityButton(gtx, image.Rect(0, 0, cell.Dx(), cell.Dy()), option, selected, hovered)
		})
	}
}

func (w *titleWindow) drawQualityButton(gtx layout.Context, local image.Rectangle, option qualityOption, selected, hovered bool) {
	if local.Empty() {
		return
	}
	if selected {
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
	switch {
	case selected:
		textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case !option.enabled:
		textColor = mainFaintColor
	}
	w.typo.drawCentered(
		gtx,
		option.label,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageQualitySize,
		textColor,
		centerOf(local),
	)
}
