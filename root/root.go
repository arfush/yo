package root

import (
	"gioui.org/app"

	"github.com/arfush/yo/core/log"
	"github.com/arfush/yo/ui/resource"
)

const windowTitle = "yo"

// root owns the application object graph and the window lifecycle.
type root struct {
	window      *app.Window
	resources   *resource.Resource
	journal     *log.Journal
	logger      *log.Logger
	windowShown bool
}

func (r *root) initResource() {
	r.resources = resource.New()
}

func (r *root) initWindowOptions() {
	r.window.Option(app.Title(windowTitle))
	r.logger.Info("window configured", "title", windowTitle)
}
