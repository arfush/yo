package root

import (
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/screen"
)

// newStubScreen builds the fallback screen installed at startup.
func (r *root) newStubScreen(navigator.Handler) navigator.Screen {
	return screen.NewStub(windowTitle)
}
