package browser

import "errors"

// Sentinels returned by the browser manager and the authorization flow. The UI
// maps them to short Russian sentences without importing Playwright types.
var (
	// ErrInvalidLink is returned when the user-supplied project link cannot be
	// opened.
	ErrInvalidLink = errors.New("invalid link")
	// ErrSiteUnreachable is returned when the site rejects or drops the
	// navigation. The usual fix is trying another mirror.
	ErrSiteUnreachable = errors.New("site did not open")
	// ErrWindowClosed is returned when the authorization browser is closed
	// before a token appears.
	ErrWindowClosed = errors.New("authorization window was closed")

	// ErrManagerNil means the manager was not opened before use.
	ErrManagerNil = errors.New("browser manager is nil")
	// ErrClosing means the manager is shutting down.
	ErrClosing = errors.New("application is closing")
	// ErrDownloadRunning means Chromium is already being installed.
	ErrDownloadRunning = errors.New("Chromium download is already running")
	// ErrChromiumMissing means the headless browser was never installed.
	ErrChromiumMissing = errors.New("Chromium is not installed")
	// ErrHeadlessNil means the shared headless browser is not running.
	ErrHeadlessNil = errors.New("headless browser is nil")
	// ErrHeadlessClosed means the headless browser exited while being used.
	ErrHeadlessClosed = errors.New("headless browser is closed")
	// ErrNoInstallPlan means Playwright finished without producing a usable
	// Chromium install plan.
	ErrNoInstallPlan = errors.New("Playwright finished without installing Chromium")
)
