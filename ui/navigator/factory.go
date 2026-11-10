package navigator

// PanicFactory builds the panic screen from its title and detail. It takes no
// Handler: the panic screen only reports a failure and never navigates. The
// composition root supplies the concrete builder in root/screen.go.
type PanicFactory func(title, detail string) Screen
