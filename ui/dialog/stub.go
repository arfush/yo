// Package dialog holds every application dialog. Dialogs live directly in this
// package; there are no per-feature subpackages.
//
// Imports: std, gioui.org/*, ui/appearance, ui/kit, ui/navigator.
// Must not import: root, ui/shell.
//
// Invariants: Layout never blocks or starts goroutines. A dialog closes itself
// through the navigator handler; the shell only draws it.
package dialog

import (
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/arfush/yo/ui/navigator"
)

// Stub is the empty fallback dialog. It owns no content and closes on Escape.
type Stub struct {
	handler navigator.Handler
	tag     struct{}
}

// NewStub returns the fallback dialog.
func NewStub(handler navigator.Handler) *Stub {
	if handler == nil {
		panic("dialog: nil handler")
	}
	return &Stub{handler: handler}
}

// Layout handles the Escape key; the shell owns the surface behind the dialog.
func (s *Stub) Layout(gtx layout.Context) {
	if !gtx.Focused(&s.tag) {
		gtx.Execute(key.FocusCmd{Tag: &s.tag})
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &s.tag, Name: key.NameEscape})
		if !ok {
			break
		}
		if event, ok := ev.(key.Event); ok && event.State == key.Press {
			s.handler.Navigate(navigator.Back{})
			return
		}
	}
}

// LayoutFooter leaves the footer band empty while the stub dialog is open.
func (s *Stub) LayoutFooter(layout.Context) {}
