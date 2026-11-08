package root

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app"

	"github.com/arfush/yo/core/log"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
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
	r.window.Option(app.Title(windowTitle))
	r.logger.Info("window configured", "title", windowTitle)
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
