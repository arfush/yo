// Package log writes the application journal and keeps recent structured
// entries in memory for the log window.
//
// The implementation is log/slog: callers are handed a named logger by For and
// log through it like any slog.Logger. The handler only owns the file,
// rotation, redaction, the bounded in-memory ring and the last-error context;
// it never guesses where an entry came from. The source attribute is attached
// explicitly when a module builds its logger, and modules are expected to use
// the constants below for their names. The log lives next to the database, in
// %APPDATA%\yoanime\yoanime.log.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxFileSize = 5 << 20
	ringSize    = 2000
	contextSize = 24
)

// Additional levels used by the application. slog reserves values below Debug
// for custom levels; Notice sits between Info and Warn.
const (
	LevelTrace  = slog.Level(-8)
	LevelNotice = slog.LevelInfo + 2
	// LevelPanic sits above Error so it is always selected by every severity
	// filter and is kept in the last-error context. The UI deliberately does
	// not offer a panic option in the level menu.
	LevelPanic = slog.LevelError + 4
)

// Source constants name every module that writes to the journal. A package
// builds its logger once at load time:
//
//	var logger = log.For(log.SourceLibSocial)
//
// and every entry it writes then carries `source=libsocial`, without the log
// package having to look at the call stack.
const (
	SourceLibSocial = "libsocial"
	SourceApp       = "app"
	SourceBrowser   = "browser"
	SourceFilmlib   = "filmlib"
	SourceKodik     = "kodik"
	SourceLibrary   = "library"
	SourceMpv       = "mpv"
	SourceParty     = "party"
	SourcePlayback  = "playback"
	SourceStorage   = "storage"
	SourceUI        = "ui"
	SourceUpdate    = "update"
)

// Sources returns every module name in display order. The log window uses the
// list only as an affordance; the row value always comes from the entry.
func Sources() []string {
	return []string{
		SourceLibSocial, SourceApp, SourceBrowser, SourceFilmlib, SourceKodik,
		SourceLibrary, SourceMpv, SourceParty, SourcePlayback, SourceStorage,
		SourceUI, SourceUpdate,
	}
}

// Entry is one structured log record. Detail contains the formatted attributes
// after redaction; Source is the module that owns the logger that wrote it.
type Entry struct {
	ID      uint64
	Time    time.Time
	Level   slog.Level
	Source  string
	Message string
	Detail  string
	Line    string
}

type errorRecord struct {
	line    string
	context []string
}

type state struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	ring     []Entry
	sequence uint64
	lastErr  *errorRecord
	writeErr error
}

var defaultState state

// handler implements the journal: it formats a record into a line, keeps the
// in-memory ring and appends to the file. Attrs are accumulated through
// WithAttrs/WithGroup so the source name set by For lands on every record.
type handler struct {
	state  *state
	attrs  []slog.Attr
	groups []string
}

var defaultLogger = slog.New(&handler{state: &defaultState})

func init() {
	// Libraries that use slog directly should land in the same file and
	// in-memory ring, tagged as the default source.
	slog.SetDefault(defaultLogger)
}

// For returns a logger that tags every entry with the given source. Source
// should be one of the Source* constants so the log window sees a familiar
// module name.
func For(source string) *slog.Logger {
	attrs := []slog.Attr{slog.String("source", source)}
	return slog.New(&handler{state: &defaultState, attrs: attrs})
}

// Init opens the log file. Calling Init again closes the previous file and
// starts a fresh in-memory view. A nil path disables file output but keeps the
// ring available.
func Init(logPath string) error {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	s.path = logPath
	s.ring = nil
	s.sequence = 0
	s.lastErr = nil
	s.writeErr = nil
	if strings.TrimSpace(logPath) == "" {
		return nil
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.writeErr = err
		return err
	}
	s.file = f
	return nil
}

// Close flushes and closes the log file.
func Close() {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
}

// Snapshot returns the recent log lines, oldest first.
func Snapshot() []string {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.ring))
	for i, entry := range s.ring {
		out[i] = entry.Line
	}
	return out
}

// Recent returns the recent structured entries, oldest first. The caller gets
// a copy and may keep it after the ring rotates.
func Recent() []Entry {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.ring))
	copy(out, s.ring)
	return out
}

// Clear drops the in-memory entries shown by the log window. The log file is
// left untouched so it can still be attached to a bug report, and the last
// error context stays copyable until a new error replaces it.
func Clear() {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ring = nil
}

// LastErrorContext returns the last error and the log lines that led to it.
// Ordinary lines are not included when no error has been logged.
func LastErrorContext() string {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastErr == nil {
		return ""
	}
	lines := make([]string, 0, len(s.lastErr.context)+2)
	lines = append(lines, s.lastErr.context...)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, s.lastErr.line)
	return strings.Join(lines, "\n")
}

// LastWriteError reports a failure to open or write the log file. It is only
// used by startup diagnostics.
func LastWriteError() error {
	s := &defaultState
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeErr
}

// LevelLabel returns the stable lower-case name used by the file and the log
// window. Only the levels the application actually emits have names; every
// other slog value is reported as unknown.
func LevelLabel(level slog.Level) string {
	switch level {
	case LevelTrace:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case LevelNotice:
		return "notice"
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

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= LevelTrace
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	at := record.Time
	if at.IsZero() {
		at = time.Now()
	}
	detail, attrs := formatAttrs(h.attrs, h.groups, record)
	entry := Entry{
		Time:    at,
		Level:   record.Level,
		Source:  sourceFromAttrs(attrs),
		Message: record.Message,
		Detail:  detail,
	}

	s := h.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	entry.ID = s.sequence
	entry.Line = entry.formatLine()

	var previous []string
	if record.Level >= slog.LevelError {
		start := len(s.ring) - contextSize
		if start < 0 {
			start = 0
		}
		previous = make([]string, 0, len(s.ring)-start)
		for _, item := range s.ring[start:] {
			previous = append(previous, item.Line)
		}
	}
	s.ring = append(s.ring, entry)
	if len(s.ring) > ringSize {
		copy(s.ring, s.ring[len(s.ring)-ringSize:])
		s.ring = s.ring[:ringSize]
	}
	if record.Level >= slog.LevelError {
		s.lastErr = &errorRecord{line: entry.Line, context: previous}
	}
	s.writeLocked(entry.Line)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &next
}

func (h *handler) WithGroup(name string) slog.Handler {
	next := *h
	if name != "" {
		next.groups = append(append([]string(nil), h.groups...), name)
	}
	return &next
}

func (e Entry) formatLine() string {
	var b strings.Builder
	b.WriteString(e.Time.Format("2006-01-02 15:04:05.000"))
	b.WriteByte(' ')
	b.WriteString(LevelLabel(e.Level))
	b.WriteByte(' ')
	b.WriteString(e.Source)
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.Detail != "" {
		b.WriteByte(' ')
		b.WriteString(e.Detail)
	}
	return b.String()
}

// sourceFromAttrs returns the source attached by For, or the default name for
// records written through slog.SetDefault.
func sourceFromAttrs(attrs []slog.Attr) string {
	for _, attr := range attrs {
		if attr.Key == "source" && attr.Value.Kind() == slog.KindString {
			if source := attr.Value.String(); source != "" {
				return source
			}
		}
	}
	return "app"
}

func formatAttrs(base []slog.Attr, groups []string, record slog.Record) (string, []slog.Attr) {
	attrs := make([]slog.Attr, 0, len(base)+record.NumAttrs())
	attrs = append(attrs, base...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	var b strings.Builder
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		key := attr.Key
		if len(groups) > 0 && key != "" {
			key = strings.Join(append(append([]string(nil), groups...), key), ".")
		}
		value := attr.Value
		if value.Kind() == slog.KindGroup {
			for _, nested := range value.Group() {
				nested.Key = key + "." + nested.Key
				value = nested.Value
				key = nested.Key
				break
			}
		}
		if redactKey(key) {
			value = slog.StringValue("[redacted]")
		}
		attr = slog.Attr{Key: key, Value: value}
		clean = append(clean, attr)
		if key == "" {
			continue
		}
		// The source already has its own column in the log window. Keeping it
		// in the structured tail duplicates it in every expanded row.
		if key == "source" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatValue(value))
	}
	return b.String(), clean
}

func redactKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "key")
}

func formatValue(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return strconv.Quote(value.String())
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339)
	case slog.KindAny:
		any := value.Any()
		if err, ok := any.(error); ok {
			return strconv.Quote(err.Error())
		}
		if stringer, ok := any.(fmt.Stringer); ok {
			return strconv.Quote(stringer.String())
		}
		return fmt.Sprint(any)
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'g', -1, 64)
	case slog.KindGroup:
		var parts []string
		for _, attr := range value.Group() {
			parts = append(parts, attr.Key+"="+formatValue(attr.Value))
		}
		return "{" + strings.Join(parts, " ") + "}"
	default:
		return fmt.Sprint(value.Any())
	}
}

func (s *state) writeLocked(line string) {
	if s.file == nil {
		return
	}
	if err := s.rotateLocked(); err != nil {
		s.writeErr = err
		return
	}
	if _, err := s.file.WriteString(line + "\n"); err != nil {
		s.writeErr = err
	}
}

func (s *state) rotateLocked() error {
	if s.file == nil {
		return nil
	}
	info, err := s.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < maxFileSize {
		return nil
	}
	if err := s.file.Close(); err != nil {
		s.file = nil
		return err
	}
	s.file = nil
	backup := s.path + ".1"
	_ = os.Remove(s.path + ".2")
	_ = os.Rename(backup, s.path+".2")
	_ = os.Rename(s.path, backup)
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	s.file = f
	return nil
}
