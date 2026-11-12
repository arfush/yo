package root

import (
	"os"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/log"
	"github.com/arfush/yo/core/platform"
	"github.com/arfush/yo/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
	"github.com/arfush/yo/ui/shell"
	"github.com/arfush/yo/version"
)

const (
	windowTitle     = "yo"
	defaultLanguage = "ru"
)

// root owns the application object graph and the window lifecycle.
type root struct {
	resource   *resource.Resource
	appearance *stdx.Holder[appearance.Appearance]
	site       *stdx.Holder[catalog.Site]
	shell      *shell.Shell
	navigator  *navigator.Navigator
	journal    *log.Journal
	logger     *log.Logger
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

// initShell builds the window chrome and installs the temporary main screen
// placeholder. The navigator installs the same screen before the first paint.
func (r *root) initShell() {
	r.shell = shell.New(shell.Options{
		Screen:     r.newStartupScreen(),
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
		Invalidate: func() { r.shell.Widget().Update() },
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
	switch {
	case pathErr != nil:
		r.logger.Warn("journal path unavailable", "error", pathErr)
	case openErr != nil:
		r.logger.Warn("journal file disabled", "error", openErr)
	default:
		r.logger.Info("journal opened", "path", path)
	}
}

// stop shuts the application down after the Qt event loop returns. Future
// teardown belongs here.
func (r *root) stop() {
	if r.navigator != nil {
		r.navigator.Close()
	}
	if err := r.journal.Close(); err != nil {
		r.logger.Error("close journal", "error", err)
	} else {
		r.logger.Info("journal closed")
	}
	r.logger.Info("application stopped")
}
