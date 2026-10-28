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
	"runtime/debug"
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

	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/libsocial"
	"github.com/arfush/yo/log"
	"github.com/arfush/yo/metadata"
	"github.com/arfush/yo/mpv"
	"github.com/arfush/yo/party"
	"github.com/arfush/yo/platform/appdata"
	"github.com/arfush/yo/playback"
	"github.com/arfush/yo/token"
	"github.com/arfush/yo/update"
	"github.com/arfush/yo/version"
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
	// which qualities a Filmlib voice offers. A cold Chromium is slow, but a
	// failed probe only leaves the standard buttons enabled.
	qualitiesProbeTimeout = 90 * time.Second
)

// Run starts the yo desktop application with the services built by the
// composition root.
func Run(services Services) {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("yo"),
			app.Size(unit.Dp(initialWidth), unit.Dp(initialHeight)),
			app.MinSize(unit.Dp(minimumWidth), unit.Dp(minimumHeight)),
			app.Decorated(false),
		)

		a, err := newApp(services)
		if err != nil {
			logger.Error("application startup failed", "error", err)
			fmt.Fprintln(os.Stderr, "yo:", err)
			os.Exit(1)
		}
		logger.Info("window attached")
		a.attachWindow(w)
		if err := a.run(w); err != nil {
			logger.Error("application stopped with an error", "error", err)
			a.close()
			fmt.Fprintln(os.Stderr, "yo:", err)
			os.Exit(1)
		}
		logger.Info("application closed")
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
	rateLimit      RateLimitState
	party          *partyManager
	main           *mainScreen
	log            *logScreen
	active         Screen
	// previousScreen is the screen the user came from. A panic in the active
	// screen returns there instead of leaving the window on a half-drawn page.
	previousScreen Screen
	recovering     bool
	shortcuts      *shortcutsScreen
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
	// qualityResults carries the background Filmlib quality probe. It runs
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
	updateStore      *update.Store
	updateClient     *http.Client
	downloadClient   *http.Client
	updatesAvailable atomic.Pointer[update.Release]
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
	autoUpdateCard chan update.Release
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

func newApp(services Services) (*application, error) {
	typo, err := newTypography()
	if err != nil {
		return nil, err
	}
	icon, err := loadAppIcon()
	if err != nil {
		return nil, fmt.Errorf("load app icon: %w", err)
	}
	sectionIcons, err := loadSectionIcons()
	if err != nil {
		return nil, fmt.Errorf("load section icons: %w", err)
	}
	lib := services.Library
	episodeCtx, episodeCancel := context.WithCancel(context.Background())
	app := &application{
		shell:             newShell(typo, icon, sectionIcons),
		browserManager:    services.Browser,
		mpvManager:        services.MPV,
		library:           lib,
		playback:          services.Playback,
		main:              newMainScreen(typo, lib, services.History),
		icon:              icon,
		watchResults:      make(chan watchResult, 8),
		qualityResults:    make(chan qualityResult, 8),
		episodeCtx:        episodeCtx,
		episodeCancel:     episodeCancel,
		episodeWake:       make(chan struct{}, 1),
		episodeStop:       make(chan struct{}),
		episodeDone:       make(chan struct{}),
		tokenPath:         services.TokenPath,
		browserProfileDir: services.BrowserProfileDir,
		updateStore:       services.UpdateStore,
		updateClient:      &http.Client{Timeout: 25 * time.Second},
		downloadClient:    &http.Client{Timeout: 30 * time.Minute},
		updateWake:        make(chan struct{}, 1),
		autoUpdateWake:    make(chan struct{}, 1),
		autoUpdateCard:    make(chan update.Release, 1),
		currentSHA:        services.CurrentSHA,
	}
	app.rateLimit = services.RateLimit
	app.main.rateLimit = services.RateLimit
	app.shell.rateLimit.state = services.RateLimit
	section := sectionAnime
	if raw, ok := lib.Setting(librarySectionKey); ok {
		section = sectionFromSetting(raw)
	}
	// Restore the palette, header name and per-section history before the first
	// frame. Without this the saved manga/ranobe section would open with the
	// anime history until the user switched away and back.
	applySectionTheme(section)
	app.main.setSection(section)
	app.shell.selectSection(section)
	setAppIcon(app.shell.sectionIcon())
	state := services.UpdateStore.State()
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

// attachWindow checks the AnimeLIB token first, then makes sure Chromium is
// available when a new token has to be generated, then installs mpv when it is
// missing, and only after that opens the main window.
func (a *application) attachWindow(w *app.Window) {
	a.window = w
	// A 429 must repaint the header immediately, even if the screen does not
	// animate on its own.
	if a.rateLimit != nil {
		a.rateLimit.SetListener(w.Invalidate)
	}
	a.main.invalidate = w.Invalidate
	a.main.hidden = a.windowHidden
	a.shell.hidden = a.windowHidden
	a.main.onOpenTitle = func(item historyTitle) {
		if item.reading() {
			a.openReadingTitle(item)
			return
		}
		a.openTitle(item)
	}
	a.main.onPlayHistory = func(item historyTitle) historyPlayResult {
		return a.playFromHistory(item)
	}
	a.main.onSection = func(section librarySection) {
		a.selectSection(section)
	}
	a.main.onUserStatus = func(item historyTitle, status library.UserStatus) error {
		return a.setUserStatus(item, status)
	}
	a.main.onError = a.reportError
	a.main.onClearError = a.clearError
	a.shell.onSection = a.selectSection
	a.shell.onUpdate = a.openUpdate
	a.shell.onSettings = a.openSettings
	a.shell.onParty = a.openParty
	a.shell.onLog = a.openLog
	a.startTray()
	a.prepare(w)
	a.startUpdateLoop(w)
	a.startEpisodeLoop(w)
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
			err := a.browserManager.Download(ctx, func(progress browser.Progress) {
				report(downloadProgress{Percent: progress.Percent, Preparing: progress.Preparing})
			})
			if err != nil {
				return errInstallBrowser.wrap(err)
			}
			return nil
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

	prompt := newTokenPrompt(a.shell.typo, tokenPromptConfig{
		Authorize: func(ctx context.Context, site, profileDir string, bounds *catalog.Bounds) error {
			value, err := a.browserManager.Authorize(ctx, site, profileDir, bounds)
			if err != nil {
				return errAuthorize.wrap(err)
			}
			if err := token.Save(a.tokenPath, value); err != nil {
				return errAuthorize.wrap(err)
			}
			return nil
		},
		Window:          w,
		DefaultDomain:   browser.DefaultDomain,
		NormalizeDomain: browser.NormalizeDomain,
	}, a.browserProfileDir)
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
			err := a.mpvManager.Download(ctx, func(progress mpv.Progress) {
				report(downloadProgress{Percent: progress.Percent, Preparing: progress.Preparing})
			})
			if err != nil {
				return errInstallMPV.wrap(err)
			}
			return nil
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
func (a *application) offerStartupUpdate(release *update.Release) bool {
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
func (a *application) showUpdateScreen(release update.Release, auto bool) {
	if a.window == nil {
		return
	}
	screen := newFirstRun(
		a.shell.typo,
		a.window,
		"Обновление yo",
		fmt.Sprintf("Скачиваем %s. Приложение перезапустится автоматически.", release.Version),
		func(ctx context.Context, report func(downloadProgress)) error {
			if err := a.downloadAndApply(ctx, release, report); err != nil {
				return errUpdate.wrap(err)
			}
			return nil
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
	settings := newSettingsScreen(a.shell.typo, settingsValues{
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
			return errSettingsSave.wrap(err)
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
			return errSettingsSave.wrap(err)
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
			return errSettingsSave.wrap(err)
		}
		// The channel changed, so a pending download may no longer belong to
		// the selected channel. Refetch before starting another silent install.
		a.cancelAutoUpdate()
		a.triggerUpdateCheck()
		return nil
	}
	settings.onControl = func(enabled bool) error {
		if err := a.library.SetSharedControlEnabled(enabled); err != nil {
			return errSettingsSave.wrap(err)
		}
		a.party.SetSharedControl(enabled)
		return nil
	}
	settings.onAutoSkip = func(segment metadata.Segment, enabled bool) error {
		if err := a.library.SetAutoSkipSegment(segment, enabled); err != nil {
			return errSettingsSave.wrap(err)
		}
		return nil
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

func (a *application) downloadAndApply(ctx context.Context, release update.Release, report func(downloadProgress)) error {
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
func (a *application) downloadRelease(ctx context.Context, release update.Release, report func(downloaded, total int64)) (string, error) {
	target, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine the application path: %w", err)
	}
	if err := update.CheckTarget(target); err != nil {
		return "", err
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return "", err
	}
	name := filepath.Base(release.AssetName)
	if name == "." || name == "" {
		name = update.AssetName(release.Version, runtime.GOARCH)
	}
	destination := filepath.Join(updateDir, name)
	return update.Download(ctx, a.downloadClient, release, destination, report)
}

// applyDownload starts the hidden helper that replaces the executable after
// this process exits.
func (a *application) applyDownload(source string) error {
	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine the application path: %w", err)
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return err
	}
	logPath, err := appdata.UpdateLogPath()
	if err != nil {
		return err
	}
	plan, err := update.Prepare(target, source, updateDir, logPath)
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
	if update.Repository == "" {
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
	release, err := update.FetchReleaseChannel(ctx, a.updateClient, a.updateChannel())
	now := time.Now()
	if err != nil {
		if errors.Is(err, update.ErrNoRelease) {
			release = nil
			err = nil
		} else {
			// A network failure must not hide the release cached from the last
			// successful check.
			fmt.Fprintln(os.Stderr, "yo: проверка обновлений:", err)
			_ = a.updateStore.SaveLastCheck(now)
			return updateRetryInterval
		}
	}
	if saveErr := a.updateStore.RecordCheck(release, now); saveErr != nil {
		fmt.Fprintln(os.Stderr, "yo: сохранение состояния обновлений:", saveErr)
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
func (a *application) maybeAutoUpdate(release *update.Release) {
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
func (a *application) startAutoUpdate(release update.Release) {
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
				fmt.Fprintln(os.Stderr, "yo: автоматическое обновление:", err)
			}
			return
		}
		// Playback may have started while the executable was downloading. Keep
		// the verified file and wait: installing now would close mpv.
		if !a.waitForAutoUpdate(ctx) {
			return
		}
		if err := a.applyDownload(source); err != nil {
			fmt.Fprintln(os.Stderr, "yo: автоматическое обновление:", err)
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
// syncFooterControls recalculates the footer controls before every frame. The
// cached update is shown immediately after launch, without waiting for a
// network check. Settings belongs to the main screen only: other screens own
// their footer.
func (a *application) syncFooterControls() {
	release := a.updatesAvailable.Load()
	onMain := a.active == a.main
	a.shell.settingsVisible = onMain
	a.shell.logVisible = onMain
	a.shell.partyVisible = onMain && a.main != nil && a.main.section == sectionAnime
	a.shell.partyActive = a.shell.partyVisible && a.party != nil && a.party.HasRoom()
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
func (a *application) updateAvailable(release *update.Release) bool {
	if release == nil {
		return false
	}
	if !a.updateChannel().Accepts(release.Version) {
		return false
	}
	return update.IsUpdate(release, version.Version, a.currentSHA)
}

func (a *application) updateChannel() update.Channel {
	if a.library != nil && a.library.BetaUpdatesEnabled() {
		return update.ChannelBeta
	}
	return update.ChannelStable
}

func (a *application) autoUpdatesEnabled() bool {
	return a.library != nil && a.library.AutoUpdateEnabled()
}

// selectSection switches the library header mode and persists the choice so the
// next launch reopens on the same screen. The saver's auto-switch target is
// also updated so the menu remembers the intended section even after a chain of
// section changes.
func (a *application) selectSection(section librarySection) {
	previous := sectionAnime
	if a.main != nil {
		previous = a.main.section
	}
	applySectionTheme(section)
	a.main.setSection(section)
	if a.library != nil {
		go func() { _ = a.library.SetSetting(librarySectionKey, section.key()) }()
	}
	if a.shell != nil {
		a.shell.selectSection(section)
		setAppIcon(a.shell.sectionIcon())
	}
	logger.Info("section switched", "from", previous.key(), "to", section.key(), "history", len(a.main.allHistory), "visible", len(a.main.history))
	if a.window != nil {
		a.window.Invalidate()
	}
}

// openReaderScreen is the direct reader entry kept for older call sites. Reads
// normally enter through the manga/ranobe title screen now.
func (a *application) openReaderScreen(item historyTitle) {
	a.openReader(item, libraryTitleFromHistory(item), a.main)
}

// openReader opens the dedicated manga or ranobe reader. back is the screen
// that receives control when the reader closes: a reading title page in the
// normal flow, or the library when a caller opens the reader directly.
func (a *application) openReader(item historyTitle, title catalog.Title, back Screen) {
	if a.window == nil {
		return
	}
	logger.Info("reader opened", "title", item.title, "id", item.id, "slug", item.slug)
	readerItem := titleFromLibrary(title)
	reader := newReaderScreen(a.shell.typo, readerItem, a.library)
	reader.hidden = a.windowHidden
	reader.invalidate = a.window.Invalidate
	reader.onError = a.reportError
	reader.onBack = func() {
		if a.window == nil {
			return
		}
		if back == nil {
			back = a.main
		}
		a.showWindow(a.window, back)
		a.window.Invalidate()
	}
	reader.prefill(title)
	a.showWindow(a.window, reader)
	a.window.Invalidate()
}

// openReadingTitle opens the manga or ranobe landing page. It delegates drawing
// to the shared title screen, so only labels and the forward action differ.
func (a *application) openReadingTitle(item historyTitle) {
	if a.window == nil {
		return
	}
	logger.Info("reading title page opened", "title", item.title, "id", item.id, "slug", item.slug)
	screen := newReadingTitleScreen(a.shell.typo, item)
	screen.hidden = a.windowHidden
	screen.invalidate = a.window.Invalidate
	screen.onError = a.reportError
	loaded := item
	screen.load = func(ctx context.Context, input string) (catalog.Title, error) {
		if a.library == nil {
			return catalog.Title{}, library.ErrDatabaseUnavailable
		}
		if loaded.id != 0 {
			return a.library.Load(ctx, input)
		}
		return a.library.Fetch(ctx, input)
	}
	screen.saveFunc = func(title catalog.Title, voice, chapter string) (catalog.Title, error) {
		if a.library == nil {
			return catalog.Title{}, library.ErrDatabaseUnavailable
		}
		saved, err := a.library.Save(title, voice, chapter)
		if err != nil {
			return catalog.Title{}, errSaveTitle.wrap(err)
		}
		a.main.refreshHistory(saved.ID)
		a.downloadSavedCover(saved)
		a.clearError()
		return saved, nil
	}
	screen.onSaved = func(title catalog.Title) {
		loaded = titleFromLibrary(title)
	}
	screen.onUserStatus = func(item historyTitle, status library.UserStatus) error {
		return a.setUserStatus(item, status)
	}
	screen.onRead = func(item historyTitle, title catalog.Title, voice, chapter string) {
		title.SelectedVoice = voice
		title.SelectedEpisode = chapter
		a.openReader(item, title, screen)
	}
	screen.onBack = func() {
		if a.window == nil {
			return
		}
		a.showWindow(a.window, a.main)
		a.window.Invalidate()
	}
	a.showWindow(a.window, screen)
	a.window.Invalidate()
}

func (a *application) openTitle(item historyTitle) {
	if a.window == nil {
		return
	}
	logger.Info("title page opened", "title", item.title, "id", item.id, "slug", item.slug)
	preferredQuality := 1080
	if a.library != nil {
		preferredQuality = a.library.PreferredQuality()
	}
	title := newTitleScreen(a.shell.typo, item, preferredQuality)
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
			return library.ErrDatabaseUnavailable
		}
		// The title is persisted here, not when it is opened from the URL
		// field: Save is the only action that adds it to history.
		saved, err := a.library.Save(libraryTitleFromHistory(title.item), voice, episode)
		if err != nil {
			return errSaveTitle.wrap(err)
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
		// A Filmlib film chooses the quality of that film, not an app-wide
		// preference: the choice only travels to the watch button.
		if title.item.source == filmlib.Source && title.item.movie {
			return nil
		}
		if err := a.library.SetPreferredQuality(value); err != nil {
			return errSettingsSave.wrap(err)
		}
		return nil
	}
	title.onVoice = func(index int) {
		a.probeQualities(title)
	}
	title.onWatch = func(voice, episode string, quality int) error {
		if a.library == nil {
			return library.ErrDatabaseUnavailable
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
	if item.reading() {
		// The reader has no "play" step: the history Play button falls back to
		// the normal load flow, which opens the reader and tracks the chapter.
		// mpv must never take over a manga or ranobe title.
		return historyPlayFailed
	}
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
		a.main.loadError = userErrorText(err)
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
	return a.startPlaybackNotify(title, voice, episode, quality, historyItem, partyGuest, startAt, nil)
}

// startPlaybackNotify starts playback in the background and reports the final
// launch result through onStart. The callback is called exactly once on every
// path that enters the goroutine, including failures before mpv opens. A party
// guest uses it to keep one start in flight until the session really exists.
func (a *application) startPlaybackNotify(title library.Title, voice, episode string, quality int, historyItem *historyTitle, partyGuest bool, startAt float64, onStart func(error)) error {
	if a.library == nil || a.playback == nil {
		return userMessage("Воспроизведение сейчас недоступно.")
	}
	if a.window == nil {
		return userMessage("Окно приложения ещё не готово.")
	}
	window := playback.WindowRect{}
	// mpv reopens where the user parked it last time; only a fresh install or
	// a missing record falls back to the application window rectangle.
	if a.mpvRectOK && rectOnScreen(a.mpvRect) {
		window = playback.WindowRect{
			X:      a.mpvRect.X,
			Y:      a.mpvRect.Y,
			Width:  a.mpvRect.Width,
			Height: a.mpvRect.Height,
		}
	} else if rect, ok := appWindowRect(); ok && rectOnScreen(rect) {
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
		notified := false
		notifyStart := func(err error) {
			if notified || onStart == nil {
				return
			}
			notified = true
			onStart(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), playbackStartTimeout)
		defer cancel()
		// Playback start must not sit in a rate-limit pause: the row spinner
		// would look hung. Fail fast and let the header show the countdown.
		ctx = libsocial.FailFastOnRateLimit(ctx)
		if len(title.Voiceovers) == 0 || len(title.Episodes) == 0 {
			// History rows keep only the saved selection, never the episode
			// list or the voiceovers. Reload them from AnimeLIB so a direct
			// Play from history can start mpv without opening the title page
			// first; the saved voice and episode are restored during the load.
			loaded, loadErr := a.library.Load(ctx, title.Slug)
			if loadErr != nil {
				loadErr = playbackStartError(loadErr)
				notifyStart(loadErr)
				a.deliverWatch(watchResult{err: loadErr, historyFailure: true, party: partyGuest})
				return
			}
			title = loaded
			voice, episode = loaded.SelectedVoice, loaded.SelectedEpisode
			fresh := titleFromLibrary(loaded)
			historyItem = &fresh
		}
		plan, err := a.library.PlaybackPlan(ctx, title, voice, episode)
		if err != nil {
			err = playbackStartError(err)
			notifyStart(err)
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
			OnReady: func() {
				// The application window stays visible next to mpv: the title
				// page mirrors the playing episode and its progress live.
				if a.window != nil {
					a.window.Invalidate()
				}
			},
			OnFinish: func(err error) {
				if err != nil {
					err = errPlaybackFinished.wrap(err)
				}
				a.deliverWatch(watchResult{finished: true, err: err, historyItem: historyItem, titleID: trackedID, party: partyGuest})
			},
		})
		if err != nil {
			err = playbackStartError(err)
			notifyStart(err)
			a.deliverWatch(startFailureResult(err, historyItem, trackedID, partyGuest))
			return
		}
		if !partyGuest && a.party != nil {
			a.party.hostStarted(session, title, voice, episode)
		}
		notifyStart(nil)
	}()
	return nil
}

// playbackStartError turns the watchdog deadline into a reason the user can
// act on instead of the raw context error.
func playbackStartError(err error) error {
	if err == nil || isUserError(err) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return userMessage(fmt.Sprintf("Не удалось начать воспроизведение за %s — попробуйте ещё раз", playbackStartTimeout))
	}
	if errors.Is(err, libsocial.ErrRateLimited) {
		if remaining := libsocial.RateLimitRemaining(); remaining > 0 {
			return userMessage(fmt.Sprintf("AnimeLIB временно ограничил запросы — попробуйте снова через %s", rateLimitWaitLabel(remaining)))
		}
		return userMessage("AnimeLIB временно ограничил запросы — попробуйте снова чуть позже.")
	}
	return errPlaybackStart.wrap(err)
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
	if historyItem != nil && errors.Is(err, libsocial.ErrRateLimited) {
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
				a.main.loadError = userErrorText(result.err)
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
				a.main.loadError = userErrorText(result.err)
			}
			a.reportError(result.err)
			// A direct Play from history must leave the user on the title page
			// when playback cannot start (for example, the selected voice no
			// longer has any episodes). No mpv window is left behind.
			a.openTitle(*result.historyItem)
			if title, ok := a.active.(*titleScreen); ok {
				title.setWatchError(userErrorText(result.err))
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
				a.main.loadError = userErrorText(result.err)
				a.reportError(result.err)
			}
		}
		a.openTitle(updated)
		return
	}

	title, ok := a.active.(*titleScreen)
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
		title.setWatchError(userErrorText(result.err))
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
	title, ok := a.active.(*titleScreen)
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
	// The event loop already receives time-pos updates; querying mpv from the
	// layout callback would block the whole window if the player is busy. The
	// cached value is accurate to the last time-pos event, which is enough for
	// a once-a-second progress repaint.
	title.applyProgress(number, session.Position(), session.Duration())
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second)})
}

// persistWindowRects remembers where the user keeps the application window and
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
	a.shortcuts = newShortcutsScreen(a.shell.typo, shortcutInfoFor(a.active))
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

func shortcutInfoFor(screen Screen) shortcutInfo {
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

func (a *application) clearError() {
	a.shell.clearError()
}

// setUserStatus persists a status chosen from a pill menu and updates every
// screen that may be showing the same title.
func (a *application) setUserStatus(item historyTitle, status library.UserStatus) error {
	logger.Info("user status changed", "title", item.id, "status", status)
	if item.id != 0 && a.library != nil {
		if err := a.library.SetUserStatus(item.id, status); err != nil {
			err = errUserStatus.wrap(err)
			a.reportError(err)
			return err
		}
	}
	if a.main != nil {
		a.main.setUserStatus(item.id, status)
	}
	if title, ok := a.active.(*titleScreen); ok && title.item.id == item.id {
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
	screen := newLogScreen(a.shell.typo)
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

// frame renders one window frame. The defer is deliberately at this level: a
// panic from a screen, shell or event-processing helper must not unwind the Gio
// run loop and terminate the process. The recovered frame still calls
// e.Frame in run so Gio receives a valid frame.
func (a *application) frame(gtx layout.Context, w *app.Window) {
	defer func() {
		if value := recover(); value != nil {
			a.recoverPanic(value)
		}
	}()
	a.processUpdateCard()
	a.processPartyState()
	a.processWatchResults(gtx)
	a.processQualityResults(gtx)
	a.updatePlaybackNow(gtx)
	a.persistWindowRects(gtx.Now)
	a.syncFooterControls()
	a.handleGlobalKeys(gtx)
	a.shell.layout(gtx, w)
}

// recoverPanic records the panic at its own severity, tells the user that the
// action failed and returns to the previous screen. It is intentionally
// defensive: a panic in the fallback path must not cause a second panic loop.
func (a *application) recoverPanic(value any) {
	if a.recovering {
		return
	}
	a.recovering = true
	defer func() { a.recovering = false }()

	logger.Log(context.Background(), log.LevelPanic, "ui panic", "panic", fmt.Sprint(value), "stack", string(debug.Stack()))
	if a.shell != nil {
		a.shell.setError("Что-то пошло не так. Вернулись на предыдущий экран.", time.Now())
	}
	var fallback Screen = a.main
	if a.active != nil && a.active != a.main && a.previousScreen != nil {
		fallback = a.previousScreen
	}
	if fallback != nil && fallback != a.active && a.window != nil {
		a.showWindow(a.window, fallback)
	}
}

// drawFrame hands the completed ops to Gio's GPU renderer. The renderer can
// panic on an invalid texture (the old webtoon read path did exactly that), and
// that panic is on this goroutine, so it is recoverable here as well.
func (a *application) drawFrame(gtx layout.Context, e app.FrameEvent) {
	defer func() {
		if value := recover(); value != nil {
			a.recoverPanic(value)
		}
	}()
	e.Frame(gtx.Ops)
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
			a.frame(gtx, w)
			a.drawFrame(gtx, e)
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

var logger = log.For(log.SourceUI)
