package navigator

import "github.com/arfush/yo/ui/shell"

// Intent is a navigation request. Concrete intents are declared in this package
// next to their factories; a screen only emits the value through Handler. The
// interface is empty on purpose: an intent is a typed event that carries the
// data the destination needs, never a screen or a stack operation.
type Intent any

// Handler is the navigation surface a screen or dialog receives. The navigator
// implements it; a screen never touches the stack directly.
type Handler interface {
	Navigate(Intent)
}

// Back closes the top dialog or returns to the previous screen.
type Back struct{}

// StubDialog opens the empty fallback dialog. Real dialogs add their own intent
// and factory next to this one.
type StubDialog struct{}

// StubDialogFactory builds the stub dialog. The navigator only knows the shell
// contract, never the concrete ui/dialog package.
type StubDialogFactory func(Handler) shell.Dialog
