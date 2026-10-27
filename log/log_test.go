package log

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFileAndLastErrorContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yoanime.log")
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	defer Close()

	logger := For(SourceLibrary)
	logger.Info("history loaded", "count", 3)
	logger.Error("playback failed", "error", errors.New("mpv exited"))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "history loaded") {
		t.Fatalf("log file does not contain the info entry:\n%s", data)
	}
	if !strings.Contains(string(data), "error library: playback failed") {
		t.Fatalf("log file does not contain the error:\n%s", data)
	}

	context := LastErrorContext()
	if !strings.Contains(context, "history loaded") {
		t.Fatalf("last error context = %q, want the preceding line", context)
	}
	if !strings.Contains(context, "error library: playback failed") {
		t.Fatalf("last error context = %q, want the error line", context)
	}
}

func TestForTagsEveryEntryWithTheSource(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "yoanime.log")); err != nil {
		t.Fatal(err)
	}
	defer Close()

	For(SourceAnimelib).Info("request")
	For(SourceStore).Info("schema ready")

	entries := Recent()
	if len(entries) != 2 {
		t.Fatalf("recent length = %d, want 2", len(entries))
	}
	if entries[0].Source != SourceAnimelib {
		t.Fatalf("first entry source = %q, want %q", entries[0].Source, SourceAnimelib)
	}
	if entries[1].Source != SourceStore {
		t.Fatalf("second entry source = %q, want %q", entries[1].Source, SourceStore)
	}
}

func TestSourceIsNotDuplicatedInExpandedDetail(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "yoanime.log")); err != nil {
		t.Fatal(err)
	}
	defer Close()

	For(SourceStore).Info("history loaded", "count", 3)
	entries := Recent()
	if len(entries) != 1 {
		t.Fatalf("recent length = %d, want 1", len(entries))
	}
	if entries[0].Source != SourceStore {
		t.Fatalf("source = %q, want %q", entries[0].Source, SourceStore)
	}
	if strings.Contains(entries[0].Detail, "source=") {
		t.Fatalf("detail duplicates the source column: %q", entries[0].Detail)
	}
	if entries[0].Detail != `count=3` {
		t.Fatalf("detail = %q, want count=3", entries[0].Detail)
	}
}

func TestSnapshotDoesNotIncludeOrdinaryLinesInCopy(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "yoanime.log")); err != nil {
		t.Fatal(err)
	}
	defer Close()

	For(SourceApp).Info("ordinary line")
	got := LastErrorContext()
	if strings.Contains(got, "ordinary line") {
		t.Fatalf("copy without an error contains ordinary lines: %q", got)
	}
}

func TestClearDropsRingButKeepsFileAndLastError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yoanime.log")
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	defer Close()

	logger := For(SourceUI)
	logger.Info("ordinary line")
	logger.Error("playback failed", "error", "mpv exited")
	if len(Snapshot()) != 2 {
		t.Fatalf("snapshot length = %d, want 2", len(Snapshot()))
	}
	context := LastErrorContext()

	Clear()

	if lines := Snapshot(); len(lines) != 0 {
		t.Fatalf("snapshot after clear = %q, want empty", lines)
	}
	if got := LastErrorContext(); got != context {
		t.Fatalf("last error context changed after clear:\n got %q\nwant %q", got, context)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ordinary line") || !strings.Contains(string(data), "error ui: playback failed") {
		t.Fatalf("clear removed entries from the log file:\n%s", data)
	}
}

func TestRedactsSecrets(t *testing.T) {
	if err := Init(""); err != nil {
		t.Fatal(err)
	}
	For(SourcePlayback).Info("request", "token", "super-secret", "url", "https://example.test")
	lines := Snapshot()
	if len(lines) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(lines))
	}
	if strings.Contains(lines[0], "super-secret") {
		t.Fatalf("secret leaked into the log: %q", lines[0])
	}
	if !strings.Contains(lines[0], "https://example.test") {
		t.Fatalf("non-secret value was removed: %q", lines[0])
	}
}
