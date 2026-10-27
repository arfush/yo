package mpv

import "errors"

// Sentinels returned by the mpv manager and installer.
var (
	// ErrNotFound means mpv.exe was not found in the expected directory.
	ErrNotFound = errors.New("mpv не найден")
	// ErrManagerNil means the manager was not opened before use.
	ErrManagerNil = errors.New("mpv manager is nil")
	// ErrDownloadRunning means a download is already in flight.
	ErrDownloadRunning = errors.New("mpv download is already running")
	// ErrNotInstalled means the archive extraction did not leave mpv.exe
	// where the manager expects it.
	ErrNotInstalled = errors.New("mpv.exe was not installed")
	// ErrArchiveMissingExecutable means the 7z archive had no mpv.exe inside.
	ErrArchiveMissingExecutable = errors.New("mpv.exe not found in the downloaded archive")
)
