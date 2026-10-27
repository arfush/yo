package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yoanime/internal/library"
)

type historyRow struct {
	titleClick  widget.Clickable
	playClick   widget.Clickable
	trashClick  widget.Clickable
	statusClick widget.Clickable

	hover       hoverAnim
	playHover   hoverAnim
	trashHover  hoverAnim
	statusHover hoverAnim
}

type historyRowGeometry struct {
	handle        image.Rectangle
	poster        image.Rectangle
	play          image.Rectangle
	trash         image.Rectangle
	releaseStatus image.Rectangle
	status        image.Rectangle
	titleHit      image.Rectangle
	titleX        int
}

type historyRowVisual struct {
	selected    bool
	dragging    bool
	hover       float32
	playHover   float32
	trashHover  float32
	loading     bool
	itemLoading bool
}

func (m *mainWindow) historyRowGeometry(gtx layout.Context, width, height int, item historyTitle) historyRowGeometry {
	posterWidth := gtx.Dp(mainPosterWidth)
	posterHeight := gtx.Dp(mainPosterHeight)
	controlSize := gtx.Dp(mainControlSize)
	controlGap := gtx.Dp(mainControlGap)
	pad := gtx.Dp(mainRowPadding)
	handleWidth := gtx.Dp(mainDragHandleWidth)
	handle := image.Rect(pad, 0, pad+handleWidth, height)
	poster := image.Rect(
		handle.Max.X+gtx.Dp(unit.Dp(6)),
		(height-posterHeight)/2,
		handle.Max.X+gtx.Dp(unit.Dp(6))+posterWidth,
		(height-posterHeight)/2+posterHeight,
	)
	trash := image.Rect(width-pad-controlSize, (height-controlSize)/2, width-pad, (height-controlSize)/2+controlSize)
	play := image.Rect(trash.Min.X-controlGap-controlSize, trash.Min.Y, trash.Min.X-controlGap, trash.Max.Y)
	titleX := poster.Max.X + gtx.Dp(unit.Dp(14))
	statusWidth := m.userStatusWidth(gtx)
	statusHeight := gtx.Dp(mainStatusHeight)
	statusGap := gtx.Dp(mainStatusGap)
	status := image.Rect(
		play.Min.X-gtx.Dp(unit.Dp(14))-statusWidth,
		(height-statusHeight)/2,
		play.Min.X-gtx.Dp(unit.Dp(14)),
		(height-statusHeight)/2+statusHeight,
	)
	releaseStatus := image.Rect(
		status.Min.X-statusGap-statusWidth,
		status.Min.Y,
		status.Min.X-statusGap,
		status.Max.Y,
	)
	titleHit := image.Rect(handle.Max.X, 0, releaseStatus.Min.X-gtx.Dp(unit.Dp(8)), height)
	if titleHit.Max.X < titleHit.Min.X+1 {
		titleHit.Max.X = titleHit.Min.X + 1
	}
	return historyRowGeometry{
		handle:        handle,
		poster:        poster,
		play:          play,
		trash:         trash,
		releaseStatus: releaseStatus,
		status:        status,
		titleHit:      titleHit,
		titleX:        titleX,
	}
}

// drawHistoryRowCard paints a card at rect. The geometry rectangles are local
// to the card, so the same code can draw both a list row and the floating drag
// overlay.
func (m *mainWindow) drawHistoryRowCard(gtx layout.Context, rect image.Rectangle, item historyTitle, geom historyRowGeometry, vis historyRowVisual) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()

	rowRect := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(mainRowRadius)
	background := mainRowColor
	if vis.selected {
		background = mainRowSelectedColor
	}
	if vis.hover > 0 {
		background = mixColor(background, mainRowHoverColor, vis.hover)
	}
	fillRounded(gtx, background, rowRect, radius)

	border := mainRowBorderColor
	if vis.hover > 0 {
		border = mixColor(border, mainRowHoverBorder, vis.hover)
	}
	if vis.selected || vis.dragging {
		border = mainAccentColor1
	}
	strokeRoundedRect(
		gtx,
		rowRect.Inset(gtx.Dp(unit.Dp(1))),
		float32(radius),
		float32(gtx.Dp(unit.Dp(1))),
		border,
	)

	handleColor := mainDragDotColor
	if vis.selected || vis.dragging {
		handleColor = mainDragDotActive
	}
	drawDragHandle(gtx, geom.handle, handleColor)

	m.drawPoster(gtx, geom.poster, item)

	titleRight := geom.releaseStatus.Min.X - gtx.Dp(unit.Dp(14))
	if titleRight < geom.titleX+1 {
		titleRight = geom.titleX + 1
	}
	titleFace := font.Font{Typeface: fontFamily, Weight: font.SemiBold}
	_, titleInk := m.typo.measure(gtx, item.title, titleFace, mainRowTitleSize)
	label := item.progressLabel()
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	var labelInk image.Rectangle
	if label != "" {
		_, labelInk = m.typo.measure(gtx, label, labelFace, episodeProgressLabelSz)
	}
	// A film has no episode counter, but its watch progress bar still belongs
	// in the row — until it is marked watched: a finished film has nothing left
	// to resume, so the bar would only be noise. Series without a known episode
	// keep the old title-only layout.
	showProgress := (item.movie && item.userStatus != library.UserStatusCompleted) || label != ""
	if !showProgress {
		m.drawTruncated(
			gtx,
			item.title,
			titleFace,
			mainRowTitleSize,
			mainTextColor,
			image.Pt(geom.titleX, (rowRect.Dy()-titleInk.Dy())/2),
			titleRight-geom.titleX,
		)
	} else {
		barHeight := gtx.Dp(episodeProgressHeight)
		progressGap := gtx.Dp(historyProgressGap)
		progressHeight := barHeight
		if labelInk.Dy() > progressHeight {
			progressHeight = labelInk.Dy()
		}
		blockTop := (rowRect.Dy() - (titleInk.Dy() + progressGap + progressHeight)) / 2
		m.drawTruncated(
			gtx,
			item.title,
			titleFace,
			mainRowTitleSize,
			mainTextColor,
			image.Pt(geom.titleX, blockTop),
			titleRight-geom.titleX,
		)

		barTop := blockTop + titleInk.Dy() + progressGap + (progressHeight-barHeight)/2
		labelGap := gtx.Dp(episodeProgressGap)
		barWidth := rowRect.Dx() * 28 / 100
		maxBar := titleRight - geom.titleX
		if labelInk.Dx() > 0 {
			maxBar -= labelGap + labelInk.Dx()
		}
		if barWidth > maxBar {
			barWidth = maxBar
		}
		if barWidth >= gtx.Dp(unit.Dp(56)) {
			bar := image.Rect(geom.titleX, barTop, geom.titleX+barWidth, barTop+barHeight)
			drawEpisodeProgressBar(gtx, bar, episodeProgressValue(item.resumeAt, item.duration))
			if label != "" {
				m.typo.draw(
					gtx,
					label,
					labelFace,
					episodeProgressLabelSz,
					episodeProgressLabelCol,
					image.Pt(bar.Max.X+labelGap, barTop+(barHeight-labelInk.Dy())/2),
				)
			}
		} else if label != "" {
			m.typo.draw(
				gtx,
				label,
				labelFace,
				episodeProgressLabelSz,
				episodeProgressLabelCol,
				image.Pt(titleRight-labelInk.Dx(), barTop+(barHeight-labelInk.Dy())/2),
			)
		}
	}
	drawStatusPill(gtx, m.typo, geom.releaseStatus, item.status)
	drawUserStatusPill(gtx, m.typo, geom.status, item.userStatus, true)

	// The action buttons keep one quiet surface in every state. Only the glyph
	// reacts to hover; pulling in the footer's purple hover outline made the
	// two history controls look like a second footer inside the row.
	actionStyle := iconButtonStyle{
		Fill:        mainControlFill,
		FillHover:   mainControlFill,
		Border:      mainControlBorder,
		BorderHover: mainControlBorder,
		Radius:      mainControlRadius,
	}
	if vis.itemLoading {
		loadingStyle := actionStyle
		loadingStyle.Border = mainAccentColor1
		loadingStyle.BorderHover = mainAccentColor1
		drawIconButtonSurface(gtx, geom.play, loadingStyle, 0)
		drawSpinner(gtx, centerOf(geom.play), gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(1.5)), 0, mainAccentColor1)
	} else {
		buttonTextColor := mixColor(mainControlColor, mainHistoryPlayColor, vis.playHover)
		if vis.loading {
			buttonTextColor = withAlpha(mainFaintColor, 0xb0)
		}
		drawIconButtonSurface(gtx, geom.play, actionStyle, vis.playHover)
		drawHistoryPlay(gtx, centerOf(geom.play), gtx.Dp(unit.Dp(24)), buttonTextColor)
	}

	trashColor := mixColor(mainControlColor, mainDangerColor, vis.trashHover)
	drawIconButtonSurface(gtx, geom.trash, actionStyle, vis.trashHover)
	drawHistoryTrash(gtx, centerOf(geom.trash), gtx.Dp(unit.Dp(24)), trashColor)
}

func (m *mainWindow) layoutHistoryRow(gtx layout.Context, index, width, height int) {
	if index < 0 || index >= len(m.history) || index >= len(m.rows) {
		return
	}
	row := &m.rows[index]
	item := m.history[index]
	geom := m.historyRowGeometry(gtx, width, height, item)

	_, titleClicked := row.titleClick.Update(gtx)
	if titleClicked && !m.drag.active {
		if m.selected == index {
			// The first press only highlights a title. Pressing the already
			// highlighted card opens it, so no double click is needed.
			m.startLoading(gtx, index, false)
		} else {
			m.selected = index
			m.focusHistory(gtx)
		}
	}

	playClicked := false
	if !m.loading && !m.playing {
		_, playClicked = row.playClick.Update(gtx)
		if playClicked && !m.drag.active {
			m.playHistory(gtx, index)
		}
	}
	_, trashClicked := row.trashClick.Update(gtx)
	if trashClicked && !m.drag.active {
		m.deleteRequested = true
		m.deleteIndex = index
	}
	statusClickable := item.status != statusAnnounced
	if statusClickable && row.statusClick.Clicked(gtx) && !m.drag.active {
		stride := m.listRowHeight + m.listGap
		top := m.listRect.Min.Y + m.historyRowTop(index, stride)
		anchor := geom.status.Add(image.Pt(m.listRect.Min.X, top))
		m.statusMenu.show(anchor, item.userStatus, item.id, geom.status.Dx(), geom.status.Dy())
		m.selected = index
		gtx.Execute(op.InvalidateCmd{})
	}

	titleHovered := row.titleClick.Hovered()
	playHovered := !m.loading && !m.playing && row.playClick.Hovered()
	trashHovered := row.trashClick.Hovered()
	statusHovered := statusClickable && row.statusClick.Hovered()
	rowHover := row.hover.update(gtx, titleHovered || playHovered || trashHovered || statusHovered)
	playHover := row.playHover.update(gtx, playHovered)
	trashHover := row.trashHover.update(gtx, trashHovered)

	dragging := m.drag.active && m.drag.index == index
	if dragging {
		// The dragged card is drawn by the window-level overlay. Its slot shows
		// a quiet drop target while the other rows reflow around it.
		rowRect := image.Rect(0, 0, width, height)
		radius := gtx.Dp(mainRowRadius)
		fillRounded(gtx, withAlpha(mainAccentColor1, 0x10), rowRect, radius)
		strokeRoundedRect(
			gtx,
			rowRect.Inset(gtx.Dp(unit.Dp(1))),
			float32(radius),
			float32(gtx.Dp(unit.Dp(1))),
			withAlpha(mainAccentColor1, 0x66),
		)
		return
	}

	loadingItem := m.loading && m.loadingTarget == index && m.loadingAnimateRow
	playingItem := m.playing && m.playingID == item.id
	itemLoading := loadingItem || playingItem
	m.drawHistoryRowCard(gtx, image.Rect(0, 0, width, height), item, geom, historyRowVisual{
		selected:    m.selected == index,
		hover:       rowHover,
		playHover:   playHover,
		trashHover:  trashHover,
		loading:     m.loading || m.playing,
		itemLoading: itemLoading,
	})

	if !m.loading && !m.playing {
		m.layoutHitArea(gtx, geom.play, &row.playClick, true, nil)
	}
	m.layoutHitArea(gtx, geom.trash, &row.trashClick, true, nil)
	if statusClickable {
		m.layoutHitArea(gtx, geom.status, &row.statusClick, true, nil)
	}
	m.layoutHitArea(gtx, geom.titleHit, &row.titleClick, false, nil)

	if itemLoading && m.animationsRun() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(mainSpinnerFrameTime)})
	}
}

func drawDragHandle(gtx layout.Context, rect image.Rectangle, col color.NRGBA) {
	if rect.Empty() {
		return
	}
	dotRadius := max(1, gtx.Dp(unit.Dp(1.25)))
	gapX := gtx.Dp(unit.Dp(3.5))
	gapY := gtx.Dp(unit.Dp(4.5))
	cx := rect.Min.X + rect.Dx()/2
	cy := rect.Min.Y + rect.Dy()/2
	for row := -1; row <= 1; row++ {
		for column := -1; column <= 1; column += 2 {
			fillCircle(gtx, image.Pt(cx+column*gapX/2, cy+row*gapY), dotRadius, col)
		}
	}
}
