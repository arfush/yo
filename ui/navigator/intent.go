package navigator

// Route identifies a destination registered by the composition root. The
// navigator owns the type; concrete routes belong to the feature that needs
// them, so this package never imports a screen.
type Route string

// Intent is a navigation request emitted by a screen. It names the destination
// route; parameters are added when a destination needs them.
type Intent struct {
	Route Route
}

// Handler is the navigation surface a screen receives. The navigator
// implements it; a screen never touches the stack directly.
type Handler interface {
	Navigate(Intent)
}
