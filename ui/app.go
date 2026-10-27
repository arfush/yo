package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yoanime/internal/animelib"
	"github.com/arfush/yoanime/internal/appdata"
	"github.com/arfush/yoanime/internal/browser"
	"github.com/arfush/yoanime/internal/kinopoisk"
	"github.com/arfush/yoanime/internal/library"
	"github.com/arfush/yoanime/internal/metadata"
	"github.com/arfush/yoanime/internal/mpv"
	"github.com/arfush/yoanime/internal/playback"
	"github.com/arfush/yoanime/internal/token"
	"github.com/arfush/yoanime/internal/updater"
	"github.com/arfush/yoanime/internal/updates"
	"github.com/arfush/yoanime/internal/version"
	"github.com/arfush/yoanime/log"
	"github.com/arfush/yoanime/party"
)

const (
	// episodeCheckInterval is deliberately slow: the checker walks the whole
	// saved library and every title costs three AnimeLIB requests.
	episodeCheckInterval = 10 * time.Minute
	// episodeCheckStartupDelay lets the window, token check and update check
	// settle before the first background pass starts spending the shared
	// AnimeLIB rate-limit window.
	episodeCheckStartupDelay = 10 * time.Second
	// playbackStartTimeout bounds the network work between a Play click and an
	// mpv window. Without it a stuck rate-limit wait or resolver leaves the
	// row's spinner turning forever with no way to retry.
	playbackStartTimeout = 3 * time.Minute
	// autoUpdatePollInterval is how often a downloaded update rechecks whether
	// installing it would interrupt playback or a watch party.
	autoUpdatePollInterval = 15 * time.Second
	// autoUpdateDownloadTimeout bounds the silent download. It is deliberately
	// generous because the executable is tens of megabytes.
	autoUpdateDownloadTimeout = 30 * time.Minute
	// updateCheckInterval is how often the release feed is polled while the
	// network works. A four-hour interval meant a release published after launch
	// was noticed only on the next restart.
	updateCheckInterval = 30 * time.Minute
	// updateRetryInterval shortens the next attempt after a network failure. The
	// footer shows the cached release even when the check failed, so a long
	// interval would leave a visible update without a silent install.
	updateRetryInterval = 5 * time.Minute
	// qualitiesProbeTimeout bounds the background headless resolve that learns
	// which qualities a Kinopoisk voice offers. A cold Chromium is slow, but a
	// failed probe only leaves the standard buttons enabled.
	qualitiesProbeTimeout = 90 * time.Second
)

// Run starts the yoanime desktop application.
func Run() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("yoanime"),
			app.Size(unit.Dp(initialWidth), unit.Dp(initialHeight)),
			app.MinSize(unit.Dp(minimumWidth), unit.Dp(minimumHeight)),
			app.Decorated(false),
		)

		a, err := newApp()
		if err != nil {
			log.Error("application startup failed", "error", err)
			fmt.Fprintln(os.Stderr, "yoanime:", err)
			os.Exit(1)
		}
		log.Info("window attached")
		a.attachWindow(w)
		if err := a.run(w); err != nil {
			log.Error("application stopped with an error", "error", err)
			a.close()
			fmt.Fprintln(os.Stderr, "yoanime:", err)
			os.Exit(1)
		}
		log.Info("application closed")
		a.close()
		os.Exit(0)
	}()

	app.Main()
}

type application struct {
	shell          *shell
	browserManager *browser.Manager
	mpvManager     *mpv.Manager
	library        *library.Library
	playback       *playback.Service
	party          *partyManager
	main           *mainWindow
	log            *logWindow
	active         windowScreen
	shortcuts      *shortcutsWindow
	window         *app.Window
	icon           image.Image
	// tray is the notification-area icon. It stays nil when the platform has no
	// notification area or when creating the icon failed; the close button then
	// keeps ending the process.
	tray *tray
	// hidden is set while the window lives in the notification area, so screens
	// can stop their animation clocks. quitting is set by the tray menu, the
	// only close request that is allowed to end the process.
	hidden   atomic.Bool
	quitting atomic.Bool

	watchResults chan watchResult
	// qualityResults carries the background Kinopoisk quality probe. It runs
	// while the title page is open and updates the disabled buttons in place.
	qualityResults chan qualityResult

	// The new-episode checker is one background goroutine. Its context cancels
	// an in-flight pass when the user disables notifications or closes the app;
	// the stop/done pair prevents the database from being closed underneath it.
	episodeCtx        context.Context
	episodeCancel     context.CancelFunc
	episodeWake       chan struct{}
	episodeStop       chan struct{}
	episodeDone       chan struct{}
	episodeMu         sync.Mutex
	episodeStarted    bool
	episodePassMu     sync.Mutex
	episodePassCancel context.CancelFunc

	tokenPath         string
	browserProfileDir string

	// Update state is loaded from %APPDATA%\yoanime\updates.json. The atomics
	// let the UI read it every frame while the background checker refreshes it.
	updateStore      *updates.Store
	updateClient     *http.Client
	downloadClient   *http.Client
	updatesAvailable atomic.Pointer[updates.Release]
	updateChecking   atomic.Bool
	updatePending    atomic.Bool
	updateWake       chan struct{}
	// currentSHA is the hash of the running executable. It lets a re-pushed
	// version with the same number still install its fixed build.
	currentSHA string

	// One silent auto-update may run at a time. It replaces the executable only
	// while the window sits in the tray: a visible window means the user is
	// working, and mpv or a watch party must never be interrupted.
	autoUpdateMu      sync.Mutex
	autoUpdateRunning bool
	autoUpdateCancel  context.CancelFunc
	// autoUpdateWake tells a waiting install that its conditions may have
	// changed: the window went to the tray or mpv closed.
	autoUpdateWake chan struct{}
	// autoUpdateStartup is true from launch until the launch update decision is
	// made. During that window an available update is presented as the download
	// card instead of the main screen. The window survives the first-run,
	// authorization and mpv screens: a fresh install reaches main minutes after
	// the check and must still pick up the release it found.
	autoUpdateStartup atomic.Bool
	// mainShown and firstCheckDone bound that window. Once the first check has
	// finished *and* main has been on screen, a later check must not open the
	// card over a user who is already working.
	mainShown      atomic.Bool
	firstCheckDone atomic.Bool
	// autoUpdateCard carries a release the UI goroutine should present as the
	// download card. The check runs off the UI goroutine and cannot open a
	// screen itself.
	autoUpdateCard chan updates.Release
	// playbackStarting covers the network work between a Play click and the
	// moment Service.Start registers the session; playback.Active only covers
	// the resolve inside the service. A silent update must not quit the app in
	// that gap.
	playbackStarting atomic.Bool

	// mpvRect is where the user last parked the mpv window. Playback reuses it
	// instead of inheriting the main window position, and the rect is refreshed
	// from mpv every second while it plays.
	mpvRect    windowRect
	mpvRectOK  bool
	mpvRectAt  time.Time
	appRect    windowRect
	appRectAt  time.Time
	appRectCur windowRect
}

// qualityResult is one background quality probe. slug identifies the title page
// that asked for it, so a late result cannot repaint another title.
type qualityResult struct {
	slug   string
	voice  string
	values []int
	err    error
}

type watchResult struct {
	finished       bool
	err            error
	historyItem    *historyTitle
	historyFailure bool
	// titleID is the history row the session wrote progress to. A link-opened
	// title gets an id only when playback tracks it.
	titleID int64
	// party marks a playback session that the watch-party guest loop started.
	// Its failures belong to the room screen, not to the history rows.
	party bool
}

func newApp() (*application, error) {
	typo, err := newTypography()
	if err != nil {
		return nil, err
	}
	icon, err := loadAppIcon()
	if err != nil {
		return nil, fmt.Errorf("load app icon: %w", err)
	}
	databasePath, err := appdata.DatabasePath()
	if err != nil {
		return nil, err
	}
	tokenPath, err := appdata.TokenPath()
	if err != nil {
		return nil, err
	}
	profileDir, err := appdata.BrowserProfileDir()
	if err != nil {
		return nil, err
	}
	updatesPath, err := appdata.UpdatesPath()
	if err != nil {
		return nil, err
	}
	updateStore, err := updates.Open(updatesPath)
	if err != nil {
		return nil, err
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return nil, err
	}
	updater.Cleanup(updateDir)

	// The executable path is hashed once so a re-pushed build with an unchanged
	// version can still be offered.
	currentSHA := ""
	if executable, exeErr := os.Executable(); exeErr == nil {
		if digest, hashErr := updates.FileSHA256(executable); hashErr == nil {
			currentSHA = digest
		} else {
			fmt.Fprintln(os.Stderr, "yoanime: хеш приложения:", hashErr)
		}
	}

	lib, err := library.Open(databasePath, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	history, err := lib.History()
	if err != nil {
		lib.Close()
		return nil, fmt.Errorf("load history: %w", err)
	}
	manager := browser.New()
	episodeCtx, episodeCancel := context.WithCancel(context.Background())
	app := &application{
		shell:             newShell(typo, icon),
		browserManager:    manager,
		mpvManager:        mpv.New(),
		library:           lib,
		playback:          playback.New(lib, manager),
		main:              newMainWindow(typo, lib, history),
		icon:              icon,
		watchResults:      make(chan watchResult, 8),
		qualityResults:    make(chan qualityResult, 8),
		episodeCtx:        episodeCtx,
		episodeCancel:     episodeCancel,
		episodeWake:       make(chan struct{}, 1),
		episodeStop:       make(chan struct{}),
		episodeDone:       make(chan struct{}),
		tokenPath:         tokenPath,
		browserProfileDir: profileDir,
		updateStore:       updateStore,
		updateClient:      &http.Client{Timeout: 25 * time.Second},
		downloadClient:    &http.Client{Timeout: 30 * time.Minute},
		updateWake:        make(chan struct{}, 1),
		autoUpdateWake:    make(chan struct{}, 1),
		autoUpdateCard:    make(chan updates.Release, 1),
		currentSHA:        currentSHA,
	}
	state := updateStore.State()
	app.updatesAvailable.Store(state.Available)
	app.autoUpdateStartup.Store(true)
	app.party = newPartyManager(app)

	// Restore window placements from the previous session: mpv reopens where
	// the user parked it, and the main window itself comes back to its spot.
	app.mpvRect, app.mpvRectOK = loadStoredRect(lib, "mpv_rect")
	if rect, ok := loadStoredRect(lib, "window_rect"); ok {
		app.appRectCur = rect
		// The move happens on the first view event, slightly after this point.
		// Delay persistence so the not-yet-moved default rectangle is not
		// written over the remembered one in the meantime.
		app.appRectAt = time.Now()
		rememberAppWindowRect(rect)
	}
	return app, nil
}

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

// attachWindow checks the AnimeLIB token first, then makes sure Chromium is
// available when a new token has to be generated, then installs mpv when it is
// missing, and only after that opens the main window.
func (a *application) attachWindow(w *app.Window) {
	a.window = w
	// A 429 must repaint the header immediately, even if the screen does not
	// animate on its own.
	animelib.SetRateLimitListener(w.Invalidate)
	a.main.invalidate = w.Invalidate
	a.main.hidden = a.windowHidden
	a.shell.hidden = a.windowHidden
	a.main.onOpenTitle = func(item historyTitle) {
		a.openTitle(item)
	}
	a.main.onPlayHistory = func(item historyTitle) historyPlayResult {
		return a.playFromHistory(item)
	}
	a.main.onUserStatus = func(item historyTitle, status library.UserStatus) error {
		return a.setUserStatus(item, status)
	}
	a.main.onError = a.reportError
	a.main.onClearError = a.clearError
	a.shell.onUpdate = a.openUpdate
	a.shell.onSettings = a.openSettings
	a.shell.onParty = a.openParty
	a.shell.onLog = a.openLog
	a.startTray()
	a.prepare(w)
	a.startUpdateLoop(w)
	a.startEpisodeLoop(w)
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
		fmt.Fprintln(os.Stderr, "yoanime: значок в трее:", err)
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

func (a *application) prepare(w *app.Window) {
	if a.tokenReady() {
		a.requireMPV(w)
		return
	}
	if a.browserManager.Installed() {
		a.requireToken(w)
		return
	}

	preparation := newFirstRun(
		a.shell.typo,
		w,
		"Требуется Chromium",
		"Для генерации API-токена требуется скачать Chromium.",
		func(ctx context.Context, report func(downloadProgress)) error {
			return a.browserManager.Download(ctx, func(progress browser.Progress) {
				report(downloadProgress{Percent: progress.Percent, Preparing: progress.Preparing})
			})
		},
	)
	preparation.onDone = func() {
		a.requireToken(w)
	}
	preparation.onError = a.reportError
	a.showWindow(w, preparation)
}

// tokenReady reports whether the stored token is present and has not expired.
func (a *application) tokenReady() bool {
	value, err := token.Load(a.tokenPath)
	return err == nil && !value.Expired(time.Now())
}

// requireToken is used both on launch and right after Chromium has been
// installed: it asks the user to authorize when no usable token is available.
func (a *application) requireToken(w *app.Window) {
	if a.tokenReady() {
		a.requireMPV(w)
		return
	}

	prompt := newTokenPrompt(a.shell.typo, a.browserManager, w, a.tokenPath, a.browserProfileDir)
	prompt.onDone = func() {
		a.requireMPV(w)
	}
	a.showWindow(w, prompt)
}

// requireMPV installs the bundled mpv build after authorization. A system-wide
// mpv from PATH is used as is and skips the download screen.
func (a *application) requireMPV(w *app.Window) {
	if a.mpvManager.Installed() {
		a.showMain(w)
		return
	}

	preparation := newFirstRun(
		a.shell.typo,
		w,
		"Требуется MPV",
		"MPV нужен для просмотра контента.",
		func(ctx context.Context, report func(downloadProgress)) error {
			return a.mpvManager.Download(ctx, func(progress mpv.Progress) {
				report(downloadProgress{Percent: progress.Percent, Preparing: progress.Preparing})
			})
		},
	)
	preparation.onDone = func() {
		a.showMain(w)
	}
	preparation.onError = a.reportError
	a.showWindow(w, preparation)
}

func (a *application) showMain(w *app.Window) {
	// At launch an available update is presented as a download card instead of
	// the main screen, so the user sees what is being installed. A first install
	// may reach main only after authorization and the Chromium/mpv setup, so the
	// cached release is checked on the first main screen, not at process start.
	if !a.mainShown.Swap(true) {
		if a.offerStartupUpdate(a.updatesAvailable.Load()) {
			return
		}
		// The launch check already finished without a release: close the window
		// so a periodic check cannot open the card over a working user.
		if a.firstCheckDone.Load() {
			a.autoUpdateStartup.Store(false)
		}
	}
	a.showWindow(w, a.main)
	w.Invalidate()
}

// offerStartupUpdate shows the download card instead of the first main screen
// of a launch when the cached release can be installed automatically. It
// reports whether the card replaced the main screen.
func (a *application) offerStartupUpdate(release *updates.Release) bool {
	if release == nil || !a.autoUpdatesEnabled() || !a.updateAvailable(release) {
		return false
	}
	if !a.autoUpdateStartup.CompareAndSwap(true, false) {
		return false
	}
	a.showUpdateScreen(*release, true)
	return true
}

func (a *application) openUpdate() {
	release := a.updatesAvailable.Load()
	if !a.updateAvailable(release) || a.window == nil {
		return
	}
	// The user is installing manually; stop a waiting silent install so it
	// cannot replace the executable behind the update screen.
	a.cancelAutoUpdate()
	a.autoUpdateStartup.Store(false)
	a.showUpdateScreen(*release, false)
}

// showUpdateScreen presents the download card and quits the application once
// the verified executable is ready, so the helper can replace it. auto marks the
// card shown without a click: a failed automatic download returns to the main
// screen instead of trapping the user on the card.
func (a *application) showUpdateScreen(release updates.Release, auto bool) {
	if a.window == nil {
		return
	}
	screen := newFirstRun(
		a.shell.typo,
		a.window,
		"Обновление yoanime",
		fmt.Sprintf("Скачиваем %s. Приложение перезапустится автоматически.", release.Version),
		func(ctx context.Context, report func(downloadProgress)) error {
			return a.downloadAndApply(ctx, release, report)
		},
	)
	screen.buttonText = "Скачать"
	screen.retryText = "Повторить"
	screen.errorText = "Не удалось обновить. Попробуй ещё раз."
	screen.autoStart = true
	screen.onDone = a.quitForUpdate
	if auto {
		screen.onError = func(err error) {
			// Keep the app usable and let the silent path retry once the window
			// goes to the tray.
			a.reportError(err)
			a.showWindow(a.window, a.main)
			a.triggerUpdateCheck()
		}
	} else {
		screen.onError = a.reportError
	}
	a.showWindow(a.window, screen)
}

// openSettings shows the preferences. Changing a toggle or the relay takes
// effect immediately; enabling notifications also starts a checker pass
// without waiting for the regular timer.
func (a *application) openSettings() {
	if a.window == nil || a.library == nil {
		return
	}
	if a.party == nil {
		a.party = newPartyManager(a)
	}
	partyCfg := a.party.Config()
	settings := newSettingsWindow(a.shell.typo, settingsValues{
		Notifications: a.library.EpisodeNotificationsEnabled(),
		AutoUpdate:    a.library.AutoUpdateEnabled(),
		Beta:          a.library.BetaUpdatesEnabled(),
		SharedControl: a.library.SharedControlEnabled(),
		AutoSkip:      a.library.AutoSkipSegments(),
		Name:          a.party.Name(),
		Provider:      a.party.Provider(),
		CloudflareURL: partyCfg.CloudflareURL,
		SupabaseURL:   partyCfg.SupabaseURL,
		SupabaseKey:   partyCfg.SupabaseKey,
	})
	settings.hidden = a.windowHidden
	settings.onError = a.reportError
	settings.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	settings.onToggle = func(enabled bool) error {
		if err := a.library.SetEpisodeNotificationsEnabled(enabled); err != nil {
			return err
		}
		if enabled {
			a.triggerEpisodeCheck()
		} else {
			a.cancelEpisodePass()
		}
		return nil
	}
	settings.onAutoUpdate = func(enabled bool) error {
		if err := a.library.SetAutoUpdateEnabled(enabled); err != nil {
			return err
		}
		if enabled {
			// A fresh check decides which release may be installed. This also
			// avoids installing a cached build from the previous channel.
			a.triggerUpdateCheck()
		} else {
			a.cancelAutoUpdate()
		}
		return nil
	}
	settings.onBeta = func(enabled bool) error {
		if err := a.library.SetBetaUpdatesEnabled(enabled); err != nil {
			return err
		}
		// The channel changed, so a pending download may no longer belong to
		// the selected channel. Refetch before starting another silent install.
		a.cancelAutoUpdate()
		a.triggerUpdateCheck()
		return nil
	}
	settings.onControl = func(enabled bool) error {
		if err := a.library.SetSharedControlEnabled(enabled); err != nil {
			return err
		}
		a.party.SetSharedControl(enabled)
		return nil
	}
	settings.onAutoSkip = func(segment metadata.Segment, enabled bool) error {
		return a.library.SetAutoSkipSegment(segment, enabled)
	}
	settings.onProvider = func(provider party.Provider) error {
		// SetProvider stores the choice, so the next room and the party card
		// agree with the settings.
		a.party.SetProvider(provider)
		return nil
	}
	settings.onRelay = func(cfg party.Config) error {
		return a.party.SetRelayConfig(cfg)
	}
	settings.onName = func(name string) error {
		return a.party.SetName(name)
	}
	a.showWindow(a.window, settings)
}

// openParty shows the room card. When a room is still open the screen
// reopens on the created (host) or waiting (guest) form with the same key.
func (a *application) openParty() {
	if a.window == nil {
		return
	}
	if a.party == nil {
		a.party = newPartyManager(a)
	}
	screen := newPartyWindow(a.shell.typo, a.window, a.party)
	screen.hidden = a.windowHidden
	screen.onError = a.reportError
	screen.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	a.showWindow(a.window, screen)
}

// startPartyPlayback opens the title the host is watching and starts mpv close
// to the extrapolated host position. The guest drift loop then corrects the
// remaining difference.
func (a *application) startPartyPlayback(state party.State) error {
	if a.library == nil || a.playback == nil {
		return fmt.Errorf("воспроизведение недоступно")
	}
	// The host may jump to another episode or voice while the guest is still
	// playing. mpv can only run one session, so the old one has to go first.
	if a.playback.Active() {
		if current := a.playback.Current(); current != nil {
			current.Stop()
		}
		deadline := time.Now().Add(3 * time.Second)
		for a.playback.Active() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if a.playback.Active() {
			return fmt.Errorf("mpv не успел завершить прошлую серию")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	title, err := a.library.Load(ctx, state.TitleSlug)
	if err != nil {
		return err
	}
	quality := a.library.PreferredQuality()
	startAt := 0.0
	if a.party != nil {
		if position, ok := a.party.ExpectedPosition(); ok {
			startAt = position
		}
	}
	return a.startPlayback(title, state.Voice, state.Episode, quality, nil, true, startAt)
}

func (a *application) downloadAndApply(ctx context.Context, release updates.Release, report func(downloadProgress)) error {
	if report != nil {
		report(downloadProgress{Preparing: true})
	}
	source, err := a.downloadRelease(ctx, release, func(downloaded, total int64) {
		if report == nil {
			return
		}
		percent := 0.0
		if total > 0 {
			percent = float64(downloaded) * 100 / float64(total)
		}
		report(downloadProgress{Percent: percent})
	})
	if err != nil {
		return err
	}
	if report != nil {
		report(downloadProgress{Percent: 100})
	}
	return a.applyDownload(source)
}

// downloadRelease fetches and verifies the executable without installing it, so
// the silent updater can wait for an idle moment between download and replace.
func (a *application) downloadRelease(ctx context.Context, release updates.Release, report func(downloaded, total int64)) (string, error) {
	target, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось определить файл приложения: %w", err)
	}
	if err := updater.CheckTarget(target); err != nil {
		return "", err
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return "", err
	}
	name := filepath.Base(release.AssetName)
	if name == "." || name == "" {
		name = updates.AssetName(release.Version, runtime.GOARCH)
	}
	destination := filepath.Join(updateDir, name)
	return updates.Download(ctx, a.downloadClient, release, destination, report)
}

// applyDownload starts the hidden helper that replaces the executable after
// this process exits.
func (a *application) applyDownload(source string) error {
	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить файл приложения: %w", err)
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return err
	}
	logPath, err := appdata.UpdateLogPath()
	if err != nil {
		return err
	}
	plan, err := updater.Prepare(target, source, updateDir, logPath)
	if err != nil {
		return err
	}
	return plan.Launch()
}

func (a *application) quitForUpdate() {
	a.quitting.Store(true)
	if a.window != nil {
		a.window.Perform(system.ActionClose)
	}
}

func (a *application) startUpdateLoop(w *app.Window) {
	if updates.Repository == "" {
		return
	}
	go func() {
		timer := time.NewTimer(a.checkForUpdates(w))
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
			case <-a.updateWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			timer.Reset(a.checkForUpdates(w))
		}
	}()
}

func (a *application) triggerUpdateCheck() {
	select {
	case a.updateWake <- struct{}{}:
	default:
	}
}

// checkForUpdates refreshes the cached release and returns how long the loop
// should sleep before the next attempt. A failed request retries soon: the
// footer keeps showing the cached release, and the user must not be left with a
// visible update that never installs.
func (a *application) checkForUpdates(w *app.Window) time.Duration {
	if !a.updateChecking.CompareAndSwap(false, true) {
		// A settings change or the periodic timer may ask for another check
		// while the current one is still running.
		a.updatePending.Store(true)
		return updateCheckInterval
	}
	defer func() {
		a.updateChecking.Store(false)
		a.firstCheckDone.Store(true)
		// Once main is on screen and the launch check has finished, the startup
		// window is over and a later check must not open the card over a working
		// user. Before main (authorization, Chromium and mpv on a fresh install)
		// the window stays open so showMain can still offer the release.
		if a.mainShown.Load() {
			a.autoUpdateStartup.Store(false)
		}
		if a.updatePending.Swap(false) {
			a.triggerUpdateCheck()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	release, err := updates.FetchReleaseChannel(ctx, a.updateClient, a.updateChannel())
	now := time.Now()
	if err != nil {
		if errors.Is(err, updates.ErrNoRelease) {
			release = nil
			err = nil
		} else {
			// A network failure must not hide the release cached from the last
			// successful check.
			fmt.Fprintln(os.Stderr, "yoanime: проверка обновлений:", err)
			_ = a.updateStore.SaveLastCheck(now)
			return updateRetryInterval
		}
	}
	if saveErr := a.updateStore.RecordCheck(release, now); saveErr != nil {
		fmt.Fprintln(os.Stderr, "yoanime: сохранение состояния обновлений:", saveErr)
	}
	a.updatesAvailable.Store(release)
	if w != nil {
		w.Invalidate()
	}
	a.maybeAutoUpdate(release)
	return updateCheckInterval
}

// maybeAutoUpdate runs on the update loop goroutine. During the launch window it
// hands the release to the UI goroutine, which presents the download card;
// afterwards the install waits for the tray.
func (a *application) maybeAutoUpdate(release *updates.Release) {
	if release == nil || !a.autoUpdatesEnabled() || !a.updateAvailable(release) {
		return
	}
	// Before the first main screen (fresh install: authorization, Chromium,
	// mpv) the release has to wait for showMain, which presents it once the app
	// is ready instead of interrupting the setup.
	if !a.mainShown.Load() {
		return
	}
	if a.autoUpdateStartup.CompareAndSwap(true, false) {
		select {
		case a.autoUpdateCard <- *release:
		default:
		}
		if a.window != nil {
			a.window.Invalidate()
		}
		return
	}
	a.startAutoUpdate(*release)
}

// processUpdateCard opens the download card queued by the launch check on the UI
// goroutine: showWindow touches the shell and the active screen.
func (a *application) processUpdateCard() {
	select {
	case release := <-a.autoUpdateCard:
		if a.active == a.main && !a.hidden.Load() {
			a.showUpdateScreen(release, true)
			return
		}
		// The user already moved on: fall back to the silent tray path.
		a.startAutoUpdate(release)
	default:
	}
}

// processPartyState opens the secret form when a guest's room became locked
// while the guest was on the main screen. The manager raises the flag from its
// consume loop; consuming it here keeps screen navigation on the UI goroutine.
func (a *application) processPartyState() {
	if a.party == nil {
		return
	}
	if a.active == a.main {
		if a.party.takeSecretNotice() {
			a.openParty()
		}
		return
	}
	// The party screen reads NeedsSecret directly. Drop the notice so it does
	// not reopen the card after the user leaves a room that was already
	// handled.
	if _, ok := a.active.(*partyWindow); ok {
		a.party.takeSecretNotice()
	}
}

func (a *application) startAutoUpdate(release updates.Release) {
	a.autoUpdateMu.Lock()
	if a.autoUpdateRunning {
		a.autoUpdateMu.Unlock()
		return
	}
	a.autoUpdateRunning = true
	ctx, cancel := context.WithCancel(context.Background())
	a.autoUpdateCancel = cancel
	a.autoUpdateMu.Unlock()

	go func() {
		defer func() {
			cancel()
			a.autoUpdateMu.Lock()
			a.autoUpdateCancel = nil
			a.autoUpdateRunning = false
			a.autoUpdateMu.Unlock()
		}()

		if !a.waitForAutoUpdate(ctx) {
			return
		}
		downloadCtx, stop := context.WithTimeout(ctx, autoUpdateDownloadTimeout)
		source, err := a.downloadRelease(downloadCtx, release, nil)
		stop()
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				fmt.Fprintln(os.Stderr, "yoanime: автоматическое обновление:", err)
			}
			return
		}
		// Playback may have started while the executable was downloading. Keep
		// the verified file and wait: installing now would close mpv.
		if !a.waitForAutoUpdate(ctx) {
			return
		}
		if err := a.applyDownload(source); err != nil {
			fmt.Fprintln(os.Stderr, "yoanime: автоматическое обновление:", err)
			return
		}
		a.quitForUpdate()
	}()
}

// waitForAutoUpdate blocks until installing an update cannot interrupt the
// user. The settings are re-read on every poll, so disabling auto-update also
// aborts a pending install.
func (a *application) waitForAutoUpdate(ctx context.Context) bool {
	ticker := time.NewTicker(autoUpdatePollInterval)
	defer ticker.Stop()
	for {
		if !a.autoUpdatesEnabled() {
			return false
		}
		if a.autoUpdateIdle() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-a.autoUpdateWake:
			// The screen changed or mpv closed; recheck immediately instead of
			// waiting for the next poll.
		case <-ticker.C:
		}
	}
}

// wakeAutoUpdate nudges a waiting silent install to re-evaluate its conditions.
func (a *application) wakeAutoUpdate() {
	select {
	case a.autoUpdateWake <- struct{}{}:
	default:
	}
}

func (a *application) autoUpdateIdle() bool {
	playing := a.playbackStarting.Load() || (a.playback != nil && a.playback.Active())
	inParty := a.party != nil && a.party.HasRoom()
	return autoUpdateSafe(playing, inParty, a.hidden.Load(), a.quitting.Load())
}

// autoUpdateSafe keeps the policy free of UI state so it can be tested. The
// executable may only be replaced while the window is in the tray, nothing is
// playing and no watch party is running.
func autoUpdateSafe(playing, party, hidden, quitting bool) bool {
	return !playing && !party && hidden && !quitting
}

func (a *application) cancelAutoUpdate() {
	a.autoUpdateMu.Lock()
	cancel := a.autoUpdateCancel
	a.autoUpdateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// startEpisodeLoop runs the new-episode checker in the background. A single
// goroutine owns every pass, so a slow library cannot overlap two walks of the
// same tracking table.
func (a *application) startEpisodeLoop(w *app.Window) {
	a.episodeMu.Lock()
	if a.episodeStarted {
		a.episodeMu.Unlock()
		return
	}
	a.episodeStarted = true
	a.episodeMu.Unlock()

	go func() {
		defer close(a.episodeDone)
		// History saved before episode totals were cached needs one slow
		// backfill pass. It shares the checker goroutine and its context, so a
		// disabled feature or a closed window stops it before the database is
		// closed.
		a.backfillEpisodeTotals(w)
		timer := time.NewTimer(episodeCheckStartupDelay)
		defer timer.Stop()
		for {
			select {
			case <-a.episodeStop:
				return
			case <-timer.C:
			case <-a.episodeWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			a.checkEpisodeNotifications(w)
			timer.Reset(episodeCheckInterval)
		}
	}()
}

// stopEpisodeLoop cancels an in-flight AnimeLIB pass and waits for the checker
// goroutine before close() destroys the database.
func (a *application) stopEpisodeLoop() {
	a.episodeMu.Lock()
	started := a.episodeStarted
	a.episodeStarted = false
	a.episodeMu.Unlock()
	if !started {
		return
	}
	a.episodeCancel()
	close(a.episodeStop)
	<-a.episodeDone
}

func (a *application) triggerEpisodeCheck() {
	select {
	case a.episodeWake <- struct{}{}:
	default:
	}
}

func (a *application) cancelEpisodePass() {
	a.episodePassMu.Lock()
	cancel := a.episodePassCancel
	a.episodePassMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// backfillEpisodeTotals fills the cached episode count for older history rows.
// It runs before the regular checker so a fresh install of this build draws its
// progress bars without the user opening every title first.
func (a *application) backfillEpisodeTotals(w *app.Window) {
	if a.library == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.episodeCtx, 30*time.Minute)
	defer cancel()
	refresh := func() {
		if a.main != nil {
			a.main.requestHistoryRefresh()
		}
		if w != nil {
			w.Invalidate()
		}
	}
	if err := a.library.RefreshMissingEpisodeTotals(ctx, refresh); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "yoanime: подсчёт серий:", err)
	}
}

func (a *application) checkEpisodeNotifications(w *app.Window) {
	if a.library == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.episodeCtx, 30*time.Minute)
	a.episodePassMu.Lock()
	a.episodePassCancel = cancel
	a.episodePassMu.Unlock()
	defer func() {
		cancel()
		a.episodePassMu.Lock()
		a.episodePassCancel = nil
		a.episodePassMu.Unlock()
	}()

	if err := a.library.CheckEpisodeNotifications(ctx, a.notifyEpisode); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "yoanime: проверка новых серий:", err)
	}
	if a.main != nil {
		// A new episode also updates the cached total shown in history.
		a.main.requestHistoryRefresh()
	}
	if w != nil {
		// Cached statuses may have changed from ongoing to completed.
		w.Invalidate()
	}
}

// notifyEpisode is the delivery callback shared by the checker and the
// temporary test button.
func (a *application) notifyEpisode(event library.EpisodeNotification) error {
	if a.tray == nil {
		return fmt.Errorf("уведомления Windows недоступны")
	}
	title, body := formatEpisodeNotification(event)
	return a.tray.notifyMessage(title, body)
}

// syncFooterControls recalculates the footer controls before every frame. The
// cached update is shown immediately after launch, without waiting for a
// network check. Settings belongs to the main screen only: other screens own
// their footer.
func (a *application) syncFooterControls() {
	release := a.updatesAvailable.Load()
	onMain := a.active == a.main
	a.shell.settingsVisible = onMain
	a.shell.logVisible = onMain
	a.shell.partyVisible = onMain
	a.shell.partyActive = onMain && a.party != nil && a.party.HasRoom()
	a.shell.updateVisible = onMain && a.updateAvailable(release)
	if onMain && a.main != nil {
		a.shell.notice = a.main.headerNotice
	} else {
		a.shell.notice = nil
	}
}

// updateAvailable applies the same rule to the footer button, the update
// screen and the silent updater: the release must belong to the selected
// channel, and a re-pushed version qualifies when its published build differs
// from the running executable.
func (a *application) updateAvailable(release *updates.Release) bool {
	if release == nil {
		return false
	}
	if !a.updateChannel().Accepts(release.Version) {
		return false
	}
	return updates.IsUpdate(release, version.Version, a.currentSHA)
}

func (a *application) updateChannel() updates.Channel {
	if a.library != nil && a.library.BetaUpdatesEnabled() {
		return updates.ChannelBeta
	}
	return updates.ChannelStable
}

func (a *application) autoUpdatesEnabled() bool {
	return a.library != nil && a.library.AutoUpdateEnabled()
}

func (a *application) openTitle(item historyTitle) {
	if a.window == nil {
		return
	}
	log.Info("title page opened", "title", item.title, "id", item.id, "slug", item.slug)
	preferredQuality := 1080
	if a.library != nil {
		preferredQuality = a.library.PreferredQuality()
	}
	title := newTitleWindow(a.shell.typo, item, preferredQuality)
	title.hidden = a.windowHidden
	title.onError = a.reportError
	title.onUserStatus = func(item historyTitle, status library.UserStatus) error {
		return a.setUserStatus(item, status)
	}
	title.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	title.onSave = func(voice, episode string) error {
		if a.library == nil {
			return fmt.Errorf("база данных недоступна")
		}
		// The title is persisted here, not when it is opened from the URL
		// field: Save is the only action that adds it to history.
		saved, err := a.library.Save(libraryTitleFromHistory(title.item), voice, episode)
		if err != nil {
			return err
		}
		// Remember the assigned id so a second Save updates the same entry.
		item.id = saved.ID
		item.coverPath = saved.CoverPath
		item.coverNeedsRetry = saved.CoverNeedsRetry
		title.applySaved(item)
		a.main.refreshHistory(saved.ID)
		a.downloadSavedCover(saved)
		a.clearError()
		return nil
	}
	title.onQuality = func(value int) error {
		if a.library == nil {
			return nil
		}
		// A Kinopoisk film chooses the quality of that film, not an app-wide
		// preference: the choice only travels to the watch button.
		if title.item.source == kinopoisk.Source && title.item.movie {
			return nil
		}
		return a.library.SetPreferredQuality(value)
	}
	title.onVoice = func(index int) {
		a.probeQualities(title)
	}
	title.onWatch = func(voice, episode string, quality int) error {
		if a.library == nil {
			return fmt.Errorf("база данных недоступна")
		}
		return a.startPlayback(libraryTitleFromHistory(title.item), voice, episode, quality, nil, false, 0)
	}
	a.showWindow(a.window, title)
	a.probeQualities(title)
}

// playFromHistory starts mpv directly from the history Play button. Episodes
// and voiceovers are not cached in SQLite, so startPlayback reloads them in the
// background when they are missing. The title page is only opened after mpv
// exits, as requested.
func (a *application) playFromHistory(item historyTitle) historyPlayResult {
	if a.library == nil || a.playback == nil {
		return historyPlayFailed
	}
	voice := ""
	if item.selectedVoice >= 0 && item.selectedVoice < len(item.voices) {
		selected := item.voices[item.selectedVoice]
		if len(item.episodes) > 0 && len(selected.episodes) == 0 {
			// The saved voice no longer has any episode. Let the main window
			// reload the title and show its page instead of launching an empty
			// playlist.
			return historyPlayFailed
		}
		voice = selected.name
	}
	episode := ""
	if item.selectedEpisode >= 0 && item.selectedEpisode < len(item.episodes) {
		episode = item.episodes[item.selectedEpisode]
	}
	quality := a.library.PreferredQuality()
	returnItem := item
	err := a.startPlayback(libraryTitleFromHistory(item), voice, episode, quality, &returnItem, false, 0)
	if err != nil {
		a.main.loadError = err.Error()
		a.reportError(err)
		a.main.playing = false
		a.main.playingID = 0
		if a.window != nil {
			a.window.Invalidate()
		}
		return historyPlayFailed
	}
	return historyPlayStarted
}

// startPlayback builds the plan and launches mpv in the background. The
// application window is hidden only after mpv has accepted the first file.
func (a *application) startPlayback(title library.Title, voice, episode string, quality int, historyItem *historyTitle, partyGuest bool, startAt float64) error {
	if a.library == nil || a.playback == nil {
		return fmt.Errorf("воспроизведение недоступно")
	}
	if a.window == nil {
		return fmt.Errorf("окно приложения не готово")
	}
	window := playback.WindowRect{}
	// mpv reopens where the user parked it last time; only a fresh install or
	// a missing record falls back to the application window rectangle.
	if a.mpvRectOK && a.mpvRect.Width > 0 && a.mpvRect.Height > 0 {
		window = playback.WindowRect{
			X:      a.mpvRect.X,
			Y:      a.mpvRect.Y,
			Width:  a.mpvRect.Width,
			Height: a.mpvRect.Height,
		}
	} else if rect, ok := appWindowRect(); ok {
		window = playback.WindowRect{
			X:      rect.X,
			Y:      rect.Y,
			Width:  rect.Width,
			Height: rect.Height,
		}
	}
	go func() {
		a.playbackStarting.Store(true)
		defer a.playbackStarting.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), playbackStartTimeout)
		defer cancel()
		// Playback start must not sit in a rate-limit pause: the row spinner
		// would look hung. Fail fast and let the header show the countdown.
		ctx = animelib.FailFastOnRateLimit(ctx)
		if len(title.Voiceovers) == 0 || len(title.Episodes) == 0 {
			// History rows keep only the saved selection, never the episode
			// list or the voiceovers. Reload them from AnimeLIB so a direct
			// Play from history can start mpv without opening the title page
			// first; the saved voice and episode are restored during the load.
			loaded, loadErr := a.library.Load(ctx, title.Slug)
			if loadErr != nil {
				a.deliverWatch(watchResult{err: playbackStartError(loadErr), historyFailure: true, party: partyGuest})
				return
			}
			title = loaded
			voice, episode = loaded.SelectedVoice, loaded.SelectedEpisode
			fresh := titleFromLibrary(loaded)
			historyItem = &fresh
		}
		plan, err := a.library.PlaybackPlan(ctx, title, voice, episode)
		if err != nil {
			a.deliverWatch(startFailureResult(err, historyItem, 0, partyGuest))
			return
		}
		// Watching a title is enough to keep its progress. Track it only after
		// the plan resolves: a title whose selected voice has no playable
		// episode must not be promoted from "В планах" to "Смотрю" by a plan
		// that cannot resolve an episode. A link-opened title has no history row
		// until Save, and a party guest has to keep the title and voice they
		// watched together.
		trackedID := int64(0)
		if id, trackErr := a.library.TrackViewing(title, voice, episode); trackErr == nil && id != 0 {
			trackedID = id
			title.ID = id
			plan.TitleID = id
		}
		if partyGuest {
			// The room owns the position: a guest must not resume from its own
			// history, which would start the file in the wrong place.
			plan.ResumeAt = startAt
		}
		inputLocked := false
		if partyGuest && a.party != nil {
			inputLocked = !a.party.GuestControlAllowed()
		}
		// Auto-skip belongs to the room owner. A guest follows the host's
		// picture, so its own chapter settings must not move the timeline.
		var autoSkip map[metadata.Segment]bool
		if !partyGuest {
			autoSkip = a.library.AutoSkipSegments()
		}
		session, err := a.playback.Start(ctx, plan, playback.StartOptions{
			Quality:          quality,
			Window:           window,
			AutoSkipSegments: autoSkip,
			InputLocked:      inputLocked,
			OnReady: func() {
				// The application window stays visible next to mpv: the title
				// page mirrors the playing episode and its progress live.
				if a.window != nil {
					a.window.Invalidate()
				}
			},
			OnFinish: func(err error) {
				a.deliverWatch(watchResult{finished: true, err: err, historyItem: historyItem, titleID: trackedID, party: partyGuest})
			},
		})
		if err != nil {
			a.deliverWatch(startFailureResult(err, historyItem, trackedID, partyGuest))
			return
		}
		if !partyGuest && a.party != nil {
			a.party.hostStarted(session, title, voice, episode)
		}
	}()
	return nil
}

// playbackStartError turns the watchdog deadline into a reason the user can
// act on instead of the raw context error.
func playbackStartError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("не удалось начать воспроизведение за %s — попробуйте ещё раз", playbackStartTimeout)
	}
	if errors.Is(err, animelib.ErrRateLimited) {
		if remaining := animelib.RateLimitRemaining(); remaining > 0 {
			return fmt.Errorf("AnimeLIB временно ограничил запросы — попробуйте снова через %s", rateLimitWaitLabel(remaining))
		}
		return errors.New("AnimeLIB временно ограничил запросы — попробуйте снова чуть позже")
	}
	return err
}

// startFailureResult turns a failed playback start into a watch result. A
// rate-limit pause on a direct Play from history keeps the user on the main
// window: the row spinner stops and the header keeps showing the countdown,
// instead of opening the title page for an error that will clear by itself.
func startFailureResult(err error, historyItem *historyTitle, titleID int64, partyGuest bool) watchResult {
	result := watchResult{
		err:         playbackStartError(err),
		historyItem: historyItem,
		titleID:     titleID,
		party:       partyGuest,
	}
	if historyItem != nil && errors.Is(err, animelib.ErrRateLimited) {
		result.historyItem = nil
		result.historyFailure = true
	}
	return result
}

func (a *application) deliverWatch(result watchResult) {
	// The UI drains this channel every frame and there can only be one active
	// session, so blocking here is safe. Dropping a result used to leave the
	// history row or the title page spinning forever.
	a.watchResults <- result
	if a.window != nil {
		a.window.Invalidate()
	}
}

// processWatchResults updates the UI after a playback start failure or after
// mpv exits.
func (a *application) processWatchResults(gtx layout.Context) {
	for {
		select {
		case result := <-a.watchResults:
			a.handleWatchResult(result)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

// processQualityResults applies the background Kinopoisk probes that arrived
// since the last frame.
func (a *application) processQualityResults(gtx layout.Context) {
	for {
		select {
		case result := <-a.qualityResults:
			a.applyQualityResult(result)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

// applyQualityResult hands a probed ladder to the title page that asked for it.
// A stale result (the user already navigated away) is dropped.
func (a *application) applyQualityResult(result qualityResult) {
	title, ok := a.active.(*titleWindow)
	if !ok || title.item.slug != result.slug {
		return
	}
	if result.err != nil {
		// The probe is a nicety: the standard quality buttons stay enabled and
		// playback still picks the nearest available step.
		log.Warn("Kinopoisk quality probe failed", "title", result.slug, "voice", result.voice, "error", result.err)
		return
	}
	index := -1
	for voiceIndex, voice := range title.item.voices {
		if voice.name == result.voice {
			index = voiceIndex
			break
		}
	}
	if index < 0 {
		return
	}
	title.applyQualities(index, result.values)
}

// probeQualities learns which stream qualities the selected Kinopoisk voice
// offers and disables the buttons it cannot deliver. The probe runs in the
// background with the shared headless Chromium and never opens a window; its
// result is cached when the user saves the title to history.
func (a *application) probeQualities(title *titleWindow) {
	if a.library == nil || a.playback == nil || title == nil {
		return
	}
	if title.item.source != kinopoisk.Source {
		return
	}
	index := title.selectedVoice
	if index < 0 || index >= len(title.item.voices) {
		return
	}
	voice := title.item.voices[index]
	if len(voice.qualities) > 0 || len(voice.episodes) == 0 {
		return
	}
	item := title.item
	libraryTitle := libraryTitleFromHistory(item)
	voiceName := voice.name
	episode := voice.episodes[0]
	preferred := a.library.PreferredQuality()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), qualitiesProbeTimeout)
		defer cancel()
		plan, err := a.library.PlaybackPlan(ctx, libraryTitle, voiceName, episode)
		var values []int
		if err == nil {
			values, err = a.playback.Qualities(ctx, plan, preferred)
		}
		select {
		case a.qualityResults <- qualityResult{slug: item.slug, voice: voiceName, values: values, err: err}:
		default:
		}
		if a.window != nil {
			a.window.Invalidate()
		}
	}()
}

func (a *application) handleWatchResult(result watchResult) {
	if result.finished {
		// mpv is gone, so a pending silent install may proceed even when the
		// screen stayed the same.
		a.wakeAutoUpdate()
	}
	if result.party {
		// A room-driven session keeps its progress like any other playback, so
		// the list has to show the new position (and the title a guest watched
		// for the first time) once mpv closes.
		if a.party != nil {
			if result.err != nil && !result.finished {
				a.party.guestFailed(result.err)
			} else if result.finished {
				// A guest that closed the player leaves the room instead of
				// sitting on the waiting card with the party control lit.
				a.party.guestFinished(result.err)
			}
		}
		if result.finished && a.main != nil {
			a.main.requestHistoryRefresh()
		}
		return
	}
	if result.historyFailure {
		// Loading the title for a direct Play from history failed before mpv
		// could start. Stay on the main window and show the reason: there is
		// no loaded title data to open a title page with.
		if a.main != nil {
			a.main.playing = false
			a.main.playingID = 0
			if result.err != nil {
				a.main.loadError = result.err.Error()
				a.reportError(result.err)
			}
		}
		return
	}
	if result.historyItem != nil {
		if a.main != nil && a.main.playingID == result.historyItem.id {
			a.main.playing = false
			a.main.playingID = 0
		}
		if result.err != nil && !result.finished {
			if a.main != nil {
				a.main.loadError = result.err.Error()
			}
			a.reportError(result.err)
			// A direct Play from history must leave the user on the title page
			// when playback cannot start (for example, the selected voice no
			// longer has any episodes). No mpv window is left behind.
			a.openTitle(*result.historyItem)
			if title, ok := a.active.(*titleWindow); ok {
				title.setWatchError(result.err.Error())
			}
			return
		}
		if !result.finished {
			return
		}
		if a.main != nil {
			a.main.refreshHistory(result.historyItem.id)
		}
		updated := *result.historyItem
		if a.library != nil && updated.id != 0 {
			// Playback may have promoted "В планах" to "Смотрю" or completed
			// the title. Reload the pill before the title page is opened.
			updated.userStatus = a.library.UserStatus(updated.id)
			episode, resumeAt, duration := a.library.Progress(updated.id)
			updated.resumeAt = resumeAt
			updated.duration = duration
			if index := episodeIndex(updated, episode); index >= 0 {
				updated.selectedEpisode = index
			}
		}
		if a.main != nil {
			for _, item := range a.main.history {
				if item.id == updated.id {
					// Visible history rows only carry the refreshed cover and
					// progress. Keep the episode list loaded for playback.
					updated.coverPath = item.coverPath
					updated.coverNeedsRetry = item.coverNeedsRetry
					break
				}
			}
			if result.err != nil {
				a.main.loadError = result.err.Error()
				a.reportError(result.err)
			}
		}
		a.openTitle(updated)
		return
	}

	title, ok := a.active.(*titleWindow)
	if !ok {
		return
	}
	if result.titleID != 0 && title.item.id == 0 {
		// Playback tracked a link-opened title in history; adopt the row so the
		// progress refresh and a later Save use the same entry.
		title.item.id = result.titleID
	}
	if a.library != nil && title.item.id != 0 {
		// TrackViewing promotes planned titles and the final EOF marks the
		// title completed. Reflect both before the user sees the page again.
		title.setUserStatus(a.library.UserStatus(title.item.id))
	}
	if result.err != nil {
		title.setWatchError(result.err.Error())
		a.reportError(result.err)
	} else if result.finished {
		title.setWatching(false)
		a.clearError()
	}
	if !result.finished {
		return
	}
	if a.library != nil && title.item.id != 0 {
		episode, resumeAt, duration := a.library.Progress(title.item.id)
		title.applyProgress(episode, resumeAt, duration)
		if a.main != nil {
			a.main.refreshHistory(title.item.id)
		}
	}
}

// downloadSavedCover fetches the poster of a freshly saved title in the
// background and hands it to the main window through its cover-results channel.
func (a *application) downloadSavedCover(saved library.Title) {
	if saved.ID == 0 || saved.CoverPath != "" || saved.CoverURL == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		path, err := a.library.RefreshCover(ctx, saved.ID, saved.CoverURL)
		if err != nil || path == "" {
			return
		}
		select {
		case a.main.coverResults <- coverResult{id: saved.ID, path: path}:
		default:
		}
		if a.main.invalidate != nil {
			a.main.invalidate()
		}
	}()
}

// leaveParty closes the room and returns to the main screen. A guest that
// closes the player leaves the room this way, so the party control stops
// glowing instead of keeping a room nobody follows any more.
func (a *application) leaveParty() {
	if a.party != nil {
		a.party.Delete()
	}
	if a.window == nil {
		return
	}
	a.showWindow(a.window, a.main)
	a.window.Invalidate()
}

// restorePlayerWindow brings the application back when the app itself closed
// mpv. Session.Stop suppresses the finish callback that normally restores the
// window, so the caller has to do it.
func (a *application) restorePlayerWindow() {
	if !a.hidden.Load() {
		return
	}
	a.hidden.Store(false)
	showAppWindow()
	if a.window != nil {
		a.window.Invalidate()
	}
}

// updatePlaybackNow mirrors the running mpv session into the open title page:
// the playing episode stays highlighted and the header progress bar moves live.
// The one-second invalidate keeps the frame loop alive while the player runs,
// but a window parked in the tray never animates.
func (a *application) updatePlaybackNow(gtx layout.Context) {
	if a.playback == nil || a.window == nil || a.windowHidden() {
		return
	}
	session := a.playback.Current()
	if session == nil {
		return
	}
	title, ok := a.active.(*titleWindow)
	if !ok {
		return
	}
	number, _ := session.Episode()
	if number == "" {
		return
	}
	if slug := session.Slug(); slug != "" && slug != title.item.slug {
		return
	}
	title.applyProgress(number, session.LivePosition(), session.Duration())
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
}

// persistWindowRects remembers where the user keeps the application window and
// the mpv window, at most once a second each and only on change, so the next
// open lands on the same spot without writing the database every frame.
func (a *application) persistWindowRects(now time.Time) {
	if a.library == nil {
		return
	}
	if !a.windowHidden() && now.Sub(a.appRectAt) >= time.Second {
		if rect, ok := appWindowRect(); ok && rect != a.appRectCur {
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
		if !a.mpvRectOK || converted != a.mpvRect {
			a.mpvRect = converted
			a.mpvRectOK = true
			a.mpvRectAt = now
			_ = a.library.SetSetting("mpv_rect", encodeRect(converted))
		}
	}
}

func (a *application) showWindow(w *app.Window, screen windowScreen) {
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
func (a *application) installScreen(screen windowScreen) {
	a.shell.footer = nil
	if fs, ok := screen.(footerScreen); ok {
		a.shell.footer = func(gtx layout.Context) layout.Dimensions {
			fs.layoutFooter(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
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

// toggleShortcuts opens the F1 summary for the page that is currently active.
// The active screen keeps its state; only the content renderer is replaced, so
// returning restores its scroll position and selections exactly.
func (a *application) toggleShortcuts() {
	if a.window == nil {
		return
	}
	if a.shortcuts != nil {
		a.closeShortcuts()
		return
	}
	if a.active == nil {
		return
	}
	a.hidePopups()
	a.shortcuts = newShortcutsWindow(a.shell.typo, shortcutInfoFor(a.active))
	a.shortcuts.onBack = a.closeShortcuts
	shortcuts := a.shortcuts
	a.shell.footer = func(gtx layout.Context) layout.Dimensions {
		shortcuts.layoutFooter(gtx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	a.shell.overlay = nil
	a.shell.onPress = nil
	a.window.Invalidate()
}

func (a *application) closeShortcuts() {
	if a.shortcuts == nil {
		return
	}
	a.shortcuts = nil
	if a.active != nil {
		a.installScreen(a.active)
	}
	if a.window != nil {
		a.window.Invalidate()
	}
}

func shortcutInfoFor(screen windowScreen) shortcutInfo {
	if source, ok := screen.(shortcutSource); ok {
		return source.shortcutInfo()
	}
	return shortcutInfo{
		page: "Приложение",
		items: []shortcutItem{
			{keys: "F1", label: "Открыть или закрыть сводку"},
		},
	}
}

// handleGlobalKeys consumes F1 before the active screen sees it. Escape is
// claimed only while the summary is open, so normal screens keep their own
// Escape behaviour.
func (a *application) handleGlobalKeys(gtx layout.Context) {
	filters := []event.Filter{key.Filter{Name: key.NameF1}}
	if a.shortcuts != nil {
		filters = append(filters, key.Filter{Name: key.NameEscape})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameF1:
			a.toggleShortcuts()
		case key.NameEscape:
			a.closeShortcuts()
		}
	}
}

// overlayScreen is implemented by screens with a popup that must be drawn above
// the window chrome, such as the history drag preview and status menus.
type overlayScreen interface {
	layoutOverlay(layout.Context, image.Point)
}

// reportError logs the technical error and shows only a friendly sentence in
// the header. Cancellation is not a user-visible failure.
func (a *application) reportError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	log.Error("ui error", "error", err)
	message := friendlyError(err)
	if message == "" {
		return
	}
	a.shell.setError(message, time.Now())
	if a.window != nil {
		a.window.Invalidate()
	}
}

func (a *application) clearError() {
	a.shell.clearError()
}

// setUserStatus persists a status chosen from a pill menu and updates every
// screen that may be showing the same title.
func (a *application) setUserStatus(item historyTitle, status library.UserStatus) error {
	log.Info("user status changed", "title", item.id, "status", status)
	if item.id != 0 && a.library != nil {
		if err := a.library.SetUserStatus(item.id, status); err != nil {
			a.reportError(err)
			return err
		}
	}
	if a.main != nil {
		a.main.setUserStatus(item.id, status)
	}
	if title, ok := a.active.(*titleWindow); ok && title.item.id == item.id {
		title.setUserStatus(status)
	}
	a.clearError()
	if a.window != nil {
		a.window.Invalidate()
	}
	return nil
}

func (a *application) openLog() {
	if a.window == nil {
		return
	}
	screen := newLogWindow(a.shell.typo)
	screen.hidden = a.windowHidden
	screen.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	a.log = screen
	a.showWindow(a.window, screen)
}

func (a *application) close() {
	a.cancelAutoUpdate()
	a.stopEpisodeLoop()
	if a.party != nil {
		a.party.Close()
		a.party = nil
	}
	if a.tray != nil {
		a.tray.close()
		a.tray = nil
	}
	if a.playback != nil {
		a.playback.Close()
		a.playback = nil
	}
	if a.active != nil {
		a.active.Close()
		a.active = nil
	}
	if a.library != nil {
		// Rows removed within the undo window are still in the database; commit
		// them before the store goes away so they do not reappear next launch.
		a.main.finalizeHistoryDeletes()
		_ = a.library.Close()
		a.library = nil
	}
	if a.browserManager != nil {
		_ = a.browserManager.Close()
		a.browserManager = nil
	}
}

func (a *application) run(w *app.Window) error {
	var ops op.Ops

	for {
		e := w.Event()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case *app.ClosingEvent:
			// The close button and Alt+F4 park the window in the notification
			// area instead of ending the application. Only the tray menu is
			// allowed to close for real; without a tray icon nothing would be
			// left to reopen the window, so the request goes through.
			if a.tray == nil || a.quitting.Load() {
				break
			}
			e.Abort()
			a.hideToTray()
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.processUpdateCard()
			a.processPartyState()
			a.processWatchResults(gtx)
			a.processQualityResults(gtx)
			a.updatePlaybackNow(gtx)
			a.persistWindowRects(gtx.Now)
			a.syncFooterControls()
			a.handleGlobalKeys(gtx)
			a.shell.layout(gtx, w)
			e.Frame(gtx.Ops)
		case app.ConfigEvent:
			wasFocused := a.shell.cfg.Focused
			a.shell.setConfig(e.Config)
			if wasFocused && !e.Config.Focused {
				a.hidePopups()
				if a.active == a.main {
					a.main.clearSelection()
				}
				w.Invalidate()
			}
		case event.Event:
			handleViewEvent(e)
		}
	}
}

// Content can be installed by any window. The custom chrome remains common to
// every window; nil renders the empty state.
func (a *application) Content(w layout.Widget) {
	a.shell.content = w
}
