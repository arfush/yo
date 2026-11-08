package root

import (
	"os"
	"os/signal"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
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

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	go func() {
		<-interrupt
		signal.Stop(interrupt)
		r.logger.Info("interrupt received")
		r.stop()
		r.window.Perform(system.ActionClose)
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
		case app.FrameEvent:
			if !r.windowShown {
				r.logger.Info("window shown", "size", e.Size)
				r.windowShown = true
			}
			gtx := app.NewContext(new(op.Ops), e)
			e.Frame(gtx.Ops)
		}
	}
}
