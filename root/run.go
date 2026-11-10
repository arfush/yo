package root

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/op"

	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
	"github.com/arfush/yo/version"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	r := &root{window: new(app.Window)}
	r.initLog()
	defer r.stop()

	r.logger.Info("application started",
		"version", version.Ver, "os", runtime.GOOS, "arch", runtime.GOARCH)
	r.initResource()
	r.initAppearance()
	r.initSite()
	r.initWindowOptions()
	r.initShell()
	r.initNavigator()
	r.syncAppIcon()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-interrupt:
			r.logger.Info("interrupt received")
			r.window.Perform(system.ActionClose)
		case <-done:
		}
	}()

	for {
		switch e := r.window.Event().(type) {
		case app.DestroyEvent:
			if e.Err != nil {
				r.logger.Error("window destroyed", "error", e.Err)
			} else {
				r.logger.Info("window closed")
			}
			r.stop()
			return
		case app.ConfigEvent:
			r.applyConfig(e.Config)
		case app.FrameEvent:
			r.frame(e)
		case event.Event:
			r.platformWindow.HandleViewEvent(e, r.appearance.Value().Palette().Border)
		}
		r.syncAppIcon()
	}
}

// syncAppIcon applies the application icon of the active palette when it
// changes. The platform keeps the bytes until the native window exists.
func (r *root) syncAppIcon() {
	key := r.appearance.Value().AppIcon()
	if key == r.iconKey {
		return
	}
	r.iconKey = key
	r.platformWindow.SetIcon(r.resource.AppIconICO(key))
}

// applyConfig forwards the window configuration to the shell. Resizes arrive as
// config events too, so nothing here is logged per event.
func (r *root) applyConfig(cfg app.Config) {
	r.shell.SetConfig(cfg)
}

// frame draws one event loop frame. A panic anywhere in the frame is recovered
// and replaced with the panic screen; the event loop keeps running.
func (r *root) frame(e app.FrameEvent) {
	defer func() {
		if v := recover(); v != nil {
			r.showPanic(v)
		}
	}()

	if !r.windowShown {
		r.logger.Info("window shown", "size", e.Size)
		r.windowShown = true
	}
	gtx := app.NewContext(new(op.Ops), e)
	if !r.panicked {
		r.navigator.Apply()
	}
	r.shell.Prepare(gtx)
	r.shell.Layout(gtx)
	e.Frame(gtx.Ops)
}

// showPanic freezes the application on the base palette and installs the panic
// screen. The full stack goes to the journal; the screen shows the panic value.
func (r *root) showPanic(v any) {
	r.logger.Error("ui panic", "type", fmt.Sprintf("%T", v), "stack", string(debug.Stack()))
	if r.panicked {
		r.window.Invalidate()
		return
	}
	r.panicked = true
	defer func() {
		if again := recover(); again != nil {
			r.logger.Error("panic screen failed", "type", fmt.Sprintf("%T", again), "stack", string(debug.Stack()))
		}
	}()
	r.appearance.Update(func(a *appearance.Appearance) {
		a.SetPalette(appearance.BasePalette())
		a.SetAppIcon(resource.AppIconYobase)
	})
	r.shell.Freeze()
	r.shell.SetDialog(nil)
	app := r.appearance.Value()
	var build navigator.PanicFactory = r.newPanicScreen
	r.shell.SetScreen(build(panicTitle(app), fmt.Sprint(v)))
	r.shell.SetTitle(windowTitle)
	r.window.Invalidate()
}

// panicTitle resolves the localized panic heading. A broken locale falls back
// to English, because the panic screen must render even then.
func panicTitle(app appearance.Appearance) (title string) {
	title = defaultPanicTitle
	defer func() {
		if recover() != nil {
			title = defaultPanicTitle
		}
	}()
	return app.Locale().Text(resource.TextPanicTitle)
}
