// Package navigator owns the screen stack and the route vocabulary. A screen
// requests a change through Handler; the navigator resolves the route through
// a factory supplied by the composition root and installs the built screen
// into the shell.
//
// Imports: gioui.org/layout, ui/shell.
// Must not import: root, ui/screen, core/*.
// Invariants: Navigate only queues and invalidates; Apply runs on the UI
// goroutine at frame start before layout. A screen is never nil, and the
// navigator never imports a concrete screen. Close is idempotent.
package navigator

import (
	"fmt"

	"github.com/arfush/yo/ui/shell"
)

// Installer is the navigator's narrow view of the window frame. The shell
// implements it; the navigator only installs the visible screen and its title.
type Installer interface {
	SetScreen(shell.Screen)
	SetTitle(string)
}

// Options configures a navigator. Installer, Invalidate and Startup are
// required; Routes maps the destinations known to the composition root.
type Options struct {
	Installer  Installer
	Invalidate func()
	Startup    ScreenFactory
	Routes     map[Route]ScreenFactory
}

var _ Handler = (*Navigator)(nil)

// Navigator owns the ordered screen stack. It builds screens only through the
// factories supplied in Options and never draws them.
type Navigator struct {
	installer  Installer
	invalidate func()
	routes     map[Route]ScreenFactory
	entries    []entry
	pending    []Intent
	nextID     uint64
	active     bool
	closed     bool
}

// entry is one stack frame. The id lets Apply detect a change without
// comparing Screen interface values, which may hold non-comparable types.
type entry struct {
	screen Screen
	id     uint64
}

// New builds the navigator, creates the startup screen and installs it. The
// startup screen is shown on the first Apply, before the first layout.
func New(opts Options) *Navigator {
	if opts.Installer == nil {
		panic("navigator: nil installer")
	}
	if opts.Invalidate == nil {
		panic("navigator: nil invalidate")
	}
	if opts.Startup == nil {
		panic("navigator: nil startup factory")
	}
	n := &Navigator{
		installer:  opts.Installer,
		invalidate: opts.Invalidate,
		routes:     opts.Routes,
	}
	startup := opts.Startup(n)
	if startup == nil {
		panic("navigator: nil startup screen")
	}
	n.entries = append(n.entries, n.newEntry(startup))
	n.install(startup)
	return n
}

// Navigate queues an intent and asks the window for the next frame. The stack
// changes only when Apply runs.
func (n *Navigator) Navigate(intent Intent) {
	if n.closed {
		return
	}
	n.pending = append(n.pending, intent)
	n.invalidate()
}

// Apply runs every queued intent, installs the visible screen and runs the
// lifecycle. It must run on the UI goroutine before the frame is laid out.
func (n *Navigator) Apply() {
	if n.closed {
		return
	}
	if len(n.pending) == 0 {
		n.present()
		return
	}
	beforeID := n.currentID()
	before := n.current()
	pending := n.pending
	n.pending = nil

	screens := make([]Screen, 0, len(pending))
	for _, intent := range pending {
		screens = append(screens, n.build(intent))
	}
	for _, screen := range screens {
		n.entries = append(n.entries, n.newEntry(screen))
	}

	after := n.current()
	if n.currentID() == beforeID {
		return
	}
	if n.active && before != nil {
		before.OnHidden()
	}
	n.install(after)
	n.active = true
	after.OnShown()
}

// Close hides the active screen and closes every screen on the stack. It is
// idempotent and discards queued intents.
func (n *Navigator) Close() {
	if n.closed {
		return
	}
	n.closed = true
	if n.active {
		if current := n.current(); current != nil {
			current.OnHidden()
		}
		n.active = false
	}
	for _, entry := range n.entries {
		entry.screen.OnClose()
	}
	n.entries = nil
	n.pending = nil
}

// build resolves a route and constructs its screen. An unknown route or a nil
// screen is a composition error and panics.
func (n *Navigator) build(intent Intent) Screen {
	factory, ok := n.routes[intent.Route]
	if !ok {
		panic(fmt.Sprintf("navigator: unknown route %q", intent.Route))
	}
	screen := factory(n)
	if screen == nil {
		panic(fmt.Sprintf("navigator: nil screen for route %q", intent.Route))
	}
	return screen
}

// present shows the current screen once, on the first frame.
func (n *Navigator) present() {
	if n.active {
		return
	}
	current := n.current()
	if current == nil {
		return
	}
	n.active = true
	current.OnShown()
}

func (n *Navigator) install(screen Screen) {
	n.installer.SetScreen(screen)
	n.installer.SetTitle(screen.Title())
}

func (n *Navigator) current() Screen {
	if len(n.entries) == 0 {
		return nil
	}
	return n.entries[len(n.entries)-1].screen
}

func (n *Navigator) currentID() uint64 {
	if len(n.entries) == 0 {
		return 0
	}
	return n.entries[len(n.entries)-1].id
}

func (n *Navigator) newEntry(screen Screen) entry {
	n.nextID++
	return entry{screen: screen, id: n.nextID}
}
