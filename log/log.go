// Package log writes the application log and keeps the recent entries in
// memory for the log window.
//
// The log is intentionally independent from the UI: background workers,
// network clients and the store can all call it without importing the UI. The
// file lives next to the database, in %APPDATA%\yoanime\yoanime.log.
package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxFileSize = 5 << 20
	ringSize    = 2000
	contextSize = 24
)

type errorRecord struct {
	line    string
	context []string
}

var (
	mu       sync.Mutex
	file     *os.File
	path     string
	ring     []string
	lastErr  *errorRecord
	writeErr error
)

// Init opens the log file. Calling Init again closes the previous file. A nil
// path disables file output but keeps the in-memory ring available.
func Init(logPath string) error {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
		file = nil
	}
	path = logPath
	ring = nil
	lastErr = nil
	if strings.TrimSpace(logPath) == "" {
		return nil
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		writeErr = err
		return err
	}
	file = f
	writeErr = nil
	return nil
}

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
		file = nil
	}
}

// Debug writes a diagnostic entry.
func Debug(message string, attrs ...any) {
	write("DEBUG", message, attrs...)
}

// Info writes an informational entry.
func Info(message string, attrs ...any) {
	write("INFO", message, attrs...)
}

// Warn writes a recoverable problem.
func Warn(message string, attrs ...any) {
	write("WARN", message, attrs...)
}

// Error writes a failure. Its surrounding lines become the context copied from
// the log window.
func Error(message string, attrs ...any) {
	write("ERROR", message, attrs...)
}

// Snapshot returns the recent log lines, oldest first.
func Snapshot() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, len(ring))
	copy(out, ring)
	return out
}

// Clear drops the in-memory entries shown by the log window. The log file is
// left untouched so it can still be attached to a bug report, and the last
// error context stays copyable until a new error replaces it.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	ring = nil
}

// LastErrorContext returns the last error and the log lines that led to it.
// Ordinary lines are not included when no error has been logged.
func LastErrorContext() string {
	mu.Lock()
	defer mu.Unlock()
	if lastErr == nil {
		return "Ошибок пока не было."
	}
	lines := make([]string, 0, len(lastErr.context)+2)
	lines = append(lines, lastErr.context...)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, lastErr.line)
	return strings.Join(lines, "\n")
}

// LastWriteError reports a failure to open or write the log file. It is only
// used by startup diagnostics.
func LastWriteError() error {
	mu.Lock()
	defer mu.Unlock()
	return writeErr
}

func write(level, message string, attrs ...any) {
	line := formatLine(level, message, attrs...)

	mu.Lock()
	defer mu.Unlock()

	context := make([]string, 0, contextSize)
	if len(ring) > 0 {
		start := len(ring) - contextSize
		if start < 0 {
			start = 0
		}
		context = append(context, ring[start:]...)
	}
	ring = append(ring, line)
	if len(ring) > ringSize {
		ring = append([]string(nil), ring[len(ring)-ringSize:]...)
	}
	if level == "ERROR" {
		lastErr = &errorRecord{line: line, context: context}
	}

	writeLocked(line)
}

func formatLine(level, message string, attrs ...any) string {
	var b strings.Builder
	b.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	b.WriteByte(' ')
	b.WriteString(level)
	b.WriteByte(' ')
	b.WriteString(message)
	for index := 0; index+1 < len(attrs); index += 2 {
		key := fmt.Sprint(attrs[index])
		value := formatValue(key, attrs[index+1])
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
	}
	return b.String()
}

func formatValue(key string, value any) string {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "key") {
		return "[redacted]"
	}
	if err, ok := value.(error); ok {
		return fmt.Sprintf("%q", err.Error())
	}
	return fmt.Sprintf("%q", fmt.Sprint(value))
}

func writeLocked(line string) {
	if file == nil {
		return
	}
	if err := rotateLocked(); err != nil {
		writeErr = err
		return
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		writeErr = err
	}
}

func rotateLocked() error {
	if file == nil {
		return nil
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < maxFileSize {
		return nil
	}
	if err := file.Close(); err != nil {
		file = nil
		return err
	}
	file = nil
	backup := path + ".1"
	_ = os.Remove(path + ".2")
	_ = os.Rename(backup, path+".2")
	_ = os.Rename(path, backup)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	file = f
	return nil
}
