package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"

	"github.com/arfush/yo/library"
)

// loadStoredRect reads a "x,y,w,h" settings row into a physical-pixel rect.
func loadStoredRect(lib *library.Library, key string) (windowRect, bool) {
	raw, ok := lib.Setting(key)
	if !ok {
		return windowRect{}, false
	}
	return decodeRect(raw)
}

// encodeRect writes a physical-pixel rect as "x,y,w,h".
func encodeRect(rect windowRect) string {
	return strconv.Itoa(rect.X) + "," + strconv.Itoa(rect.Y) + "," + strconv.Itoa(rect.Width) + "," + strconv.Itoa(rect.Height)
}

// decodeRect parses the "x,y,w,h" form. A malformed or degenerate value is
// ignored so a broken settings row cannot park a window off-screen.
func decodeRect(raw string) (windowRect, bool) {
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return windowRect{}, false
	}
	values := make([]int, 4)
	for index, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return windowRect{}, false
		}
		values[index] = value
	}
	if values[2] <= 0 || values[3] <= 0 {
		return windowRect{}, false
	}
	return windowRect{X: values[0], Y: values[1], Width: values[2], Height: values[3]}, true
}

// windowHidden reports whether the window is parked in the notification area.
// Screens stop scheduling animation frames while it is true: a hidden window
// has nothing to draw and the wake-ups would only keep a core busy.
func (a *application) windowHidden() bool {
	return a.hidden.Load()
}

// startTray publishes the notification-area icon. It is best effort: when the
// icon cannot be created the close button keeps terminating the application,
// which is the behaviour the user expects from a window without a tray icon.
func (a *application) startTray() {
	if !traySupported() {
		return
	}
	icon, err := newTray(a.icon, a.restoreWindow, a.quitFromTray)
	if err != nil {
		fmt.Fprintln(os.Stderr, "yo: значок в трее:", err)
		return
	}
	a.tray = icon
}

// hideToTray parks the window in the notification area. Playback, downloads and
// in-flight requests keep running; only the window disappears.
//
// The Win32 call runs on its own goroutine on purpose: ShowWindow sends a
// message to the window thread, and that thread is waiting for this one to
// return to Window.Event() while a close request is being handled. Calling it
// inline would block both threads forever.
func (a *application) hideToTray() {
	a.hidePopups()
	a.hidden.Store(true)
	go hideAppWindow()
	// A pending silent install may proceed now that the user parked the window.
	a.wakeAutoUpdate()
}

// hidePopups closes any transient menu before the window stops being the
// active one. Clicking outside the window arrives as a focus change, not as a
// pointer event, so the application has to dismiss the popup here.
func (a *application) hidePopups() {
	if popup, ok := a.active.(popupScreen); ok {
		popup.dismissPopup()
	}
}

// restoreWindow brings the window back from the notification area. While mpv is
// playing it owns the screen, so a tray click (or a second launch) must not
// bring the application window on top of it.
func (a *application) restoreWindow() {
	w := a.window
	if w == nil {
		return
	}
	if a.playback != nil && a.playback.Active() {
		return
	}
	a.hidden.Store(false)
	showAppWindow()
	w.Invalidate()
}

// quitFromTray ends the process from the tray menu. The close request is no
// longer aborted, so the window is destroyed and close() runs the usual cleanup
// (mpv, Chromium and the database).
func (a *application) quitFromTray() {
	w := a.window
	if w == nil {
		return
	}
	a.quitting.Store(true)
	w.Perform(system.ActionClose)
}

// persistWindowRects writes the remembered positions of the application and
// the mpv window, at most once a second each and only on change, so the next
// open lands on the same spot without writing the database every frame.
func (a *application) persistWindowRects(now time.Time) {
	if a.library == nil {
		return
	}
	if !a.windowHidden() && now.Sub(a.appRectAt) >= time.Second {
		if rect, ok := appWindowRect(); ok && rectOnScreen(rect) && rect != a.appRectCur {
			a.appRectCur = rect
			a.appRectAt = now
			_ = a.library.SetSetting("window_rect", encodeRect(rect))
		}
	}
	session := a.playback.Current()
	if session == nil || now.Sub(a.mpvRectAt) < time.Second {
		return
	}
	if rect, ok := session.MpvWindowRect(); ok {
		converted := windowRect{X: rect.Min.X, Y: rect.Min.Y, Width: rect.Dx(), Height: rect.Dy()}
		if rectOnScreen(converted) && (!a.mpvRectOK || converted != a.mpvRect) {
			a.mpvRect = converted
			a.mpvRectOK = true
			a.mpvRectAt = now
			_ = a.library.SetSetting("mpv_rect", encodeRect(converted))
		}
	}
}

func (a *application) showWindow(w *app.Window, screen Screen) {
	if !a.recovering && a.active != nil && a.active != screen {
		a.previousScreen = a.active
	}
	if a.active != nil && a.active != screen {
		a.active.Close()
	}
	a.active = screen
	a.shortcuts = nil
	a.installScreen(screen)
	if screen == a.main {
		a.main.onShown()
	}
	w.Invalidate()
}

// installScreen wires an active screen into the shell without changing its
// state. It is used both for normal navigation and after the F1 summary closes,
// where main.onShown must not run because it would clear the URL field.
func (a *application) installScreen(screen Screen) {
	a.shell.footer = nil
	a.shell.expandContent = false
	if fs, ok := screen.(footerScreen); ok {
		a.shell.footer = func(gtx layout.Context) layout.Dimensions {
			fs.layoutFooter(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	if _, ok := screen.(fullHeightScreen); ok {
		a.shell.expandContent = true
	}
	if screen == a.main {
		a.shell.onPress = a.main.noteWindowPress
	} else {
		a.shell.onPress = nil
	}
	if overlay, ok := screen.(overlayScreen); ok {
		a.shell.overlay = overlay.layoutOverlay
	} else {
		a.shell.overlay = nil
	}
	a.shell.content = func(gtx layout.Context) layout.Dimensions {
		if a.shortcuts != nil {
			a.shortcuts.layout(gtx)
		} else {
			screen.layout(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}
