package root

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/arfush/yo/core/log"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/resource"
	"github.com/arfush/yo/ui/screen"
	"github.com/arfush/yo/ui/shell"
	"github.com/arfush/yo/version"
)

const (
	windowTitle     = "yo"
	defaultLanguage = "ru"
)

// root owns the application object graph and the window lifecycle.
type root struct {
	window      *app.Window
	resources   *resource.Resource
	appearance  *stdx.Holder[appearance.Appearance]
	shell       *shell.Shell
	navigator   *navigator.Navigator
	journal     *log.Journal
	logger      *log.Logger
	windowShown bool
	stopOnce    sync.Once
}

func (r *root) initResource() {
	r.resources = resource.New()
}

func (r *root) initAppearance() {
	r.appearance = stdx.NewHolder(appearance.New(appearance.Options{
		Resources: r.resources,
		Language:  defaultLanguage,
	}))
	r.logger.Info("appearance ready", "language", defaultLanguage)
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

// initShell builds the window chrome and installs the fallback screen. The
// navigator replaces it with the startup screen before the first frame.
func (r *root) initShell() {
	r.shell = shell.New(shell.Options{
		Screen:     screen.NewStub(windowTitle),
		Window:     r.window,
		Appearance: r.appearance.Value(),
		Resources:  r.resources,
		Title:      windowTitle,
		Version:    version.Ver,
	})
	r.logger.Info("shell ready")
}

// initNavigator installs the screen and dialog stacks above the shell. The
// startup screen and the fallback dialog are the only destinations until the
// first feature registers its own factories.
func (r *root) initNavigator() {
	r.navigator = navigator.New(navigator.Options{
		Installer:  r.shell,
		Invalidate: r.window.Invalidate,
		Startup:    r.newStubScreen,
		StubDialog: r.newStubDialog,
	})
	r.logger.Info("navigator ready")
}

// initLog creates the process journal and the composition root logger. A
// journal file failure does not abort startup: console and the ring still work.
func (r *root) initLog() {
	opts := log.Options{Stdout: os.Stdout, Stderr: os.Stderr}
	path, pathErr := journalPath()
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
		r.logger.Warn("journal file disabled", "path", path, "error", openErr)
	default:
		r.logger.Info("journal opened", "path", path)
	}
}

// stop shuts the application down. It may be called from the window event loop
// and from the interrupt handler, so it must be idempotent. Future teardown
// belongs here as well.
func (r *root) stop() {
	r.stopOnce.Do(func() {
		if r.navigator != nil {
			r.navigator.Close()
		}
		r.logger.Info("application stopped")
		if err := r.journal.Close(); err != nil {
			r.logger.Error("close journal", "error", err)
		}
	})
}

// journalPath resolves the journal location next to the future database:
// %APPDATA%\yo\yo.log on Windows.
func journalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dir = filepath.Join(dir, "yo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create journal directory: %w", err)
	}
	return filepath.Join(dir, "yo.log"), nil
}
