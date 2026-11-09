package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName  = "yo"
	journalName = "yo.log"
)

// JournalPath returns the per-user journal file path and creates its directory.
// The directory follows the OS convention for user configuration, so the
// journal sits next to the future application data on every platform. Only
// concrete paths are exported; there is no generic AppData accessor.
func JournalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dir = filepath.Join(dir, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create journal directory: %w", err)
	}
	return filepath.Join(dir, journalName), nil
}
