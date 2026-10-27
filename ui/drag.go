package ui

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/arfush/yoanime/internal/library"
)

// dragState tracks a history row being reordered by its six-dot handle.
// index is the current slot of the dragged title. pos is the floating card's
// top-left in main-content coordinates.
type dragState struct {
	active  bool
	index   int
	pointer pointer.ID

	startIndex            int
	originalSelected      int
	originalLoadingTarget int

	grab   image.Point
	pos    image.Point
	width  int
	height int
	listY  int
	moved  bool
}

// addHistoryDragInput installs a pointer handler over the six-dot handle
// column. Keeping it out of the rest of the row leaves titles and controls
// fully clickable.
func (m *mainWindow) addHistoryDragInput(gtx layout.Context, rect image.Rectangle) {
	left := gtx.Dp(mainRowPadding)
	right := left + gtx.Dp(mainDragHandleWidth)
	area := image.Rect(left, 0, right, rect.Dy())
	if area.Empty() {
		return
	}
	stack := clip.Rect(area).Push(gtx.Ops)
	cursor := pointer.CursorGrab
	if m.drag.active {
		cursor = pointer.CursorGrabbing
	}
	cursor.Add(gtx.Ops)
	event.Op(gtx.Ops, &m.dragTag)
	stack.Pop()
}

func (m *mainWindow) handleHistoryDrag(gtx layout.Context, rect image.Rectangle, rowHeight, gap int) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &m.dragTag,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}

		switch e.Kind {
		case pointer.Press:
			if m.drag.active {
				continue
			}
			if e.Source == pointer.Mouse && e.Buttons != pointer.ButtonPrimary {
				continue
			}
			x := int(e.Position.X)
			handleLeft := gtx.Dp(mainRowPadding)
			handleRight := handleLeft + gtx.Dp(mainDragHandleWidth)
			if x < handleLeft || x >= handleRight {
				continue
			}
			index := m.historyIndexAt(int(e.Position.Y), rowHeight, gap)
			if index < 0 {
				continue
			}
			rowTop := m.historyRowTop(index, rowHeight+gap)
			m.drag = dragState{
				active:                true,
				index:                 index,
				pointer:               e.PointerID,
				startIndex:            index,
				originalSelected:      m.selected,
				originalLoadingTarget: m.loadingTarget,
				grab: image.Pt(
					int(e.Position.X),
					int(e.Position.Y)-rowTop,
				),
				pos:    image.Pt(rect.Min.X, rect.Min.Y+rowTop),
				width:  rect.Dx(),
				height: rowHeight,
				listY:  int(e.Position.Y),
			}
			m.selected = index
			m.focusHistory(gtx)
			gtx.Execute(pointer.GrabCmd{Tag: &m.dragTag, ID: e.PointerID})
			gtx.Execute(op.InvalidateCmd{})

		case pointer.Move, pointer.Drag:
			if !m.drag.active || e.PointerID != m.drag.pointer {
				continue
			}
			m.drag.moved = true
			m.drag.listY = int(e.Position.Y)
			m.updateDragPosition(rect, int(e.Position.X), int(e.Position.Y))
			if !cursorInWindow() {
				m.cancelDrag(gtx)
				continue
			}
			target := m.historyIndexAt(m.drag.listY, rowHeight, gap)
			if target < 0 || target == m.drag.index {
				continue
			}
			m.moveHistoryItem(m.drag.index, target)
			m.drag.index = target
			m.selected = target
			gtx.Execute(op.InvalidateCmd{})

		case pointer.Release:
			if !m.drag.active || e.PointerID != m.drag.pointer {
				continue
			}
			if !cursorInWindow() {
				m.cancelDrag(gtx)
				continue
			}
			// A fast drag can be coalesced into the release event, so apply
			// the final position before ending the gesture.
			m.drag.listY = int(e.Position.Y)
			target := m.historyIndexAt(m.drag.listY, rowHeight, gap)
			if target >= 0 && target != m.drag.index {
				m.moveHistoryItem(m.drag.index, target)
				m.selected = target
			}
			m.drag = dragState{index: -1}
			m.persistHistoryOrder()
			gtx.Execute(op.InvalidateCmd{})

		case pointer.Cancel:
			if !m.drag.active || e.PointerID != m.drag.pointer {
				continue
			}
			m.cancelDrag(gtx)
		}
	}

	// Captured pointer events can stop at the window edge. Polling the real
	// cursor makes leaving the window deterministic even when no release event
	// is delivered.
	if m.drag.active && !cursorInWindow() {
		m.cancelDrag(gtx)
	}
	if !m.drag.active {
		return
	}

	// Holding a card near the top or bottom edge scrolls the history. The
	// dragged row is re-evaluated after scrolling because the row under the
	// pointer has changed.
	if m.autoScrollHistoryDrag(gtx, rect, rowHeight, gap) {
		gtx.Execute(op.InvalidateCmd{})
	}
	target := m.historyIndexAt(m.drag.listY, rowHeight, gap)
	if target >= 0 && target != m.drag.index {
		m.moveHistoryItem(m.drag.index, target)
		m.drag.index = target
		m.selected = target
		gtx.Execute(op.InvalidateCmd{})
	}
}

// autoScrollHistoryDrag scrolls the list while a dragged card is held near an
// edge. The speed ramps up as the pointer moves further into the edge zone,
// and stops at either end of the history.
func (m *mainWindow) autoScrollHistoryDrag(gtx layout.Context, rect image.Rectangle, rowHeight, gap int) bool {
	if !m.drag.active || !m.drag.moved || rect.Dy() <= 0 {
		return false
	}
	stride := rowHeight + gap
	if stride <= 0 {
		return false
	}
	edge := gtx.Dp(unit.Dp(56))
	if edge <= 0 {
		return false
	}
	if edge*2 > rect.Dy() {
		edge = rect.Dy() / 2
	}
	if edge <= 0 {
		return false
	}

	first, offset := m.listAnchor(stride)
	y := m.drag.listY
	const maxItemsPerFrame = 0.08
	var items float32
	if y < edge {
		if first <= 0 && offset <= 0 {
			return false
		}
		strength := float32(edge-y) / float32(edge)
		if strength > 1.5 {
			strength = 1.5
		}
		items = -maxItemsPerFrame * strength
	} else if y > rect.Dy()-edge {
		if first+m.list.Position.Count >= len(m.history)+1 {
			return false
		}
		strength := float32(y-(rect.Dy()-edge)) / float32(edge)
		if strength > 1.5 {
			strength = 1.5
		}
		items = maxItemsPerFrame * strength
	}
	if items == 0 {
		return false
	}
	m.list.ScrollBy(items)
	return true
}

func (m *mainWindow) updateDragPosition(rect image.Rectangle, x, y int) {
	m.drag.pos = image.Pt(
		rect.Min.X+x-m.drag.grab.X,
		rect.Min.Y+y-m.drag.grab.Y,
	)
}

// listAnchor normalizes the list position into a first index and an offset
// smaller than one row. ScrollBy can leave the offset beyond a full row until
// the next Layout pass.
func (m *mainWindow) listAnchor(stride int) (int, int) {
	if stride <= 0 {
		return m.list.Position.First, m.list.Position.Offset
	}
	first, offset := m.list.Position.First, m.list.Position.Offset
	if offset >= stride {
		first += offset / stride
		offset %= stride
	}
	for offset < 0 {
		first--
		offset += stride
	}
	return first, offset
}

// historyRowTop returns the top of a row in list-viewport coordinates.
func (m *mainWindow) historyRowTop(index, stride int) int {
	if stride <= 0 {
		return 0
	}
	first, offset := m.listAnchor(stride)
	return (index-first)*stride - offset
}

// cancelDrag puts the dragged title back into its original slot.
func (m *mainWindow) cancelDrag(gtx layout.Context) {
	if !m.drag.active {
		return
	}
	m.moveHistoryItem(m.drag.index, m.drag.startIndex)
	m.selected = m.drag.originalSelected
	m.loadingTarget = m.drag.originalLoadingTarget
	m.drag = dragState{index: -1}
	gtx.Execute(op.InvalidateCmd{})
}

// historyIndexAt maps a y coordinate in list viewport space to a title index.
// Points above the viewport mean the first visible row; points below it are
// clamped to the last title. The gap between cards belongs to the preceding
// card.
func (m *mainWindow) historyIndexAt(y, rowHeight, gap int) int {
	if len(m.history) == 0 || rowHeight <= 0 {
		return -1
	}
	stride := rowHeight + gap
	if stride <= 0 {
		return -1
	}
	first, offset := m.listAnchor(stride)
	rel := y + offset
	index := first
	if rel > 0 {
		index += rel / stride
	}
	if index < 0 {
		return 0
	}
	if index >= len(m.history) {
		return len(m.history) - 1
	}
	return index
}

// layoutOverlay draws the dragged row above the window chrome. origin is the
// top-left of the main content in window coordinates.
func (m *mainWindow) layoutOverlay(gtx layout.Context, origin image.Point) {
	if m.statusMenu.open {
		itemID := m.statusMenu.id
		m.statusMenu.layout(gtx, m.typo, origin, func(status library.UserStatus) {
			if m.onUserStatus == nil {
				return
			}
			if item := m.historyItemByID(itemID); item.id != 0 {
				_ = m.onUserStatus(item, status)
			}
		})
	}
	if !m.drag.active || m.drag.index < 0 || m.drag.index >= len(m.history) {
		return
	}
	if m.drag.width <= 0 || m.drag.height <= 0 {
		return
	}
	rect := image.Rect(
		origin.X+m.drag.pos.X,
		origin.Y+m.drag.pos.Y,
		origin.X+m.drag.pos.X+m.drag.width,
		origin.Y+m.drag.pos.Y+m.drag.height,
	)
	shadow := rect.Add(image.Pt(0, gtx.Dp(unit.Dp(5))))
	fillRounded(gtx, color.NRGBA{A: 0x78}, shadow, gtx.Dp(mainRowRadius))

	item := m.history[m.drag.index]
	geom := m.historyRowGeometry(gtx, m.drag.width, m.drag.height, item)
	m.drawHistoryRowCard(gtx, rect, item, geom, historyRowVisual{
		selected:    true,
		dragging:    true,
		itemLoading: m.loading && m.loadingAnimateRow && m.loadingTarget == m.drag.index,
	})
}

// moveHistoryItem moves one title while keeping selection and the active
// loading indicator attached to their content.
func (m *mainWindow) moveHistoryItem(from, to int) {
	if from == to || from < 0 || to < 0 || from >= len(m.history) || to >= len(m.history) {
		return
	}
	item := m.history[from]
	if from < to {
		copy(m.history[from:to], m.history[from+1:to+1])
	} else {
		copy(m.history[to+1:from+1], m.history[to:from])
	}
	m.history[to] = item
	m.selected = reorderIndex(m.selected, from, to)
	m.loadingTarget = reorderIndex(m.loadingTarget, from, to)
	if m.filter == filterAll {
		// The unfiltered view owns the all-titles order. Keep the complete
		// order in sync so the database write and later filters see the move.
		m.allHistory = append(m.allHistory[:0], m.history...)
		return
	}
	m.syncVisibleStatusOrder()
}

// syncVisibleStatusOrder copies the current filtered order into the per-status
// order of the matching all-history rows.
func (m *mainWindow) syncVisibleStatusOrder() {
	if m.filter == filterAll {
		return
	}
	for index := range m.history {
		if allIndex := historyIndexByID(m.allHistory, m.history[index].id); allIndex >= 0 {
			m.allHistory[allIndex].statusOrder = int64(index)
		}
		m.history[index].statusOrder = int64(index)
	}
}
