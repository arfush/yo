package root

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/op"

	"github.com/arfush/yo/core/platform"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/screen"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	r := &root{window: new(app.Window)}
	r.initLog()
	defer r.stop()

	r.logger.Info("application started")
	r.initResource()
	r.initAppearance()
	r.initSite()
	r.initWindowOptions()
	r.initShell()
	r.initNavigator()

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
			r.shell.SetConfig(e.Config)
		case app.FrameEvent:
			r.frame(e)
		case event.Event:
			platform.HandleViewEvent(e, r.appearance.Value().Palette().Border)
		}
	}
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
	r.logger.Error("ui panic", "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
	if r.panicked {
		r.window.Invalidate()
		return
	}
	r.panicked = true
	defer func() {
		if again := recover(); again != nil {
			r.logger.Error("panic screen failed", "panic", fmt.Sprint(again), "stack", string(debug.Stack()))
		}
	}()
	r.appearance.Update(func(a *appearance.Appearance) { a.SetPalette(appearance.BasePalette()) })
	r.shell.Freeze()
	r.shell.SetDialog(nil)
	app := r.appearance.Value()
	r.shell.SetScreen(screen.NewPanic(app, panicTitle(app), fmt.Sprint(v)))
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
	return app.Locale().Text(panicTitleKey)
}
