package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// episodeGridColumns picks how many cells fit into width while keeping the
// episode buttons at least minWidth wide. Seasons use the same shape so the two
// blocks look like one control.
func episodeGridColumns(width, gap, minWidth int) int {
	columns := 7
	for columns > 1 {
		cell := (width - gap*(columns-1)) / columns
		if cell >= minWidth {
			break
		}
		columns--
	}
	return columns
}

func (w *titleScreen) layoutEpisodes(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() || len(w.episodes) == 0 {
		return
	}
	visible := w.visibleEpisodeIndices()
	if len(visible) == 0 {
		return
	}
	gap := gtx.Dp(titlePageEpisodeGap)
	minWidth := gtx.Dp(titlePageEpisodeMinWidth)
	columns := episodeGridColumns(rect.Dx(), gap, minWidth)
	buttonWidth := (rect.Dx() - gap*(columns-1)) / columns
	if buttonWidth <= 0 {
		return
	}
	rowHeight := gtx.Dp(titlePageEpisodeHeight)
	rows := (len(visible) + columns - 1) / columns
	w.episodeList.Axis = layout.Vertical
	w.episodeList.Gap = gap
	w.episodeColumns = columns

	if !w.episodesPlaced {
		w.episodesPlaced = true
		stride := rowHeight + gap
		visibleRows := 1
		if stride > 0 {
			visibleRows = max(1, (rect.Dy()+gap)/stride)
		}
		first := w.selectedVisibleRow()
		if first+visibleRows > rows {
			first = rows - visibleRows
		}
		if first < 0 {
			first = 0
		}
		w.episodeList.Position = layout.Position{First: first}
	}

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	w.episodeList.Layout(lgtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
		for column := 0; column < columns; column++ {
			position := row*columns + column
			if position >= len(visible) {
				break
			}
			index := visible[position]
			left := column * (buttonWidth + gap)
			cell := image.Rect(left, 0, left+buttonWidth, rowHeight)
			w.layoutEpisodeButton(gtx, index, cell)
		}
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	stack.Pop()
}

// layoutSeasons draws the season grid above the episode grid. Only a series
// with more than one season gets it; a single-season title shows its episodes
// directly, exactly like an AnimeLIB title. It returns the height the grid used
// so the caller can place the episode label below it.
func (w *titleScreen) layoutSeasons(gtx layout.Context, rect image.Rectangle) int {
	if rect.Empty() || len(w.item.seasons) <= 1 {
		return 0
	}
	gap := gtx.Dp(titlePageEpisodeGap)
	minWidth := gtx.Dp(titlePageEpisodeMinWidth)
	columns := episodeGridColumns(rect.Dx(), gap, minWidth)
	width := (rect.Dx() - gap*(columns-1)) / columns
	if width <= 0 {
		return 0
	}
	rowHeight := gtx.Dp(titlePageEpisodeHeight)
	rows := (len(w.item.seasons) + columns - 1) / columns
	for index := range w.item.seasons {
		row := index / columns
		column := index % columns
		left := rect.Min.X + column*(width+gap)
		top := rect.Min.Y + row*(rowHeight+gap)
		cell := image.Rect(left, top, left+width, top+rowHeight)
		w.layoutSeasonButton(gtx, index, cell)
	}
	return rows*rowHeight + (rows-1)*gap
}

func (w *titleScreen) layoutSeasonButton(gtx layout.Context, index int, rect image.Rectangle) {
	if rect.Empty() || index < 0 || index >= len(w.item.seasons) || index >= len(w.seasons) {
		return
	}
	click := &w.seasons[index]
	_, clicked := click.Update(gtx)
	if clicked {
		w.selectSeason(gtx, index)
	}
	hovered := click.Hovered()
	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		w.drawSeasonFace(gtx, index, image.Rectangle{Max: rect.Size()}, hovered)
	})
}

func (w *titleScreen) drawSeasonFace(gtx layout.Context, index int, local image.Rectangle, hovered bool) {
	if index < 0 || index >= len(w.item.seasons) || local.Empty() {
		return
	}
	selected := index == w.selectedSeason
	if selected {
		fillRoundedGradient(gtx, local, local, gtx.Dp(unit.Dp(8)), mainAccentColor1, mainAccentColor2)
	} else {
		fill := titleEpisodeFill
		border := titleEpisodeBorder
		if hovered {
			fill = titleEpisodeHover
			border = titleVoiceHoverBorder
		}
		fillRounded(gtx, border, local, gtx.Dp(unit.Dp(8)))
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(7)))
	}
	textColor := titleEpisodeText
	if selected {
		textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	w.typo.drawCentered(
		gtx,
		fmt.Sprintf("%d", w.item.seasons[index].number),
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageEpisodeSize,
		textColor,
		centerOf(local),
	)
}

func (w *titleScreen) layoutEpisodeButton(gtx layout.Context, index int, rect image.Rectangle) {
	if rect.Empty() || index < 0 || index >= len(w.episodes) {
		return
	}
	click := &w.episodes[index]
	_, clicked := click.Update(gtx)
	missing := w.episodeMissing(w.item.episodes[index])
	if clicked && !missing {
		w.selectedEpisode = index
		w.selectedSeason = w.item.seasonIndexForEpisode(index)
		w.episodesPlaced = false
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

func (w *titleScreen) drawEpisodeFace(gtx layout.Context, index int, local image.Rectangle, missing, hovered bool) {
	if index < 0 || index >= len(w.episodes) || index >= len(w.item.episodes) || local.Empty() {
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
		episodeDisplayLabel(w.item.episodes[index]),
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageEpisodeSize,
		textColor,
		centerOf(local),
	)
}
