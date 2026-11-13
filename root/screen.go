package root

import (
	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/screen"
)

// stubFrameInterval is how often the placeholder asks for a repaint, in
// milliseconds. It keeps the mascot animation smooth.
const stubFrameInterval = 16

// newStartupScreen builds the temporary main screen, a placeholder until the
// real main screen exists. It keeps the native window title constant.
func (r *root) newStartupScreen() navigator.Screen {
	startup := screen.NewStub()
	startup.ScreenTitle = windowTitle
	return startup
}

// startStubAnimation drives the placeholder mascot until screens own their own
// invalidation. Temporary, like the stub itself.
func (r *root) startStubAnimation() {
	timer := qt6.NewQTimer2(r.shell.Widget().QObject)
	timer.SetInterval(stubFrameInterval)
	timer.OnTimeout(func() { r.shell.Widget().Update() })
	timer.Start2()
}
