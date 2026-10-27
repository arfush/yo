// Package updater replaces the running yoanime executable after a verified
// release has been downloaded.
//
// Windows keeps the image file of a running process locked, so the application
// cannot overwrite itself. Instead it starts a tiny hidden cmd.exe helper. The
// helper copies the verified download to a temporary .new file next to the
// target, then atomically renames it over the old executable once the process
// exits. The previous executable is replaced, so no permanent .old backup is
// left behind.
package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	helperName = "update-helper.cmd"
	logName    = "update.log"
)

// Plan describes a ready-to-run replacement.
type Plan struct {
	Target  string
	Source  string
	WorkDir string
	Script  string
	Log     string
}

// CheckTarget verifies that the application may replace target once the
// process exits. It deliberately does not open the running executable for
// writing: Windows rejects that even when the directory is writable.
func CheckTarget(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("не удалось определить путь к приложению")
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("не удалось определить путь к приложению: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("не удалось найти файл приложения: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("файл приложения не является обычным файлом")
	}
	if info.Mode().Perm()&0o200 == 0 {
		return fmt.Errorf("файл приложения защищён от записи")
	}

	directory := filepath.Dir(absolute)
	probe, err := os.CreateTemp(directory, ".yoanime-update-*")
	if err != nil {
		return fmt.Errorf("нет прав на запись в папку приложения: %w", err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// Prepare validates both files and returns the helper plan.
func Prepare(target, source, workDir, logPath string) (*Plan, error) {
	if err := CheckTarget(target); err != nil {
		return nil, err
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(target, source) {
		return nil, fmt.Errorf("новый файл совпадает с запущенным приложением")
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("не удалось найти скачанное обновление: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("скачанное обновление повреждено")
	}
	header := make([]byte, 2)
	file, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть скачанное обновление: %w", err)
	}
	_, readErr := file.Read(header)
	_ = file.Close()
	if readErr != nil || header[0] != 'M' || header[1] != 'Z' {
		return nil, fmt.Errorf("скачанный файл не является Windows-приложением")
	}

	if workDir == "" {
		workDir = filepath.Dir(source)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("не удалось создать папку обновления: %w", err)
	}
	if logPath == "" {
		logPath = filepath.Join(workDir, logName)
	}
	return &Plan{
		Target:  target,
		Source:  source,
		WorkDir: workDir,
		Script:  filepath.Join(workDir, helperName),
		Log:     logPath,
	}, nil
}

// Cleanup removes leftovers from interrupted downloads and old helpers.
func Cleanup(workDir string) {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		full := filepath.Join(workDir, name)
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		switch {
		case strings.HasSuffix(name, ".part"), strings.HasSuffix(name, ".new"):
			// A helper may still be creating or renaming the file right after
			// a restart. Only remove clearly stale leftovers.
			if now.Sub(info.ModTime()) > time.Hour {
				_ = os.Remove(full)
			}
		case strings.HasPrefix(name, "update-") && strings.HasSuffix(name, ".cmd"):
			if now.Sub(info.ModTime()) > 24*time.Hour {
				_ = os.Remove(full)
			}
		}
	}
}

// helperScript is kept ASCII-only; paths arrive as command-line arguments so
// non-Latin user profile names survive the cmd.exe round trip.
func helperScript() string {
	return strings.Join([]string{
		"@echo off",
		"setlocal",
		`set "TARGET=%~1"`,
		`set "SOURCE=%~2"`,
		`set "LOG=%~3"`,
		`set "NEW=%TARGET%.new"`,
		"set /a TRIES=0",
		":prepare",
		`if exist "%NEW%" del /f /q "%NEW%" >nul 2>&1`,
		`copy /y "%SOURCE%" "%NEW%" >nul 2>&1`,
		"if errorlevel 1 goto retry",
		`fc /b "%SOURCE%" "%NEW%" >nul 2>&1`,
		"if errorlevel 1 goto retry",
		":replace",
		`move /y "%NEW%" "%TARGET%" >nul 2>&1`,
		"if errorlevel 1 goto wait",
		`del /f /q "%SOURCE%" >nul 2>&1`,
		`start "" "%TARGET%"`,
		"exit /b 0",
		":retry",
		`if exist "%NEW%" del /f /q "%NEW%" >nul 2>&1`,
		":wait",
		"set /a TRIES+=1",
		"if %TRIES% GEQ 120 goto failed",
		"ping -n 2 127.0.0.1 >nul",
		`if exist "%NEW%" goto replace`,
		"goto prepare",
		":failed",
		`if exist "%NEW%" del /f /q "%NEW%" >nul 2>&1`,
		`>>"%LOG%" echo Update failed after %TRIES% attempts`,
		"exit /b 1",
		"",
	}, "\r\n")
}
