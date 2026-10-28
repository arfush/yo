package ui

import "gioui.org/layout"

// Screen is the content of a yo window. The custom chrome is common
// to all windows; each screen owns only the area between the header and footer.
type Screen interface {
	layout(layout.Context)
	Close()
}

// footerScreen lets a screen draw its own controls into the common footer.
type footerScreen interface {
	layoutFooter(layout.Context)
}

// fullHeightScreen lets a screen claim the footer strip for its content. The
// reader is the only bare screen: the chapter fills the window below the
// header, because an empty chrome band would just waste vertical space.
type fullHeightScreen interface {
	fullHeight()
}

// popupScreen lets the application close transient menus when the window loses
// focus, for example when the user clicks another application.
type popupScreen interface {
	dismissPopup()
}
