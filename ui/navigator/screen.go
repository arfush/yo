package navigator

import (
	"gioui.org/layout"

	"github.com/arfush/yo/ui/shell"
)

// Screen is what the navigator drives. The shell draws it; the navigator owns
// its title and lifecycle.
type Screen interface {
	shell.Screen
	Title() string
	OnShown()
	OnHidden()
	OnClose()
}

// ScreenBase supplies the title and the no-op parts of Screen. A screen embeds
// it and implements Layout itself.
type ScreenBase struct {
	ScreenTitle string
}

// Title returns the header title of the screen.
func (b ScreenBase) Title() string { return b.ScreenTitle }

// LayoutFooter draws nothing.
func (ScreenBase) LayoutFooter(layout.Context) {}

// OnShown is a no-op.
func (ScreenBase) OnShown() {}

// OnHidden is a no-op.
func (ScreenBase) OnHidden() {}

// OnClose is a no-op.
func (ScreenBase) OnClose() {}

// ScreenFactory builds a screen with the handler it uses to request navigation.
type ScreenFactory func(Handler) Screen
