package root

import (
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/screen"
)

// newStartupScreen builds the temporary main screen, a placeholder until the
// real main screen exists. It reuses the panic screen layout with no detail and
// keeps the native window title constant.
func (r *root) newStartupScreen(navigator.Handler) navigator.Screen {
	app := r.appearance.Value()
	startup := screen.NewPanic(app, panicTitle(app), "")
	startup.ScreenTitle = windowTitle
	return startup
}
