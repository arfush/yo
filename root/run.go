package root

import (
	"os"
	"os/signal"
	"runtime"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/version"
)

// Run starts the application and blocks until the window is closed.
func Run() {
	qt.NewQApplication([]string{os.Args[0]})
	r := &root{}
	r.initLog()
	defer r.stop()

	r.logger.Info("application started",
		"version", version.Ver, "os", runtime.GOOS, "arch", runtime.GOARCH)
	r.initResource()
	r.initAppearance()
	r.initSite()
	r.initShell()
	r.initNavigator()
	r.startStubAnimation()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-interrupt:
			r.logger.Info("interrupt received")
			qt.QCoreApplication_Quit()
		case <-done:
		}
	}()

	r.shell.Show()
	qt.QApplication_Exec()
	r.logger.Info("window closed")
}
