package ui

import "gioui.org/layout"

// Screen is the content of a yoanime window. The custom chrome is common
// to all windows; each screen owns only the area between the header and footer.
type Screen interface {
	layout(layout.Context)
	Close()
}

// footerScreen lets a screen draw its own controls into the common footer.
type footerScreen interface {
	layoutFooter(layout.Context)
}

// popupScreen lets the application close transient menus when the window loses
// focus, for example when the user clicks another application.
type popupScreen interface {
	dismissPopup()
}
