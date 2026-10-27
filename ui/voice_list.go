package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func (w *titleScreen) layoutVoiceLabel(gtx layout.Context, rect image.Rectangle) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, "ОЗВУЧКА", face, titlePageSectionSize)
	top := rect.Min.Y + gtx.Dp(unit.Dp(5))
	w.typo.draw(gtx, "ОЗВУЧКА", face, titlePageSectionSize, mainMutedColor, image.Pt(rect.Min.X, top))
	return top + ink.Dy() + gtx.Dp(unit.Dp(7))
}

func (w *titleScreen) layoutVoices(gtx layout.Context, rect image.Rectangle) {
	w.tooltipIndex = -1
	w.tooltipKodik = false
	if rect.Empty() || len(w.item.voices) == 0 {
		return
	}
	rowHeight := gtx.Dp(titlePageVoiceHeight)
	gap := gtx.Dp(titlePageVoiceGap)
	w.voiceList.Gap = gap

	voiceCount := len(w.item.voices)
	if !w.voicesPlaced {
		w.voicesPlaced = true
		stride := rowHeight + gap
		visible := 1
		if stride > 0 {
			visible = max(1, (rect.Dy()+gap)/stride)
		}
		first := w.selectedVoice - visible/2
		maxFirst := max(0, voiceCount-visible)
		if first < 0 {
			first = 0
		}
		if first > maxFirst {
			first = maxFirst
		}
		w.voiceList.Position = layout.Position{First: first}
	}
	tailHeight := gtx.Dp(titlePageVoiceTailSpace)
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	// The synthetic tail row matches the history list: the last voice can
	// scroll up slightly so it is not pressed against the footer.
	w.voiceList.Layout(lgtx, voiceCount+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == voiceCount {
			return layout.Dimensions{Size: image.Pt(rect.Dx(), tailHeight)}
		}
		w.layoutVoiceRow(gtx, index, rect.Dx(), rowHeight)
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	stack.Pop()

	if w.tooltipIndex >= 0 && w.tooltipIndex < len(w.item.voices) {
		stride := rowHeight + gap
		if stride > 0 {
			rowTop := rect.Min.Y + (w.tooltipIndex-w.voiceList.Position.First)*stride - w.voiceList.Position.Offset
			w.tooltipAnchor = image.Pt(rect.Max.X+gtx.Dp(unit.Dp(8)), rowTop+rowHeight/2)
		}
	}
}

func (w *titleScreen) layoutVoiceRow(gtx layout.Context, index, width, height int) {
	if index < 0 || index >= len(w.item.voices) {
		return
	}
	row := &w.voices[index]
	voice := w.item.voices[index]
	selected := index == w.selectedVoice
	incomplete := !voice.complete()
	hasKodik := voice.hasKodik

	_, clicked := row.click.Update(gtx)
	if clicked {
		w.selectVoice(index)
		gtx.Execute(op.InvalidateCmd{})
	}
	if incomplete {
		_, _ = row.help.Update(gtx)
	}
	if hasKodik {
		_, _ = row.kodik.Update(gtx)
	}
	hovered := row.click.Hovered() || (incomplete && row.help.Hovered()) || (hasKodik && row.kodik.Hovered())
	hover := row.hover.update(gtx, hovered)
	helpHover := float32(0)
	if incomplete {
		helpHover = row.helpHover.update(gtx, row.help.Hovered())
		if row.help.Hovered() {
			w.tooltipIndex = index
			w.tooltipKodik = false
		}
	}
	kodikHover := float32(0)
	if hasKodik {
		kodikHover = row.kodikHover.update(gtx, row.kodik.Hovered())
		if row.kodik.Hovered() {
			w.tooltipIndex = index
			w.tooltipKodik = true
		}
	}

	rowRect := image.Rect(0, 0, width, height)
	fill := titleVoiceFill
	border := titleVoiceBorder
	text := titleVoiceText
	if selected {
		fill = titleVoiceSelectedFill
		border = titleVoiceSelectedBorder
		text = mainTextColor
	}
	if hover > 0 && !selected {
		fill = mixColor(fill, titleVoiceHoverFill, hover)
		border = mixColor(border, titleVoiceHoverBorder, hover)
	}

	radius := gtx.Dp(unit.Dp(8))
	fillRounded(gtx, border, rowRect, radius)
	fillRounded(gtx, fill, rowRect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	dotColor := mainDragDotColor
	if selected {
		dotColor = mainAccentColor1
	}
	fillCircle(gtx, image.Pt(gtx.Dp(unit.Dp(15)), height/2), gtx.Dp(unit.Dp(4)), dotColor)

	textX := gtx.Dp(unit.Dp(29))
	badgeWidth := gtx.Dp(unit.Dp(26))
	badgeHeight := gtx.Dp(unit.Dp(20))
	badgeGap := gtx.Dp(unit.Dp(6))
	badgeTop := (height - badgeHeight) / 2
	badgeRight := width - gtx.Dp(unit.Dp(10))
	// The Kodik badge keeps the right edge; the missing-episodes badge sits to
	// its left so both signals stay visible on the same row.
	kodikRect := image.Rect(badgeRight-badgeWidth, badgeTop, badgeRight, badgeTop+badgeHeight)
	if hasKodik {
		badgeRight = kodikRect.Min.X - badgeGap
	}
	helpRect := image.Rect(badgeRight-badgeWidth, badgeTop, badgeRight, badgeTop+badgeHeight)
	textRight := rowRect.Max.X - gtx.Dp(unit.Dp(10))
	if incomplete {
		textRight = helpRect.Min.X - gtx.Dp(unit.Dp(7))
	} else if hasKodik {
		textRight = kodikRect.Min.X - gtx.Dp(unit.Dp(7))
	}
	_, textInk := w.typo.measure(gtx, voice.name, font.Font{Typeface: fontFamily, Weight: font.Medium}, titlePageVoiceSize)
	textY := (height - textInk.Dy()) / 2
	drawTruncatedText(
		gtx,
		w.typo,
		voice.name,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageVoiceSize,
		text,
		image.Pt(textX, textY),
		textRight-textX,
	)

	if incomplete {
		w.drawVoiceBadge(gtx, helpRect, "!?", helpHover)
	}
	if hasKodik {
		w.drawVoiceBadge(gtx, kodikRect, "K", kodikHover)
	}

	rowRight := width - gtx.Dp(unit.Dp(42))
	if incomplete {
		rowRight = helpRect.Min.X - badgeGap
	} else if hasKodik {
		rowRight = kodikRect.Min.X - badgeGap
	}
	rowHit := image.Rect(0, 0, rowRight, height)
	w.layoutHitArea(gtx, rowHit, &row.click, true, nil)
	if incomplete {
		w.layoutHitArea(gtx, helpRect, &row.help, true, nil)
	}
	if hasKodik {
		w.layoutHitArea(gtx, kodikRect, &row.kodik, true, nil)
	}
}

// drawVoiceBadge paints one of the small square markers at the right of a
// voice row. hover is the badge's own hover progress.
func (w *titleScreen) drawVoiceBadge(gtx layout.Context, rect image.Rectangle, label string, hover float32) {
	fill := mixColor(titleHelpFill, titleHelpHoverFill, hover)
	border := mixColor(titleHelpBorder, titleHelpHoverBorder, hover)
	radius := gtx.Dp(unit.Dp(5))
	fillRounded(gtx, fill, rect, radius)
	strokeRoundedRect(gtx, rect.Inset(gtx.Dp(unit.Dp(1))), float32(radius), float32(gtx.Dp(unit.Dp(1))), border)
	w.typo.drawCentered(
		gtx,
		label,
		font.Font{Typeface: fontFamily, Weight: font.Bold},
		unit.Sp(9.5),
		titleVoiceIncompleteText,
		centerOf(rect),
	)
}
