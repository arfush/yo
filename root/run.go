package root

import (
	"gioui.org/app"
	"gioui.org/op"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	r := &root{window: new(app.Window)}
	r.initWindowOptions()

	for {
		switch e := r.window.Event().(type) {
		case app.DestroyEvent:
			return
		case app.FrameEvent:
			gtx := app.NewContext(new(op.Ops), e)
			e.Frame(gtx.Ops)
		}
	}
}
