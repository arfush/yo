package navigator

// Intent is a navigation request. Concrete intents are declared in this package
// next to their factories; a screen only emits the value through Handler. The
// interface is empty on purpose: an intent is a typed event that carries the
// data the destination needs, never a screen or a stack operation.
type Intent any

// Handler is the navigation surface a screen or dialog receives. The navigator
// implements it; a screen never touches the stack directly.
type Handler interface {
	Navigate(Intent)
}

// Back closes the top dialog or returns to the previous screen.
type Back struct{}
