package navigator

import (
	"fmt"

	"github.com/arfush/yo/ui/shell"
)

// Installer is the navigator's narrow view of the window frame. The shell
// implements it; the navigator only installs the visible content and its title.
type Installer interface {
	SetScreen(shell.Screen)
	SetTitle(string)
	SetDialog(shell.Dialog)
}

// Options configures a navigator. Installer, Invalidate and Startup are
// required; future screens and dialogs add their own factories here.
type Options struct {
	Installer  Installer
	Invalidate func()
	Startup    ScreenFactory
}

var _ Handler = (*Navigator)(nil)

// Navigator owns the ordered screen and dialog stacks. It builds screens only
// through the factories supplied in Options and never draws them.
type Navigator struct {
	installer  Installer
	invalidate func()
	entries    []Screen
	dialogs    []shell.Dialog
	pending    []Intent
	active     bool
	closed     bool
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
	}
	startup := opts.Startup(n)
	if startup == nil {
		panic("navigator: nil startup screen")
	}
	n.entries = append(n.entries, startup)
	n.install(startup)
	return n
}

// Navigate queues an intent and asks the window for the next frame. The stacks
// change only when Apply runs.
func (n *Navigator) Navigate(intent Intent) {
	if n.closed {
		return
	}
	n.pending = append(n.pending, intent)
	n.invalidate()
}

// Apply shows the startup screen and runs every queued intent. It must run on
// the UI goroutine before the frame is laid out.
func (n *Navigator) Apply() {
	if n.closed {
		return
	}
	n.present()
	if len(n.pending) == 0 {
		return
	}
	pending := n.pending
	n.pending = nil
	for _, intent := range pending {
		n.apply(intent)
	}
}

// Close hides the active screen, clears the dialog stack and closes every
// screen on the stack. It is idempotent and discards queued intents.
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
	if len(n.dialogs) > 0 {
		n.dialogs = nil
		n.installer.SetDialog(nil)
	}
	for _, screen := range n.entries {
		screen.OnClose()
	}
	n.entries = nil
	n.pending = nil
}

// apply performs one queued intent. An unknown intent is a composition error
// and panics.
func (n *Navigator) apply(intent Intent) {
	switch intent.(type) {
	case Back:
		n.back()
	default:
		panic(fmt.Sprintf("navigator: unknown intent %T", intent))
	}
}

// push installs a screen above the active one and runs its lifecycle.
func (n *Navigator) push(screen Screen) {
	if screen == nil {
		panic("navigator: nil screen")
	}
	if n.active {
		if current := n.current(); current != nil {
			current.OnHidden()
		}
	}
	n.entries = append(n.entries, screen)
	n.install(screen)
	n.active = true
	screen.OnShown()
}

// back closes the top dialog. When no dialog is open it returns to the previous
// screen; at the root screen it does nothing.
func (n *Navigator) back() {
	if len(n.dialogs) > 0 {
		n.dialogs = n.dialogs[:len(n.dialogs)-1]
		n.installDialog()
		return
	}
	if len(n.entries) <= 1 {
		return
	}
	top := n.current()
	top.OnHidden()
	n.entries = n.entries[:len(n.entries)-1]
	top.OnClose()
	next := n.current()
	n.install(next)
	next.OnShown()
}

// openDialog pushes a dialog above the active screen.
func (n *Navigator) openDialog(dialog shell.Dialog) {
	if dialog == nil {
		panic("navigator: nil dialog")
	}
	n.dialogs = append(n.dialogs, dialog)
	n.installer.SetDialog(dialog)
}

// installDialog makes the top dialog visible, or clears the shell when the
// dialog stack is empty.
func (n *Navigator) installDialog() {
	if len(n.dialogs) == 0 {
		n.installer.SetDialog(nil)
		return
	}
	n.installer.SetDialog(n.dialogs[len(n.dialogs)-1])
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
	return n.entries[len(n.entries)-1]
}
