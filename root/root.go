package root

import "gioui.org/app"

// root owns the application object graph and the window lifecycle.
type root struct {
	window *app.Window
}

func (r *root) initWindowOptions() {
	r.window.Option(app.Title("yo"))
}
