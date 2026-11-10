package root

import (
	"github.com/arfush/yo/ui/dialog"
	"github.com/arfush/yo/ui/resource"
	"github.com/arfush/yo/ui/shell"
)

// newDependencyInstallation builds the dependency installation dialog. The
// download action is not wired to core/dependency yet: the dialog currently
// presents the request only.
func (r *root) newDependencyInstallation() shell.Dialog {
	app := r.appearance.Value()
	return dialog.NewDependencyInstallation(r.appearance.Readonly(), dialog.DependencyOptions{
		Resource:    r.resource,
		Handler:     r.navigator,
		Title:       app.Locale().Text(resource.TextDependencyTitle),
		Description: app.Locale().Text(resource.TextDependencyDescription),
		ShowLater:   true,
	})
}
