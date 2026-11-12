package shell

import (
	"image"

	qt "github.com/mappu/miqt/qt6"
)

// onPress routes a left-button press.
func (s *Shell) onPress(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	s.press = RegionNone
	s.pressMenuRow = -1

	if s.menuOpen {
		menu, _, rows, _ := s.menuLayout(l)
		if pos.In(menu) {
			for i, row := range rows {
				if pos.In(row) {
					s.pressMenuRow = i
					break
				}
			}
			return
		}
		s.dismissMenu()
		s.widget.Update()
		return
	}

	if !l.maximized {
		if edges, ok := s.edgeAt(l, pos); ok {
			if handle := s.widget.WindowHandle(); handle != nil {
				handle.StartSystemResize(edges)
			}
			return
		}
	}

	if screen := s.interactive(); screen != nil && s.inScreen(l, pos) {
		s.screenPress = true
		s.press = RegionNone
		screen.Pointer(PointerEvent{Action: PointerDown, Pos: pos})
		s.widget.Update()
		return
	}

	switch {
	case pos.In(l.close):
		s.press = RegionClose
	case pos.In(l.maximize):
		s.press = RegionMaximize
	case pos.In(l.minimize):
		s.press = RegionMinimize
	case pos.In(l.trigger):
		s.press = RegionSite
	case pos.Y < l.headerDivider:
		s.press = RegionNone
		if handle := s.widget.WindowHandle(); handle != nil {
			handle.StartSystemMove()
		}
	default:
		s.press = RegionNone
	}
}

// onRelease completes a press. Clicks count only when the release lands on the
// same region.
func (s *Shell) onRelease(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()

	if s.menuOpen {
		_, _, rows, sites := s.menuLayout(l)
		if s.pressMenuRow >= 0 && s.pressMenuRow < len(rows) && pos.In(rows[s.pressMenuRow]) {
			s.switchSite(sites[s.pressMenuRow])
			s.widget.Update()
		}
		s.pressMenuRow = -1
		return
	}

	if s.screenPress {
		s.screenPress = false
		if screen := s.interactive(); screen != nil {
			screen.Pointer(PointerEvent{Action: PointerUp, Pos: pos})
			s.widget.Update()
		}
		return
	}

	pressed := s.press
	s.press = RegionNone
	switch pressed {
	case RegionSite:
		if pos.In(l.trigger) {
			if s.menuOpen {
				s.dismissMenu()
			} else {
				s.menuOpen = true
				s.pressMenuRow = -1
			}
			s.widget.Update()
		}
	case RegionMinimize:
		if pos.In(l.minimize) {
			s.widget.ShowMinimized()
		}
	case RegionMaximize:
		if pos.In(l.maximize) {
			s.toggleMaximize()
		}
	case RegionClose:
		if pos.In(l.close) {
			s.widget.Close()
		}
	}
}

// onMove updates the hovered region, the menu row highlight and the cursor.
func (s *Shell) onMove(e *qt.QMouseEvent) {
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	region := s.regionAt(l, pos)
	row := -1
	if s.menuOpen {
		_, _, rows, _ := s.menuLayout(l)
		for i, r := range rows {
			if pos.In(r) {
				row = i
				break
			}
		}
	}

	screen := s.interactive()
	if screen != nil && !s.menuOpen {
		if s.screenPress || s.inScreen(l, pos) {
			if screen.Pointer(PointerEvent{Action: PointerMove, Pos: pos}) {
				s.widget.Update()
			}
		} else if screen.Pointer(PointerEvent{Action: PointerLeave}) {
			s.widget.Update()
		}
	}

	changed := region != s.hover || row != s.menuRowHover
	s.hover = region
	s.menuRowHover = row

	switch {
	case screen != nil && s.screenPress:
		s.setCursor(screen.Cursor(pos))
	case !l.maximized && row < 0 && region == RegionNone:
		if edges, ok := s.edgeAt(l, pos); ok {
			s.setCursor(edgeCursor(edges))
		} else {
			s.setCursor(qt.ArrowCursor)
		}
	case row >= 0:
		s.setCursor(qt.PointingHandCursor)
	case region == RegionSite || region == RegionMinimize || region == RegionMaximize || region == RegionClose:
		s.setCursor(qt.PointingHandCursor)
	case screen != nil && s.inScreen(l, pos):
		s.setCursor(screen.Cursor(pos))
	default:
		s.setCursor(qt.ArrowCursor)
	}
	if changed {
		s.widget.Update()
	}
}

// onWheel forwards a wheel event to the screen under the pointer.
func (s *Shell) onWheel(e *qt.QWheelEvent) {
	screen := s.interactive()
	if screen == nil || s.menuOpen {
		return
	}
	point := e.Position()
	pos := image.Pt(int(point.X()), int(point.Y()))
	l := s.computeLayout()
	if !s.inScreen(l, pos) {
		return
	}
	delta := e.AngleDelta().Y() / 120
	if e.HasPixelDelta() {
		delta = e.PixelDelta().Y()
	}
	if delta == 0 {
		return
	}
	if screen.Wheel(WheelEvent{Pos: pos, Delta: int(delta)}) {
		s.widget.Update()
	}
	e.Accept()
}

// inScreen reports whether pos belongs to the content or footer band.
func (s *Shell) inScreen(l layout, pos image.Point) bool {
	return pos.In(l.contentRect()) || pos.In(l.footerRect())
}

// onDoubleClick toggles maximize on the empty part of the header.
func (s *Shell) onDoubleClick(e *qt.QMouseEvent) {
	if e.Button() != qt.LeftButton {
		return
	}
	pos := image.Pt(e.X(), e.Y())
	l := s.computeLayout()
	if pos.Y >= l.headerDivider {
		return
	}
	if pos.In(l.trigger) || pos.In(l.minimize) || pos.In(l.maximize) || pos.In(l.close) {
		return
	}
	s.toggleMaximize()
}

// toggleMaximize swaps between the maximized and normal window states.
func (s *Shell) toggleMaximize() {
	if s.widget.IsMaximized() {
		s.widget.ShowNormal()
	} else {
		s.widget.ShowMaximized()
	}
}

// edgeAt reports the window edges under pos for a system resize.
func (s *Shell) edgeAt(l layout, pos image.Point) (qt.Edge, bool) {
	var edges qt.Edge
	if pos.X < resizeInset {
		edges |= qt.LeftEdge
	} else if pos.X >= l.size.X-resizeInset {
		edges |= qt.RightEdge
	}
	if pos.Y < resizeInset {
		edges |= qt.TopEdge
	} else if pos.Y >= l.size.Y-resizeInset {
		edges |= qt.BottomEdge
	}
	return edges, edges != 0
}

// regionAt maps a point to the control under it.
func (s *Shell) regionAt(l layout, pos image.Point) Region {
	switch {
	case pos.In(l.close):
		return RegionClose
	case pos.In(l.maximize):
		return RegionMaximize
	case pos.In(l.minimize):
		return RegionMinimize
	case pos.In(l.trigger):
		return RegionSite
	case pos.In(l.footerRect()):
		return RegionFooter
	}
	return RegionNone
}

// edgeCursor returns the resize cursor for a set of window edges.
func edgeCursor(edges qt.Edge) qt.CursorShape {
	switch edges {
	case qt.LeftEdge, qt.RightEdge:
		return qt.SizeHorCursor
	case qt.TopEdge, qt.BottomEdge:
		return qt.SizeVerCursor
	case qt.TopEdge | qt.LeftEdge, qt.BottomEdge | qt.RightEdge:
		return qt.SizeFDiagCursor
	case qt.TopEdge | qt.RightEdge, qt.BottomEdge | qt.LeftEdge:
		return qt.SizeBDiagCursor
	}
	return qt.ArrowCursor
}

// setCursor changes the widget cursor only when the shape differs.
func (s *Shell) setCursor(shape qt.CursorShape) {
	if s.cursor == shape {
		return
	}
	s.cursor = shape
	cursor, ok := s.cursors[shape]
	if !ok {
		cursor = qt.NewQCursor2(shape)
		s.cursors[shape] = cursor
	}
	s.widget.SetCursor(cursor)
}
