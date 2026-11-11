package root

import (
	"os"
	"sync"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/log"
	"github.com/arfush/yo/core/platform"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
	"github.com/arfush/yo/ui/shell"
	"github.com/arfush/yo/version"
)

const (
	windowTitle     = "yo"
	defaultLanguage = "ru"

	defaultPanicTitle = "Oops.. Something went wrong!"
)

// root owns the application object graph and the window lifecycle.
type root struct {
	window         *app.Window
	platformWindow *platform.Window
	resource       *resource.Resource
	appearance     *stdx.Holder[appearance.Appearance]
	site           *stdx.Holder[catalog.Site]
	shell          *shell.Shell
	navigator      *navigator.Navigator
	journal        *log.Journal
	logger         *log.Logger
	platformLog    *log.Logger
	windowShown    bool
	panicked       bool
	iconKey        resource.AppIconKey
	stopOnce       sync.Once
}

func (r *root) initResource() {
	r.resource = resource.New()
	r.logger.Info("resources ready")
}

func (r *root) initAppearance() {
	r.appearance = stdx.NewHolder(appearance.New(appearance.Options{
		Resource: r.resource,
		Language: defaultLanguage,
	}))
	r.logger.Info("appearance ready", "language", defaultLanguage)
}

func (r *root) initSite() {
	r.site = stdx.NewHolder(catalog.SiteYoanime)
	r.logger.Info("site ready", "site", catalog.SiteYoanime.String())
}

func (r *root) initWindowOptions() {
	r.window.Option(
		app.Title(windowTitle),
		app.Size(unit.Dp(shell.InitialWidth), unit.Dp(shell.InitialHeight)),
		app.MinSize(unit.Dp(shell.MinimumWidth), unit.Dp(shell.MinimumHeight)),
		app.Decorated(false),
	)
	r.logger.Info("window configured", "title", windowTitle)
}

// initShell builds the window chrome and installs the temporary main screen
// placeholder. The navigator installs the same screen before the first frame.
func (r *root) initShell() {
	r.shell = shell.New(shell.Options{
		Screen:     r.newStartupScreen(),
		Window:     r.window,
		Cursor:     r.platformWindow,
		Site:       r.site,
		Appearance: r.appearance,
		Resource:   r.resource,
		Version:    "v" + version.Ver,
		Logger:     r.journal.Logger(log.SourceShell),
	})
	r.logger.Info("shell ready")
}

// initNavigator installs the screen stack above the shell. The temporary main
// screen placeholder is the only screen until the first feature registers its
// own factory.
func (r *root) initNavigator() {
	r.navigator = navigator.New(navigator.Options{
		Installer:  r.shell,
		Invalidate: r.window.Invalidate,
		Startup:    r.newStartupScreen,
		Logger:     r.journal.Logger(log.SourceNavigator),
	})
	r.logger.Info("navigator ready")
}

// initLog creates the process journal and the composition root logger. A
// journal file failure does not abort startup: console and the ring still work.
func (r *root) initLog() {
	opts := log.Options{Stdout: os.Stdout, Stderr: os.Stderr}
	path, pathErr := platform.JournalPath()
	if pathErr == nil {
		opts.Path = path
	}
	journal, openErr := log.Open(opts)
	r.journal = journal
	r.logger = journal.Logger(log.SourceRoot)
	r.platformLog = journal.Logger(log.SourcePlatform)
	r.platformWindow = platform.NewWindow(r.platformLog)
	switch {
	case pathErr != nil:
		r.logger.Warn("journal path unavailable", "error", pathErr)
	case openErr != nil:
		r.logger.Warn("journal file disabled", "error", openErr)
	default:
		r.logger.Info("journal opened", "path", path)
	}
}

// stop shuts the application down. It runs on the UI goroutine; the interrupt
// handler only requests a window close. It is idempotent because the destroy
// path and the deferred call can both reach it. Future teardown belongs here.
func (r *root) stop() {
	r.stopOnce.Do(func() {
		if r.platformWindow != nil {
			r.platformWindow.Close()
		}
		if r.navigator != nil {
			r.navigator.Close()
		}
		if err := r.journal.Close(); err != nil {
			r.logger.Error("close journal", "error", err)
		} else {
			r.logger.Info("journal closed")
		}
		r.logger.Info("application stopped")
	})
}
