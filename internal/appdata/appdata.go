// Package appdata resolves the per-user directory where yoanime keeps its
// database, token and browser profile.
//
// On Windows that directory is %APPDATA%\yoanime. Keeping user data out of the
// executable directory means an application update or a read-only install
// location cannot lose the library.
package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const dirName = "yoanime"

// legacyDirName is the lower-case directory used by releases before the
// app-data rename. On a case-insensitive file system it is the same directory;
// on a case-sensitive one it is moved to the new name on first launch.
const legacyDirName = "yoanime"

const (
	databaseName  = "yoanime.sqlite"
	tokenName     = "animelib_token.txt"
	profileName   = "browser"
	coversName    = "covers"
	mpvName       = "mpv"
	updatesName   = "updates.json"
	updateDirName = "update"
	updateLogName = "update.log"
	logName       = "yoanime.log"
	// partyName is the relay configuration file. Releases before the party
	// rename stored it as legacyPartyName; PartyConfigPath moves that file once
	// so an existing override keeps working.
	partyName       = "party.json"
	legacyPartyName = "watchparty.json"
)

// Dir returns the per-user data directory and creates it when necessary.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(base) == "" {
		// UserConfigDir is implemented on every supported platform, but a
		// missing home directory should still produce a usable path.
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("locate user configuration directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}

	dir, err := ensureDir(base)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ensureDir creates the data directory under base and normalizes the casing of
// the old lower-case name. Windows treats both names as the same directory, so
// a plain MkdirAll would leave the old spelling in Explorer and in paths shown
// to the user.
func ensureDir(base string) (string, error) {
	dir := filepath.Join(base, dirName)
	legacy := filepath.Join(base, legacyDirName)

	if entries, err := os.ReadDir(base); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == dirName || !strings.EqualFold(entry.Name(), dirName) {
				continue
			}
			// Rename through a temporary name because Windows refuses a
			// case-only rename directly.
			oldPath := filepath.Join(base, entry.Name())
			temp := filepath.Join(base, dirName+".rename")
			if err := os.Rename(oldPath, temp); err == nil {
				if err := os.Rename(temp, dir); err != nil {
					_ = os.Rename(temp, oldPath)
				}
			}
			break
		}
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			if renameErr := os.Rename(legacy, dir); renameErr != nil {
				return "", fmt.Errorf("rename %s to %s: %w", legacy, dir, renameErr)
			}
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}

// MPVDir returns the directory where yoanime keeps its own mpv build. The
// directory is not created until a download starts.
func MPVDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, mpvName), nil
}

// MPVPath returns the path of the mpv executable managed by yoanime.
func MPVPath() (string, error) {
	dir, err := MPVDir()
	if err != nil {
		return "", err
	}
	name := "mpv"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name), nil
}

// UpdatesPath returns the JSON file that remembers the last release check and
// the release cached for the footer button.
func UpdatesPath() (string, error) {
	return path(updatesName)
}

// UpdateDir returns the directory used for downloaded releases and the helper
// that replaces the executable. It is created on demand.
func UpdateDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	updateDir := filepath.Join(dir, updateDirName)
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", updateDir, err)
	}
	return updateDir, nil
}

// UpdateLogPath returns the file the hidden update helper appends errors to.
func UpdateLogPath() (string, error) {
	dir, err := UpdateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, updateLogName), nil
}

// LogPath returns the application log file inside the data directory.
func LogPath() (string, error) {
	return path(logName)
}

// DatabasePath returns the SQLite database path inside the data directory.
func DatabasePath() (string, error) {
	return path(databaseName)
}

// TokenPath returns the AnimeLIB token path inside the data directory.
func TokenPath() (string, error) {
	return path(tokenName)
}

// PartyConfigPath returns the relay configuration the user edits after
// deploying the Worker or the Supabase project.
func PartyConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return migratePartyConfig(dir), nil
}

// migratePartyConfig renames the pre-rename watchparty.json once. If the
// rename fails, the legacy path is returned instead of silently dropping the
// user's relay override; the next launch retries the migration.
func migratePartyConfig(dir string) string {
	current := filepath.Join(dir, partyName)
	if _, err := os.Stat(current); err == nil {
		return current
	} else if !os.IsNotExist(err) {
		return current
	}
	legacy := filepath.Join(dir, legacyPartyName)
	if _, err := os.Stat(legacy); err != nil {
		return current
	}
	if err := os.Rename(legacy, current); err != nil {
		return legacy
	}
	return current
}

// BrowserProfileDir returns the persistent Chromium profile path. The profile
// keeps the AnimeLIB session between token renewals.
func BrowserProfileDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	profile := filepath.Join(dir, profileName)
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", profile, err)
	}
	return profile, nil
}

func path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
