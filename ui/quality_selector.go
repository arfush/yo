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

// layoutQualities draws the quality block. A step the selected Filmlib voice
// cannot deliver is painted muted and ignores clicks instead of silently
// falling back during playback. The row scrolls horizontally when the ladder
// does not fit, and the wheel works on it like on a vertical list.
func (w *titleScreen) layoutQualities(gtx layout.Context, rect image.Rectangle) {
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
	if minWidth := gtx.Dp(titlePageQualityMinWidth); width < minWidth {
		width = minWidth
	}
	if width <= 0 {
		return
	}
	w.qualityList.Axis = layout.Horizontal
	w.qualityList.Gap = gap
	// A plain vertical wheel over the horizontal strip should scroll it too.
	w.qualityList.ScrollAnyAxis = true
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	w.qualityList.Layout(lgtx, n, func(gtx layout.Context, index int) layout.Dimensions {
		cell := image.Rect(0, 0, width, rect.Dy())
		w.layoutQualityButton(gtx, index, cell)
		return layout.Dimensions{Size: cell.Size()}
	})
	stack.Pop()
}

func (w *titleScreen) layoutQualityButton(gtx layout.Context, index int, cell image.Rectangle) {
	if cell.Empty() || index < 0 || index >= len(w.qualityOptions) || index >= len(w.qualities) {
		return
	}
	option := w.qualityOptions[index]
	click := &w.qualities[index]
	_, clicked := click.Update(gtx)
	if !option.enabled {
		w.layoutHitArea(gtx, cell, click, false, func(gtx layout.Context) {
			w.drawQualityButton(gtx, image.Rect(0, 0, cell.Dx(), cell.Dy()), option, false, false)
		})
		return
	}
	if clicked {
		w.selectedQuality = index
		if w.onQuality != nil {
			if err := w.onQuality(option.value); err != nil {
				w.watchError = userErrorText(err)
				if w.onError != nil {
					w.onError(err)
				}
			}
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	hovered := click.Hovered()
	selected := index == w.selectedQuality
	w.layoutClickableDraw(gtx, cell, click, func(gtx layout.Context) {
		w.drawQualityButton(gtx, image.Rect(0, 0, cell.Dx(), cell.Dy()), option, selected, hovered)
	})
}

func (w *titleScreen) drawQualityButton(gtx layout.Context, local image.Rectangle, option qualityOption, selected, hovered bool) {
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
