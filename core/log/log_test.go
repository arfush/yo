package log_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/arfush/yo/core/log"
)

func TestConsoleRouting(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	j := newTestJournal(t, log.Options{Stdout: &stdout, Stderr: &stderr})
	logger := j.Logger(log.SourceRoot)

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
	j, err := log.Open(log.Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	j.Logger(log.SourceRoot).Info("hello", "answer", 42)
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
	j, err := log.Open(log.Options{Path: path, MaxFileSize: 1})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	logger := j.Logger(log.SourceRoot)
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

func TestRotationFailureReportsToStderr(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yo.log")
	// A non-empty directory in the backup slots makes the rename steps fail.
	if err := os.MkdirAll(filepath.Join(path+".2", "busy"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Mkdir(path+".1", 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	var stderr bytes.Buffer
	j, err := log.Open(log.Options{Path: path, Stderr: &stderr, MaxFileSize: 1})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })

	logger := j.Logger(log.SourceRoot)
	logger.Info("one")
	logger.Info("two")

	got := stderr.String()
	if !strings.Contains(got, "warn journal: journal rotation failed") {
		t.Fatalf("stderr = %q; want the rotation failure", got)
	}
	if !strings.Contains(got, "path=") || !strings.Contains(got, "error=") {
		t.Fatalf("stderr = %q; want path and error attributes", got)
	}

	logger.Info("three")
	if got := len(j.Recent()); got != 3 {
		t.Fatalf("Recent = %d entries; want 3", got)
	}
}

func TestRingAndClear(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, log.Options{RingSize: 2})
	logger := j.Logger(log.SourceRoot)
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

	j := newTestJournal(t, log.Options{})
	logger := j.Logger(log.SourceRoot)
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
	j := newTestJournal(t, log.Options{Stdout: &stdout})
	j.Logger(log.SourceRoot).Info("auth", "password", "hunter2", "token", "secret-value", "safe", "ok")

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

func TestStringQuoting(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	j := newTestJournal(t, log.Options{Stdout: &stdout})
	logger := j.Logger(log.SourceRoot)
	logger.Info("journal opened", "path", `C:\Users\example\AppData\Roaming\yo\yo.log`)
	logger.Info("say", "text", `a "quoted" value`)

	got := stdout.String()
	if !strings.Contains(got, `path="C:\Users\example\AppData\Roaming\yo\yo.log"`) {
		t.Fatalf("journal = %q; want a readable Windows path", got)
	}
	if strings.Contains(got, `\\`) {
		t.Fatalf("journal = %q; want single backslashes", got)
	}
	if !strings.Contains(got, `text="a \"quoted\" value"`) {
		t.Fatalf("journal = %q; want escaped quotes", got)
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
		{log.LevelPanic, "panic"},
		{slog.Level(99), "unknown"},
	}
	for _, tt := range tests {
		if got := log.LevelLabel(tt.level); got != tt.want {
			t.Fatalf("LevelLabel(%d) = %q; want %q", tt.level, got, tt.want)
		}
	}
}

func TestEmptySourcePanics(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, log.Options{})
	assertPanics(t, func() { j.Logger("") })
}

func TestFileErrorKeepsJournalUsable(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing", "yo.log")
	j, err := log.Open(log.Options{Path: path, Stdout: &stdout})
	if err == nil {
		t.Fatal("Open succeeded; want a file error")
	}
	if j == nil {
		t.Fatal("Open returned a nil journal")
	}
	t.Cleanup(func() { _ = j.Close() })

	j.Logger(log.SourceRoot).Info("still works")
	if !strings.Contains(stdout.String(), "still works") {
		t.Fatalf("stdout = %q; want the entry", stdout.String())
	}
}

func TestConcurrentLogging(t *testing.T) {
	t.Parallel()

	j := newTestJournal(t, log.Options{RingSize: 100})
	logger := j.Logger(log.SourceRoot)

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

func newTestJournal(t *testing.T, opts log.Options) *log.Journal {
	t.Helper()
	j, err := log.Open(opts)
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
