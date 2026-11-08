package root

import (
	"gioui.org/app"
	"gioui.org/op"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	r := &root{window: new(app.Window)}
	r.initLog()
	defer r.closeLog()

	r.logger.Info("application started")
	r.initResource()
	r.initWindowOptions()

	for {
		switch e := r.window.Event().(type) {
		case app.DestroyEvent:
			if e.Err != nil {
				r.logger.Error("window destroyed", "error", e.Err)
			} else {
				r.logger.Info("window closed")
			}
			r.logger.Info("application stopped")
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
