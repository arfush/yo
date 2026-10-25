// Package mpv locates and installs the mpv executable used by Yonime.
package mpv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arfush/yonime/internal/appdata"
	"github.com/arfush/yonime/internal/logging"
)

// Progress reports the overall mpv installation progress. Preparing is true
// while the release metadata or archive is being resolved.
type Progress struct {
	Percent   float64
	Preparing bool
}

// Manager controls the mpv build kept in the per-user data directory.
type Manager struct {
	client *http.Client

	mu          sync.Mutex
	downloading bool
}

// New returns a manager. Nothing is downloaded until Download is called.
func New() *Manager {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &Manager{
		client: &http.Client{Transport: transport},
	}
}

// Installed reports whether an mpv executable can be found.
func (m *Manager) Installed() bool {
	_, err := Locate()
	return err == nil
}

// Locate returns the mpv executable. PATH wins; otherwise the build managed by
// Yonime in %APPDATA%\Yonime\mpv is used.
func Locate() (string, error) {
	if value := strings.TrimSpace(os.Getenv("YONIME_MPV")); value != "" {
		if info, err := os.Stat(value); err == nil && !info.IsDir() && info.Size() > 0 {
			if absolute, err := filepath.Abs(value); err == nil {
				return absolute, nil
			}
			return value, nil
		}
	}
	if path, err := exec.LookPath("mpv"); err == nil {
		return path, nil
	}
	if path, err := appdata.MPVPath(); err == nil {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() && info.Size() > 0 {
			return path, nil
		}
	}
	return "", errors.New("mpv не найден")
}

// Download installs mpv into %APPDATA%\Yonime\mpv. The callback is invoked
// from a background goroutine and must be safe to call repeatedly.
func (m *Manager) Download(ctx context.Context, onProgress func(Progress)) error {
	if m == nil {
		return errors.New("mpv manager is nil")
	}
	if onProgress == nil {
		onProgress = func(Progress) {}
	}

	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return errors.New("mpv download is already running")
	}
	m.downloading = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.mu.Unlock()
	}()

	onProgress(Progress{Preparing: true})
	if m.Installed() {
		logging.Info("mpv is already installed")
		onProgress(Progress{Percent: 100})
		return nil
	}
	logging.Info("mpv installation started")
	if err := ctx.Err(); err != nil {
		return err
	}

	dir, err := appdata.MPVDir()
	if err != nil {
		logging.Error("cannot resolve the mpv directory", "error", err)
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logging.Error("cannot create the mpv directory", "error", err)
		return fmt.Errorf("create mpv directory: %w", err)
	}

	release, err := resolveRelease(ctx, m.client)
	if err != nil {
		logging.Error("cannot resolve the mpv release", "error", err)
		return err
	}
	onProgress(Progress{Percent: 0})

	temp, err := os.CreateTemp(dir, ".mpv-*.7z")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}()

	err = downloadArchive(ctx, m.client, release, temp, func(downloaded, total int64) {
		percent := 0.0
		if total > 0 {
			percent = float64(downloaded) * 85 / float64(total)
			if percent > 85 {
				percent = 85
			}
		}
		onProgress(Progress{Percent: percent})
	})
	closeErr := temp.Close()
	if err != nil {
		logging.Error("cannot download the mpv archive", "error", err)
		return err
	}
	if closeErr != nil {
		logging.Error("cannot close the mpv archive", "error", closeErr)
		return closeErr
	}

	onProgress(Progress{Percent: 85})
	if err := extractArchive(ctx, tempName, dir, func(fraction float64) {
		percent := 85 + fraction*15
		if percent > 99 {
			percent = 99
		}
		onProgress(Progress{Percent: percent})
	}); err != nil {
		logging.Error("cannot extract the mpv archive", "error", err)
		return err
	}

	path, err := appdata.MPVPath()
	if err != nil {
		logging.Error("cannot resolve the mpv path", "error", err)
		return err
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
		logging.Error("mpv executable was not installed", "path", path, "error", err)
		return errors.New("mpv.exe was not installed")
	}
	logging.Info("mpv installation finished", "path", path)
	onProgress(Progress{Percent: 100})
	return nil
}
