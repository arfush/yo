package log

import (
	"log/slog"
	"strings"
	"time"
)

// Entry is one structured log record. Detail contains the formatted attributes
// after redaction; Source is the module that owns the logger that wrote it.
type Entry struct {
	ID      uint64
	Time    time.Time
	Level   slog.Level
	Source  Source
	Message string
	Detail  string
	Line    string
}

type errorRecord struct {
	line    string
	context []string
}

// Recent returns the recent structured entries, oldest first. The caller gets
// a copy and may keep it after the ring rotates.
func (j *Journal) Recent() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Entry, len(j.ring))
	copy(out, j.ring)
	return out
}

// Snapshot returns the recent log lines, oldest first.
func (j *Journal) Snapshot() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]string, len(j.ring))
	for i, entry := range j.ring {
		out[i] = entry.Line
	}
	return out
}

// Clear drops the in-memory entries shown by the log window. The log file is
// left untouched, and the last error context stays copyable until a new error
// replaces it.
func (j *Journal) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ring = nil
}

// LastErrorContext returns the last error and the log lines that led to it.
// Ordinary lines are not included when no error has been logged.
func (j *Journal) LastErrorContext() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.lastErr == nil {
		return ""
	}
	lines := make([]string, 0, len(j.lastErr.context)+2)
	lines = append(lines, j.lastErr.context...)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, j.lastErr.line)
	return strings.Join(lines, "\n")
}
