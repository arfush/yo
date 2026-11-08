package root

import (
	"gioui.org/app"

	"github.com/arfush/yo/ui/resource"
)

// root owns the application object graph and the window lifecycle.
type root struct {
	window    *app.Window
	resources *resource.Resource
}

func (r *root) initResource() {
	r.resources = resource.New()
}

func (r *root) initWindowOptions() {
	r.window.Option(app.Title("yo"))
}
