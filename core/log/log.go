package log

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Source names the module that wrote a journal entry.
type Source string

const (
	// SourceRoot is the composition root: startup, window creation and
	// configuration, shutdown and process-level failures.
	SourceRoot Source = "root"
)

// String returns the source name written to the journal.
func (s Source) String() string { return string(s) }

// Valid reports whether the source has a name. The zero Source is invalid.
func (s Source) Valid() bool { return s != "" }

const (
	// DefaultMaxFileSize is the journal file size that triggers rotation.
	DefaultMaxFileSize int64 = 5 << 20
	// DefaultRingSize is the number of recent entries kept in memory.
	DefaultRingSize = 2000
)

// LevelPanic sits above Error so it is always selected by severity filters and
// is kept in the last-error context.
const LevelPanic = slog.LevelError + 4

// Logger is a standard slog logger with the journal handler installed.
type Logger = slog.Logger

// Options configures the journal. Path opens or creates the log file; an empty
// path disables file output. Stdout receives Debug and Info lines; Stderr
// receives Warn, Error and Panic lines. A nil writer disables that stream.
// Zero MaxFileSize and RingSize use the defaults above.
type Options struct {
	Path        string
	Stdout      io.Writer
	Stderr      io.Writer
	MaxFileSize int64
	RingSize    int
}

// Journal owns the file, console streams and in-memory ring. It is safe for
// concurrent use and must not be copied.
type Journal struct {
	mu          sync.Mutex
	path        string
	maxFileSize int64
	ringSize    int
	file        *os.File
	stdout      io.Writer
	stderr      io.Writer
	ring        []Entry
	sequence    uint64
	lastErr     *errorRecord
}

// Open builds a journal and opens its file when Path is set. A file error is
// returned together with a usable Journal: console and the ring keep working,
// so the caller can record the failure and continue.
func Open(opts Options) (*Journal, error) {
	j := &Journal{
		path:        strings.TrimSpace(opts.Path),
		maxFileSize: opts.MaxFileSize,
		ringSize:    opts.RingSize,
		stdout:      opts.Stdout,
		stderr:      opts.Stderr,
	}
	if j.maxFileSize <= 0 {
		j.maxFileSize = DefaultMaxFileSize
	}
	if j.ringSize <= 0 {
		j.ringSize = DefaultRingSize
	}
	if j.path == "" {
		return j, nil
	}
	file, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return j, err
	}
	j.file = file
	return j, nil
}

// Close flushes and closes the journal file. It is idempotent.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}

// Logger returns a logger tagged with source. An empty source panics: a module
// must name itself.
func (j *Journal) Logger(source Source) *Logger {
	if !source.Valid() {
		panic("log: source is empty")
	}
	return slog.New(&handler{journal: j, source: source})
}

// LevelLabel returns the stable lower-case name used by the file and the log
// window. Only the levels the application emits have names.
func LevelLabel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	case LevelPanic:
		return "panic"
	default:
		return "unknown"
	}
}

func (j *Journal) console(level slog.Level) io.Writer {
	if level >= slog.LevelWarn {
		return j.stderr
	}
	return j.stdout
}

func (j *Journal) writeLocked(entry Entry) error {
	writers := make([]io.Writer, 0, 2)
	if j.file != nil {
		if err := j.rotateLocked(); err == nil {
			writers = append(writers, j.file)
		}
	}
	if dst := j.console(entry.Level); dst != nil {
		writers = append(writers, dst)
	}
	line := entry.Line + "\n"
	var first error
	for _, dst := range writers {
		if _, err := io.WriteString(dst, line); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (j *Journal) rotateLocked() error {
	if j.file == nil {
		return nil
	}
	info, err := j.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < j.maxFileSize {
		return nil
	}
	if err := j.file.Close(); err != nil {
		j.file = nil
		return err
	}
	j.file = nil
	backup := j.path + ".1"
	_ = os.Remove(j.path + ".2")
	if err := os.Rename(backup, j.path+".2"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(j.path, backup); err != nil {
		return err
	}
	file, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	j.file = file
	return nil
}
