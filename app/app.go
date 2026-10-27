// Package app is the composition root. It owns process startup, opens the
// local database and browser profile, builds the service graph and hands it to
// the UI. Screens never construct these services themselves.
package app

import (
	"fmt"
	"os"

	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/log"
	"github.com/arfush/yo/mpv"
	"github.com/arfush/yo/platform/appdata"
	"github.com/arfush/yo/platform/install"
	"github.com/arfush/yo/platform/singleinstance"
	"github.com/arfush/yo/platform/tray"
	"github.com/arfush/yo/playback"
	"github.com/arfush/yo/ui"
	"github.com/arfush/yo/update"
	"github.com/arfush/yo/version"
)

// Run starts the desktop application. It never returns while the window loop
// is alive; a second launch hands focus to the running copy and exits.
func Run() {
	if logPath, err := appdata.LogPath(); err == nil {
		if err := log.Init(logPath); err != nil {
			fmt.Fprintln(os.Stderr, "yoanime: cannot open log:", err)
		}
		defer log.Close()
	}
	logger.Info("application starting", "version", version.Version)

	release, ok := singleinstance.Acquire()
	if !ok {
		// Another copy already owns the window, database and browser profile.
		// Hand the focus over so it can come back from the notification area.
		logger.Info("another instance is already running")
		tray.ActivateRunningInstance()
		return
	}
	defer release()

	// A self-update replaces the exe inside the install directory. Refresh the
	// Add/Remove Programs entry so it does not keep advertising the old version.
	install.SyncDisplayVersion(version.Version)

	services, err := buildServices()
	if err != nil {
		logger.Error("application startup failed", "error", err)
		fmt.Fprintln(os.Stderr, "yoanime:", err)
		os.Exit(1)
	}
	ui.Run(services)
}

func buildServices() (ui.Services, error) {
	databasePath, err := appdata.DatabasePath()
	if err != nil {
		return ui.Services{}, err
	}
	tokenPath, err := appdata.TokenPath()
	if err != nil {
		return ui.Services{}, err
	}
	profileDir, err := appdata.BrowserProfileDir()
	if err != nil {
		return ui.Services{}, err
	}
	updatesPath, err := appdata.UpdatesPath()
	if err != nil {
		return ui.Services{}, err
	}
	updateStore, err := update.Open(updatesPath)
	if err != nil {
		return ui.Services{}, err
	}
	updateDir, err := appdata.UpdateDir()
	if err != nil {
		return ui.Services{}, err
	}
	update.Cleanup(updateDir)

	// The executable path is hashed once so a re-pushed build with an unchanged
	// version can still be offered.
	currentSHA := ""
	if executable, exeErr := os.Executable(); exeErr == nil {
		if digest, hashErr := update.FileSHA256(executable); hashErr == nil {
			currentSHA = digest
		} else {
			logger.Warn("cannot hash the executable", "error", hashErr)
		}
	}

	lib, err := library.Open(databasePath, tokenPath)
	if err != nil {
		return ui.Services{}, fmt.Errorf("open database: %w", err)
	}
	history, err := lib.History()
	if err != nil {
		lib.Close()
		return ui.Services{}, fmt.Errorf("load history: %w", err)
	}
	browserManager := browser.New()
	return ui.Services{
		Library:           lib,
		Browser:           browserManager,
		MPV:               mpv.New(),
		Playback:          playback.New(lib, browserManager),
		UpdateStore:       updateStore,
		RateLimit:         animeLibRateLimit{},
		History:           history,
		TokenPath:         tokenPath,
		BrowserProfileDir: profileDir,
		CurrentSHA:        currentSHA,
	}, nil
}

var logger = log.For(log.SourceApp)
