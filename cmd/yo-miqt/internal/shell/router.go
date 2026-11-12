package shell

import (
	"image"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// ScreenID identifies a registered screen.
type ScreenID string

// ScreenHome is the home screen.
const ScreenHome ScreenID = "home"

// Router is a small screen registry. It implements Screen, FooterScreen and
// Interactive by forwarding to the active screen.
type Router struct {
	screens map[ScreenID]Screen
	active  Screen
	id      ScreenID
}

// NewRouter returns an empty router.
func NewRouter() *Router {
	return &Router{screens: make(map[ScreenID]Screen)}
}

// Register adds a screen. Registering the same ID replaces it.
func (r *Router) Register(id ScreenID, screen Screen) {
	if screen == nil {
		panic("shell: nil screen " + string(id))
	}
	r.screens[id] = screen
}

// Show activates a registered screen.
func (r *Router) Show(id ScreenID) {
	screen, ok := r.screens[id]
	if !ok {
		panic("shell: unknown screen " + string(id))
	}
	r.active = screen
	r.id = id
}

// Paint draws the active screen.
func (r *Router) Paint(p *qt.QPainter, content image.Rectangle, palette appearance.Palette) {
	if r.active != nil {
		r.active.Paint(p, content, palette)
	}
}

// PaintFooter draws the footer of the active screen when it has one.
func (r *Router) PaintFooter(p *qt.QPainter, footer image.Rectangle, palette appearance.Palette) {
	if screen, ok := r.active.(FooterScreen); ok {
		screen.PaintFooter(p, footer, palette)
	}
}

// Pointer forwards a pointer event to the active screen.
func (r *Router) Pointer(event PointerEvent) bool {
	if screen, ok := r.active.(Interactive); ok {
		return screen.Pointer(event)
	}
	return false
}

// Wheel forwards a wheel event to the active screen.
func (r *Router) Wheel(event WheelEvent) bool {
	if screen, ok := r.active.(Interactive); ok {
		return screen.Wheel(event)
	}
	return false
}

// Cursor returns the active screen's cursor.
func (r *Router) Cursor(pos image.Point) qt.CursorShape {
	if screen, ok := r.active.(Interactive); ok {
		return screen.Cursor(pos)
	}
	return qt.ArrowCursor
}
