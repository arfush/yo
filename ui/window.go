package ui

import "gioui.org/layout"

// windowScreen is the content of a Yonime window. The custom chrome is common
// to all windows; each screen owns only the area between the header and footer.
type windowScreen interface {
	layout(layout.Context)
	Close()
}

// footerScreen lets a screen draw its own controls into the common footer.
type footerScreen interface {
	layoutFooter(layout.Context)
}
