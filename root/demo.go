package root

import (
	"os"
	"time"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/screen"
)

// Temporary dialog demo wiring. Enabled only through the environment so a
// normal run is unaffected. Not part of the product.
const (
	demoEnv   = "YO_DEMO"
	demoDelay = 10 * time.Second
)

type demoState struct {
	enabled    bool
	panickedAt time.Time
	shown      bool
}

func (r *root) initDemo() {
	r.demo.enabled = os.Getenv(demoEnv) != ""
	if r.demo.enabled {
		r.logger.Info("dialog demo enabled")
	}
}

// demoFrame panics once, waits ten seconds on the panic screen and then
// installs the dialog demo.
func (r *root) demoFrame() {
	if !r.demo.enabled || r.demo.shown {
		return
	}
	if !r.panicked {
		r.demo.panickedAt = time.Now()
		panic("root: dialog demo panic")
	}
	if time.Since(r.demo.panickedAt) < demoDelay {
		return
	}
	r.showDemoScreen()
}

// showDemoScreen restores the active site palette, which the panic handler
// replaces with the neutral base, and installs the demo.
func (r *root) showDemoScreen() {
	r.demo.shown = true
	site := r.site.Value()
	r.appearance.Update(func(a *appearance.Appearance) {
		a.SetPalette(appearance.PaletteFor(site))
		a.SetAppIcon(appearance.AppIconFor(site))
	})
	r.shell.SetScreen(screen.NewDialogDemo(r.appearance.Readonly(), r.resource))
	r.shell.SetTitle(windowTitle)
	r.window.Invalidate()
	r.logger.Info("dialog demo shown")
}
