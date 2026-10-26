package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type filterRowMetrics struct {
	widths  []int
	gap     int
	size    unit.Sp
	padding unit.Dp
	total   int
}

type filterPillStep struct {
	size    unit.Sp
	padding unit.Dp
	gap     unit.Dp
}

// filterPillSteps are tried from the designed metrics to a compact version.
// The last step is used even if the window is smaller still, so the filters do
// not disappear completely on an unexpectedly narrow viewport.
var filterPillSteps = [...]filterPillStep{
	{size: mainFilterSize, padding: mainFilterPaddingX, gap: mainFilterGap},
	{size: 10, padding: unit.Dp(7), gap: unit.Dp(4)},
	{size: 9, padding: unit.Dp(5), gap: unit.Dp(3)},
	{size: 8, padding: unit.Dp(3), gap: unit.Dp(2)},
}

// measureFilterRow picks the largest pill metrics that fit in available. On a
// wide window the gaps grow a little, up to mainFilterMaxGap, so the row still
// follows the window, but the pills stay a compact cluster.
func measureFilterRow(gtx layout.Context, typo *typography, available int) filterRowMetrics {
	measure := func(step filterPillStep) filterRowMetrics {
		metrics := filterRowMetrics{
			widths:  make([]int, len(userStatusFilterOptions)),
			size:    step.size,
			padding: step.padding,
			gap:     gtx.Dp(step.gap),
		}
		for index, option := range userStatusFilterOptions {
			metrics.widths[index] = filterPillWidth(gtx, typo, option.label, step.size, step.padding)
			metrics.total += metrics.widths[index]
		}
		metrics.total += metrics.gap * (len(metrics.widths) - 1)
		return metrics
	}

	metrics := measure(filterPillSteps[0])
	if metrics.total > available {
		for _, step := range filterPillSteps[1:] {
			metrics = measure(step)
			if metrics.total <= available {
				break
			}
		}
		return metrics
	}

	maxGap := gtx.Dp(mainFilterMaxGap)
	if metrics.gap < maxGap && len(metrics.widths) > 1 {
		grow := (available - metrics.total) / (len(metrics.widths) - 1)
		if grow > maxGap-metrics.gap {
			grow = maxGap - metrics.gap
		}
		if grow > 0 {
			metrics.gap += grow
			metrics.total += grow * (len(metrics.widths) - 1)
		}
	}
	return metrics
}

// historyFilterSpan is the stable free area used by the filter row. The count
// reserve, rather than the current "(n)" width, keeps the row from moving when
// titles are added or removed. The right edge follows the clear action, so the
// span still adapts to window resizes.
func historyFilterSpan(gtx layout.Context, rect, titleInk, countInk, clearRect image.Rectangle) (int, int) {
	countReserve := max(gtx.Dp(mainFilterReserve), countInk.Dx())
	left := rect.Min.X + titleInk.Dx() + gtx.Dp(unit.Dp(7)) + countReserve + gtx.Dp(unit.Dp(14))
	right := clearRect.Min.X - gtx.Dp(unit.Dp(12))
	return left, right
}

// centeredFilterRowStart keeps a row inside its available span. A row wider
// than the span starts at the left edge; at that point the header clips it
// rather than dropping the controls.
func centeredFilterRowStart(left, available, total int) int {
	if total >= available {
		return left
	}
	return left + (available-total)/2
}

// layoutHistoryFilters draws the six status filters centred in the stable
// middle span. Inactive pills are neutral gray; selecting one reveals its
// status color.
func (m *mainWindow) layoutHistoryFilters(gtx layout.Context, left, right, centerY int) {
	available := right - left
	if available <= 0 {
		return
	}
	metrics := measureFilterRow(gtx, m.typo, available)
	x := centeredFilterRowStart(left, available, metrics.total)
	height := gtx.Dp(mainFilterHeight)
	clickedIndex := -1
	for index, option := range userStatusFilterOptions {
		rect := image.Rect(x, centerY-height/2, x+metrics.widths[index], centerY+height/2)
		click := &m.filterClicks[index]
		hover := m.filterHovers[index].update(gtx, click.Hovered())
		if click.Clicked(gtx) {
			clickedIndex = index
		}
		m.layoutHitArea(gtx, rect, click, true, func(gtx layout.Context) {
			drawFilterPill(gtx, m.typo, image.Rect(0, 0, rect.Dx(), rect.Dy()), option, m.filter == option.filter, hover, metrics.size)
		})
		x += metrics.widths[index] + metrics.gap
	}
	if clickedIndex >= 0 {
		m.setFilter(gtx, userStatusFilterOptions[clickedIndex].filter)
	}
}

// setFilter applies a history filter from either a mouse click or a keyboard
// shortcut. It is a no-op when the filter is already selected.
func (m *mainWindow) setFilter(gtx layout.Context, next userStatusFilter) {
	if next == m.filter {
		return
	}
	m.filter = next
	m.applyFilter()
	gtx.Execute(op.InvalidateCmd{})
}

// stepFilter moves through the filter pills in their visual order and wraps
// around, so the left and right arrows always have a visible result.
func (m *mainWindow) stepFilter(gtx layout.Context, delta int) {
	count := len(userStatusFilterOptions)
	if count == 0 {
		return
	}
	current := 0
	for index, option := range userStatusFilterOptions {
		if option.filter == m.filter {
			current = index
			break
		}
	}
	delta %= count
	next := (current + delta + count) % count
	m.setFilter(gtx, userStatusFilterOptions[next].filter)
}
