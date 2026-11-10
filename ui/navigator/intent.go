package navigator

// Intent is a navigation request. Concrete intents are declared in this package
// next to their factories; a screen only emits the value through Handler. The
// interface is empty on purpose: an intent is a typed event that carries the
// data the destination needs, never a screen or a stack operation.
type Intent any

// Handler is the navigation surface a screen receives. The navigator implements
// it; a screen never touches the stack directly.
type Handler interface {
	Navigate(Intent)
}

// Back returns to the previous screen. At the root screen it does nothing.
type Back struct{}
