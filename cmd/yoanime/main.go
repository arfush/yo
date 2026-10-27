package main

import (
	"fmt"
	"os"

	"github.com/arfush/yoanime/internal/appdata"
	"github.com/arfush/yoanime/internal/install"
	"github.com/arfush/yoanime/internal/singleinstance"
	"github.com/arfush/yoanime/internal/version"
	"github.com/arfush/yoanime/log"
	"github.com/arfush/yoanime/ui"
)

func main() {
	if logPath, err := appdata.LogPath(); err == nil {
		if err := log.Init(logPath); err != nil {
			fmt.Fprintln(os.Stderr, "yoanime: cannot open log:", err)
		}
		defer log.Close()
	}
	log.Info("application starting", "version", version.Version)

	release, ok := singleinstance.Acquire()
	if !ok {
		// Another copy already owns the window, database and browser profile.
		// Hand the focus over so it can come back from the notification area.
		log.Info("another instance is already running")
		ui.ActivateRunningInstance()
		return
	}
	defer release()
	// A self-update replaces the exe inside the install directory. Refresh the
	// Add/Remove Programs entry so it does not keep advertising the old version.
	install.SyncDisplayVersion(version.Version)
	ui.Run()
}
