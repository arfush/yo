package root

import (
	"github.com/arfush/yo/ui/dialog"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/shell"
)

// newStubDialog builds the fallback dialog. Real dialogs add their own factory
// next to it.
func (r *root) newStubDialog(handler navigator.Handler) shell.Dialog {
	return dialog.NewStub(handler)
}
