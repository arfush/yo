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
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yonime/internal/animelib"
	"github.com/arfush/yonime/internal/appdata"
	"github.com/arfush/yonime/internal/browser"
	"github.com/arfush/yonime/internal/library"
	"github.com/arfush/yonime/internal/mpv"
	"github.com/arfush/yonime/internal/playback"
	"github.com/arfush/yonime/internal/token"
	"github.com/arfush/yonime/internal/updater"
	"github.com/arfush/yonime/internal/updates"
	"github.com/arfush/yonime/internal/version"
	"github.com/arfush/yonime/internal/watchparty"
)

const (
	// episodeCheckInterval is deliberately slow: the checker walks the whole
	// saved library and every title costs three AnimeLIB requests.
	episodeCheckInterval = 10 * time.Minute
	// episodeCheckStartupDelay lets the window, token check and update check
	// settle before the first background pass starts spending the shared
	// AnimeLIB rate-limit window.
	episodeCheckStartupDelay = 10 * time.Second
)

// Run starts the Yonime desktop application.
func Run() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Yonime"),
			app.Size(unit.Dp(initialWidth), unit.Dp(initialHeight)),
			app.MinSize(unit.Dp(minimumWidth), unit.Dp(minimumHeight)),
			app.Decorated(false),
		)

		a, err := newApp()
		if err != nil {
			fmt.Fprintln(os.Stderr, "yonime:", err)
			os.Exit(1)
		}
		a.attachWindow(w)
		if err := a.run(w); err != nil {
			a.close()
			fmt.Fprintln(os.Stderr, "yonime:", err)
			os.Exit(1)
		}
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
	active         windowScreen
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

	// Update state is loaded from %APPDATA%\yonime\updates.json. The atomics
	// let the UI read it every frame while the background checker refreshes it.
	updateStore      *updates.Store
	updateClient     *http.Client
	downloadClient   *http.Client
	updatesAvailable atomic.Pointer[updates.Release]
	updateChecking   atomic.Bool
	updateWake       chan struct{}
	// currentSHA is the hash of the running executable. It lets a re-pushed
	// version with the same number still install its fixed build.
	currentSHA string
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
			fmt.Fprintln(os.Stderr, "yonime: хеш приложения:", hashErr)
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
		currentSHA:        currentSHA,
	}
	state := updateStore.State()
	app.updatesAvailable.Store(state.Available)
	app.party = newPartyManager(app)
	return app, nil
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
	a.shell.onUpdate = a.openUpdate
	a.shell.onSettings = a.openSettings
	a.shell.onParty = a.openWatchParty
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
		fmt.Fprintln(os.Stderr, "yonime: значок в трее:", err)
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
	a.hidden.Store(true)
	go hideAppWindow()
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
	a.showWindow(w, preparation)
}

func (a *application) showMain(w *app.Window) {
	a.showWindow(w, a.main)
	w.Invalidate()
}

func (a *application) openUpdate() {
	release := a.updatesAvailable.Load()
	if !a.updateAvailable(release) {
		return
	}
	if a.window == nil {
		return
	}
	update := *release
	screen := newFirstRun(
		a.shell.typo,
		a.window,
		"Обновление Yonime",
		fmt.Sprintf("Скачиваем %s. Приложение перезапустится автоматически.", update.Version),
		func(ctx context.Context, report func(downloadProgress)) error {
			return a.downloadAndApply(ctx, update, report)
		},
	)
	screen.buttonText = "Скачать"
	screen.retryText = "Повторить"
	screen.errorText = "Не удалось обновить. Попробуй ещё раз."
	screen.autoStart = true
	screen.onDone = a.quitForUpdate
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
	settings := newSettingsWindow(a.shell.typo, settingsValues{
		Notifications:  a.library.EpisodeNotificationsEnabled(),
		SharedControl:  a.library.SharedControlEnabled(),
		Provider:       a.party.Provider(),
		ProviderChoice: a.party.HasProviderChoice(),
	})
	settings.hidden = a.windowHidden
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
	settings.onControl = func(enabled bool) error {
		if err := a.library.SetSharedControlEnabled(enabled); err != nil {
			return err
		}
		a.party.SetSharedControl(enabled)
		return nil
	}
	settings.onProvider = func(provider watchparty.Provider) error {
		if !a.party.HasProviderChoice() {
			return nil
		}
		// SetProvider stores the choice, so the next room and the party card
		// agree with the settings.
		a.party.SetProvider(provider)
		return nil
	}
	a.showWindow(a.window, settings)
}

// openWatchParty shows the room card. When a room is still open the screen
// reopens on the created (host) or waiting (guest) form with the same key.
func (a *application) openWatchParty() {
	if a.window == nil {
		return
	}
	if a.party == nil {
		a.party = newPartyManager(a)
	}
	screen := newWatchPartyWindow(a.shell.typo, a.window, a.party)
	screen.hidden = a.windowHidden
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
func (a *application) startPartyPlayback(state watchparty.State) error {
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
	report(downloadProgress{Preparing: true})
	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить файл приложения: %w", err)
	}
	if err := updater.CheckTarget(target); err != nil {
		return err
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return err
	}
	name := filepath.Base(release.AssetName)
	if name == "." || name == "" {
		name = updates.AssetName(release.Version, runtime.GOARCH)
	}
	destination := filepath.Join(updateDir, name)
	source, err := updates.Download(ctx, a.downloadClient, release, destination, func(downloaded, total int64) {
		percent := 0.0
		if total > 0 {
			percent = float64(downloaded) * 100 / float64(total)
		}
		report(downloadProgress{Percent: percent})
	})
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
	report(downloadProgress{Percent: 100})
	return plan.Launch()
}

func (a *application) quitForUpdate() {
	a.quitting.Store(true)
	if a.window != nil {
		a.window.Perform(system.ActionClose)
	}
}

func (a *application) startUpdateLoop(w *app.Window) {
	go func() {
		a.checkForUpdates(w)
		ticker := time.NewTicker(4 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.checkForUpdates(w)
			case <-a.updateWake:
				a.checkForUpdates(w)
			}
		}
	}()
}

func (a *application) triggerUpdateCheck() {
	select {
	case a.updateWake <- struct{}{}:
	default:
	}
}

func (a *application) checkForUpdates(w *app.Window) {
	if updates.Repository == "" {
		return
	}
	if !a.updateChecking.CompareAndSwap(false, true) {
		return
	}
	defer a.updateChecking.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	release, err := updates.FetchRelease(ctx, a.updateClient)
	now := time.Now()
	if err != nil {
		if errors.Is(err, updates.ErrNoRelease) {
			release = nil
			err = nil
		} else {
			// A network failure must not hide the release cached from the last
			// successful check.
			fmt.Fprintln(os.Stderr, "yonime: проверка обновлений:", err)
			_ = a.updateStore.SaveLastCheck(now)
			return
		}
	}
	if saveErr := a.updateStore.RecordCheck(release, now); saveErr != nil {
		fmt.Fprintln(os.Stderr, "yonime: сохранение состояния обновлений:", saveErr)
	}
	a.updatesAvailable.Store(release)
	if w != nil {
		w.Invalidate()
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
		fmt.Fprintln(os.Stderr, "yonime: подсчёт серий:", err)
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
		fmt.Fprintln(os.Stderr, "yonime: проверка новых серий:", err)
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
	a.shell.partyVisible = onMain
	a.shell.partyActive = onMain && a.party != nil && a.party.HasRoom()
	a.shell.updateVisible = onMain && a.updateAvailable(release)
}

// updateAvailable applies the same rule to the footer button and the update
// screen: newer versions always qualify, and a re-pushed version qualifies when
// its published build differs from the running executable.
func (a *application) updateAvailable(release *updates.Release) bool {
	return updates.IsUpdate(release, version.Version, a.currentSHA)
}

func (a *application) openTitle(item historyTitle) {
	if a.window == nil {
		return
	}
	preferredQuality := 1080
	if a.library != nil {
		preferredQuality = a.library.PreferredQuality()
	}
	title := newTitleWindow(a.shell.typo, item, preferredQuality)
	title.hidden = a.windowHidden
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
		return nil
	}
	title.onQuality = func(index int) error {
		if a.library == nil {
			return nil
		}
		if index < 0 || index >= len(titleQualityValues) {
			return nil
		}
		return a.library.SetPreferredQuality(titleQualityValues[index])
	}
	title.onWatch = func(voice, episode string, quality int) error {
		if a.library == nil {
			return fmt.Errorf("база данных недоступна")
		}
		return a.startPlayback(libraryTitleFromHistory(title.item), voice, episode, quality, nil, false, 0)
	}
	a.showWindow(a.window, title)
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
	if rect, ok := appWindowRect(); ok {
		window = playback.WindowRect{
			X:      rect.X,
			Y:      rect.Y,
			Width:  rect.Width,
			Height: rect.Height,
		}
	}
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if len(title.Voiceovers) == 0 || len(title.Episodes) == 0 {
			// History rows keep only the saved selection, never the episode
			// list or the voiceovers. Reload them from AnimeLIB so a direct
			// Play from history can start mpv without opening the title page
			// first; the saved voice and episode are restored during the load.
			loaded, loadErr := a.library.Load(ctx, title.Slug)
			if loadErr != nil {
				a.deliverWatch(watchResult{err: loadErr, historyFailure: true, party: partyGuest})
				return
			}
			title = loaded
			voice, episode = loaded.SelectedVoice, loaded.SelectedEpisode
			fresh := titleFromLibrary(loaded)
			historyItem = &fresh
		}
		// Watching a title is enough to keep its progress. A link-opened title
		// has no history row until Save, and a party guest has to keep the
		// title and voice they watched together, so both get tracked before
		// mpv starts writing positions.
		trackedID := int64(0)
		if id, trackErr := a.library.TrackViewing(title, voice, episode); trackErr == nil && id != 0 {
			trackedID = id
			title.ID = id
		}
		plan, err := a.library.PlaybackPlan(ctx, title, voice, episode)
		if err != nil {
			a.deliverWatch(watchResult{err: err, historyItem: historyItem, titleID: trackedID, party: partyGuest})
			return
		}
		if partyGuest {
			// The room owns the position: a guest must not resume from its own
			// history, which would start the file in the wrong place.
			plan.ResumeAt = startAt
		}
		session, err := a.playback.Start(ctx, plan, playback.StartOptions{
			Quality:          quality,
			Window:           window,
			HeaderFonts:      headerFonts(),
			HeaderBadgeWidth: a.shell.badgeWidth,
			OnReady: func() {
				a.hidden.Store(true)
				hideAppWindow()
				if a.window != nil {
					a.window.Invalidate()
				}
			},
			OnFinish: func(err error) {
				a.hidden.Store(false)
				showAppWindow()
				a.deliverWatch(watchResult{finished: true, err: err, historyItem: historyItem, titleID: trackedID, party: partyGuest})
			},
		})
		if err != nil {
			a.deliverWatch(watchResult{err: err, historyItem: historyItem, titleID: trackedID, party: partyGuest})
			return
		}
		if !partyGuest && a.party != nil {
			a.party.hostStarted(session, title, voice, episode)
		}
	}()
	return nil
}

func (a *application) deliverWatch(result watchResult) {
	select {
	case a.watchResults <- result:
	case <-time.After(5 * time.Second):
	}
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
			}
		}
		a.openTitle(updated)
		return
	}

	title, ok := a.active.(*titleWindow)
	if !ok {
		return
	}
	if result.err != nil {
		title.setWatchError(result.err.Error())
	} else if result.finished {
		title.setWatching(false)
	}
	if !result.finished {
		return
	}
	if result.titleID != 0 && title.item.id == 0 {
		// Playback tracked a link-opened title in history; adopt the row so the
		// progress refresh and a later Save use the same entry.
		title.item.id = result.titleID
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

func (a *application) showWindow(w *app.Window, screen windowScreen) {
	if a.active != nil && a.active != screen {
		a.active.Close()
	}
	a.active = screen
	a.shell.footer = nil
	if fs, ok := screen.(footerScreen); ok {
		a.shell.footer = func(gtx layout.Context) layout.Dimensions {
			fs.layoutFooter(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	if screen == a.main {
		a.main.onShown()
		a.shell.overlay = a.main.layoutOverlay
		a.shell.onPress = a.main.noteWindowPress
	} else {
		a.shell.overlay = nil
		a.shell.onPress = nil
	}
	a.shell.content = func(gtx layout.Context) layout.Dimensions {
		screen.layout(gtx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	w.Invalidate()
}

func (a *application) close() {
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
			a.processWatchResults(gtx)
			a.syncFooterControls()
			a.shell.layout(gtx, w)
			e.Frame(gtx.Ops)
		case app.ConfigEvent:
			wasFocused := a.shell.cfg.Focused
			a.shell.setConfig(e.Config)
			if wasFocused && !e.Config.Focused && a.active == a.main {
				a.main.clearSelection()
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
