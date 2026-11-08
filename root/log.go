package root

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arfush/yo/core/log"
)

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

func (r *root) closeLog() {
	if err := r.journal.Close(); err != nil {
		r.logger.Error("close journal", "error", err)
	}
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
