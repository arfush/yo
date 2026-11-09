package screen

import (
	"gioui.org/layout"

	"github.com/arfush/yo/ui/navigator"
)

// Stub is the empty fallback screen installed before the navigator has a route.
type Stub struct {
	navigator.ScreenBase
}

// NewStub returns the fallback screen with the window title.
func NewStub(title string) Stub {
	return Stub{ScreenBase: navigator.ScreenBase{ScreenTitle: title}}
}

// Layout draws nothing; the shell owns the window background.
func (Stub) Layout(layout.Context) {}
