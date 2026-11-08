package log

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConsoleRouting(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	j := newTestJournal(t, Options{Stdout: &stdout, Stderr: &stderr})
	logger := j.Logger(SourceRoot)

	logger.Debug("debug line")
	logger.Info("info line")
	logger.Warn("warn line")
	logger.Error("error line")

	if !strings.Contains(stdout.String(), "debug line") ||
		!strings.Contains(stdout.String(), "info line") {
		t.Fatalf("stdout = %q; want debug and info", stdout.String())
	}
	if strings.Contains(stdout.String(), "warn line") ||
		strings.Contains(stdout.String(), "error line") {
		t.Fatalf("stdout = %q; want no warn or error", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warn line") ||
		!strings.Contains(stderr.String(), "error line") {
		t.Fatalf("stderr = %q; want warn and error", stderr.String())
	}
}

func TestFileOutput(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yo.log")
	j, err := Open(Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	j.Logger(SourceRoot).Info("hello", "answer", 42)
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "info root: hello answer=42") {
		t.Fatalf("file = %q; want the journal line", string(data))
	}
}

func TestRotation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yo.log")
	j, err := Open(Options{Path: path, MaxFileSize: 1})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	logger := j.Logger(SourceRoot)
	for i := 0; i < 3; i++ {
		logger.Info("line", "i", i)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file: %v", err)
	}
}

func TestRingAndClear(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, Options{RingSize: 2})
	logger := j.Logger(SourceRoot)
	logger.Info("one")
	logger.Info("two")
	logger.Info("three")

	entries := j.Recent()
	if len(entries) != 2 {
		t.Fatalf("Recent = %d entries; want 2", len(entries))
	}
	if entries[0].Message != "two" || entries[1].Message != "three" {
		t.Fatalf("Recent messages = %q, %q; want two, three", entries[0].Message, entries[1].Message)
	}
	if got := j.Snapshot(); len(got) != 2 {
		t.Fatalf("Snapshot = %d lines; want 2", len(got))
	}

	j.Clear()
	if got := j.Recent(); len(got) != 0 {
		t.Fatalf("Recent after Clear = %d entries; want 0", len(got))
	}
}

func TestLastErrorContext(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, Options{})
	logger := j.Logger(SourceRoot)
	logger.Info("context line")
	logger.Error("boom")

	got := j.LastErrorContext()
	if !strings.Contains(got, "context line") {
		t.Fatalf("LastErrorContext = %q; want the preceding line", got)
	}
	if !strings.Contains(got, "boom") {
		t.Fatalf("LastErrorContext = %q; want the error line", got)
	}
}

func TestRedaction(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	j := newTestJournal(t, Options{Stdout: &stdout})
	j.Logger(SourceRoot).Info("auth", "password", "hunter2", "token", "secret-value", "safe", "ok")

	got := stdout.String()
	for _, secret := range []string{"hunter2", "secret-value"} {
		if strings.Contains(got, secret) {
			t.Fatalf("journal contains %q: %q", secret, got)
		}
	}
	if !strings.Contains(got, `password="[redacted]"`) {
		t.Fatalf("journal = %q; want redacted password", got)
	}
}

func TestLevelLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warn"},
		{slog.LevelError, "error"},
		{LevelPanic, "panic"},
		{slog.Level(99), "unknown"},
	}
	for _, tt := range tests {
		if got := LevelLabel(tt.level); got != tt.want {
			t.Fatalf("LevelLabel(%d) = %q; want %q", tt.level, got, tt.want)
		}
	}
}

func TestEmptySourcePanics(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, Options{})
	assertPanics(t, func() { j.Logger("") })
}

func TestFileErrorKeepsJournalUsable(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing", "yo.log")
	j, err := Open(Options{Path: path, Stdout: &stdout})
	if err == nil {
		t.Fatal("Open succeeded; want a file error")
	}
	if j == nil {
		t.Fatal("Open returned a nil journal")
	}
	t.Cleanup(func() { _ = j.Close() })

	j.Logger(SourceRoot).Info("still works")
	if !strings.Contains(stdout.String(), "still works") {
		t.Fatalf("stdout = %q; want the entry", stdout.String())
	}
}

func TestConcurrentLogging(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, Options{RingSize: 100})
	logger := j.Logger(SourceRoot)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for k := 0; k < 50; k++ {
				logger.Info("concurrent", "n", n, "k", k)
			}
		}(i)
	}
	wg.Wait()

	if got := len(j.Recent()); got != 100 {
		t.Fatalf("Recent = %d entries; want 100", got)
	}
}

func newTestJournal(t *testing.T, opts Options) *Journal {
	t.Helper()
	j, err := Open(opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	fn()
}
