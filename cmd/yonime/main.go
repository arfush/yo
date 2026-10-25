package main

import (
	"github.com/arfush/yonime/internal/install"
	"github.com/arfush/yonime/internal/singleinstance"
	"github.com/arfush/yonime/internal/version"
	"github.com/arfush/yonime/ui"
)

func main() {
	release, ok := singleinstance.Acquire()
	if !ok {
		// Another copy already owns the window, database and browser profile.
		// Hand the focus over so it can come back from the notification area.
		ui.ActivateRunningInstance()
		return
	}
	defer release()
	// A self-update replaces the exe inside the install directory. Refresh the
	// Add/Remove Programs entry so it does not keep advertising the old version.
	install.SyncDisplayVersion(version.Version)
	ui.Run()
}
