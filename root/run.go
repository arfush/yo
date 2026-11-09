package root

import (
	"os"
	"os/signal"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/op"

	"github.com/arfush/yo/core/platform"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	r := &root{window: new(app.Window)}
	r.initLog()
	defer r.stop()

	r.logger.Info("application started")
	r.initResource()
	r.initAppearance()
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
			if !r.windowShown {
				r.logger.Info("window shown", "size", e.Size)
				r.windowShown = true
			}
			gtx := app.NewContext(new(op.Ops), e)
			r.navigator.Apply()
			r.shell.Layout(gtx)
			e.Frame(gtx.Ops)
		case event.Event:
			platform.HandleViewEvent(e, r.appearance.Value().Palette().Border)
		}
	}
}
