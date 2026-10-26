package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func (w *titleWindow) layoutEpisodes(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() || len(w.episodes) == 0 {
		return
	}
	gap := gtx.Dp(titlePageEpisodeGap)
	minWidth := gtx.Dp(titlePageEpisodeMinWidth)
	columns := 7
	for columns > 1 {
		width := (rect.Dx() - gap*(columns-1)) / columns
		if width >= minWidth {
			break
		}
		columns--
	}
	buttonWidth := (rect.Dx() - gap*(columns-1)) / columns
	if buttonWidth <= 0 {
		return
	}
	rowHeight := gtx.Dp(titlePageEpisodeHeight)
	rows := (len(w.episodes) + columns - 1) / columns
	w.episodeList.Axis = layout.Vertical
	w.episodeList.Gap = gap

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	w.episodeList.Layout(lgtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
		for column := 0; column < columns; column++ {
			index := row*columns + column
			if index >= len(w.episodes) {
				break
			}
			left := column * (buttonWidth + gap)
			cell := image.Rect(left, 0, left+buttonWidth, rowHeight)
			w.layoutEpisodeButton(gtx, index, cell)
		}
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	stack.Pop()
}

func (w *titleWindow) layoutEpisodeButton(gtx layout.Context, index int, rect image.Rectangle) {
	if rect.Empty() || index < 0 || index >= len(w.episodes) {
		return
	}
	click := &w.episodes[index]
	_, clicked := click.Update(gtx)
	missing := w.episodeMissing(w.item.episodes[index])
	if clicked && !missing {
		w.selectedEpisode = index
		gtx.Execute(op.InvalidateCmd{})
	}
	hovered := click.Hovered() && !missing

	if missing {
		stack := op.Offset(rect.Min).Push(gtx.Ops)
		w.drawEpisodeFace(gtx, index, image.Rectangle{Max: rect.Size()}, true, false)
		stack.Pop()
		return
	}
	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		w.drawEpisodeFace(gtx, index, image.Rectangle{Max: rect.Size()}, false, hovered)
	})
}

func (w *titleWindow) drawEpisodeFace(gtx layout.Context, index int, local image.Rectangle, missing, hovered bool) {
	if index < 0 || index >= len(w.episodes) || local.Empty() {
		return
	}
	if index == w.selectedEpisode && !missing {
		fillRoundedGradient(gtx, local, local, gtx.Dp(unit.Dp(8)), mainAccentColor1, mainAccentColor2)
	} else {
		fill := titleEpisodeFill
		border := titleEpisodeBorder
		if hovered {
			fill = titleEpisodeHover
			border = titleVoiceHoverBorder
		}
		if missing {
			fill = titlePanelFill
			border = titlePanelBorder
		}
		fillRounded(gtx, border, local, gtx.Dp(unit.Dp(8)))
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(7)))
	}
	textColor := titleEpisodeText
	if missing {
		textColor = titleEpisodeMuted
	} else if index == w.selectedEpisode {
		textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	w.typo.drawCentered(
		gtx,
		w.item.episodes[index],
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageEpisodeSize,
		textColor,
		centerOf(local),
	)
}
