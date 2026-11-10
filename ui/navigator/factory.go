package navigator

import "github.com/arfush/yo/ui/shell"

// PanicFactory builds the panic screen from its title and detail. It takes no
// Handler: the panic screen only reports a failure and never navigates. The
// composition root supplies the concrete builder in root/screen.go.
type PanicFactory func(title, detail string) Screen

// DependencyInstallationFactory builds the dependency installation dialog. The
// composition root supplies the concrete builder in root/dialog.go.
type DependencyInstallationFactory func() shell.Dialog
