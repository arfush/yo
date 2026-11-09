package root

import (
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/screen"
)

// newStartupScreen builds the temporary main screen, a placeholder until the
// real main screen exists. It reuses the panic screen layout with no detail and
// keeps the native window title constant.
func (r *root) newStartupScreen() navigator.Screen {
	app := r.appearance.Value()
	startup := screen.NewPanic(r.appearance.Readonly(), panicTitle(app), "")
	startup.ScreenTitle = windowTitle
	return startup
}

// newPanicScreen builds the panic screen. It implements navigator.PanicFactory:
// the screen takes no Handler because it never navigates.
func (r *root) newPanicScreen(title, detail string) navigator.Screen {
	return screen.NewPanic(r.appearance.Readonly(), title, detail)
}
