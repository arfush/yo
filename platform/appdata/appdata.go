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

const (
	databaseName      = "yoanime.sqlite"
	tokenName         = "libsocial.token"
	profileName       = "browser"
	coversName        = "covers"
	mpvName           = "mpv"
	updatesName       = "updates.json"
	updateDirName     = "update"
	updateLogName     = "update.log"
	logName           = "yoanime.log"
	partyName         = "party.json"
	filmlibName       = "filmlib.json"
	legacyFilmlibName = "kinopoisk.json"
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

// ensureDir creates the data directory under base.
func ensureDir(base string) (string, error) {
	dir := filepath.Join(base, dirName)
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

// TokenPath returns the LibSocial token path inside the data directory.
func TokenPath() (string, error) {
	return path(tokenName)
}

// PartyConfigPath returns the relay configuration the user edits after
// deploying the Worker or the Supabase project.
func PartyConfigPath() (string, error) {
	return path(partyName)
}

// FilmlibConfigPath returns the optional mirror-host override. The file is
// only needed when a mirror moves before the application ships an update.
func FilmlibConfigPath() (string, error) {
	return path(filmlibName)
}

// LegacyFilmlibConfigPath is the old file name kept readable for forks that
// already created an override before the source was renamed.
func LegacyFilmlibConfigPath() (string, error) {
	return path(legacyFilmlibName)
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
