// Package log writes the application log and keeps recent structured entries
// in memory for the log window.
//
// The implementation is built on log/slog: callers use the small wrappers in
// this package, while the handler owns the file, rotation, redaction, source
// inference and the bounded in-memory ring. The log lives next to the database,
// in %APPDATA%\yoanime\yoanime.log.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
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
)

// Entry is one structured log record. Detail contains the formatted attributes
// after redaction; Source is the lower-case package/feature name inferred from
// the caller.
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

type handler struct {
	state  *state
	attrs  []slog.Attr
	groups []string
}

var defaultLogger = slog.New(&handler{state: &defaultState})

func init() {
	// Applications and libraries that use slog directly should land in the
	// same file and in-memory ring.
	slog.SetDefault(defaultLogger)
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

// Trace writes the most detailed diagnostic entry.
func Trace(message string, attrs ...any) {
	logAt(LevelTrace, message, attrs...)
}

// Debug writes a diagnostic entry.
func Debug(message string, attrs ...any) {
	logAt(slog.LevelDebug, message, attrs...)
}

// Info writes an informational entry.
func Info(message string, attrs ...any) {
	logAt(slog.LevelInfo, message, attrs...)
}

// Notice writes a notable but non-problematic event.
func Notice(message string, attrs ...any) {
	logAt(LevelNotice, message, attrs...)
}

// Warn writes a recoverable problem.
func Warn(message string, attrs ...any) {
	logAt(slog.LevelWarn, message, attrs...)
}

// Error writes a failure. Its surrounding lines become the context copied from
// the log window.
func Error(message string, attrs ...any) {
	logAt(slog.LevelError, message, attrs...)
}

func logAt(level slog.Level, message string, attrs ...any) {
	record := slog.NewRecord(time.Now(), level, message, callerPC())
	record.Add(attrs...)
	_ = defaultLogger.Handler().Handle(context.Background(), record)
}

// callerPC finds the first frame outside the log package implementation. This
// keeps the source meaningful even when the compiler inlines the wrappers.
func callerPC() uintptr {
	var pcs [8]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	var fallback uintptr
	for {
		frame, more := frames.Next()
		if isLogImplementation(frame) {
			if !more {
				break
			}
			continue
		}
		if fallback == 0 {
			fallback = frame.PC + 1
		}
		// Prefer an application frame over an inlined stdlib call such as
		// errors.New in an argument list.
		if sourceFromFile(frame.File) != "" {
			// CallersFrames returns the call-site PC minus one. NewRecord
			// expects a return address, so restore it before source lookup.
			return frame.PC + 1
		}
		if !more {
			break
		}
	}
	return fallback
}

func isLogImplementation(frame runtime.Frame) bool {
	if !strings.Contains(filepathSlash(frame.File), "/log/") {
		return false
	}
	if strings.HasSuffix(frame.File, "_test.go") {
		return false
	}
	return strings.Contains(frame.Function, "/log.")
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
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
		return "Ошибок пока не было."
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
// window.
func LevelLabel(level slog.Level) string {
	switch {
	case level <= LevelTrace:
		return "trace"
	case level < slog.LevelInfo:
		return "debug"
	case level < LevelNotice:
		return "info"
	case level < slog.LevelWarn:
		return "notice"
	case level < slog.LevelError:
		return "warn"
	default:
		return "error"
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
		Source:  sourceName(record.PC),
		Message: record.Message,
		Detail:  detail,
	}
	entry.Line = entry.formatLine()

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
	_ = attrs
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

func sourceName(pc uintptr) string {
	if pc == 0 {
		return "app"
	}
	// A single PC can expand into several inline frames. The innermost one is
	// often a helper such as errors.New, so walk them until an application
	// package is found instead of naming the source after a stdlib helper.
	frames := runtime.CallersFrames([]uintptr{pc})
	fallback := ""
	for {
		frame, more := frames.Next()
		if name := sourceFromFile(frame.File); name != "" {
			return name
		}
		if fallback == "" {
			fallback = sourceFromFunction(frame.Function)
		}
		if !more {
			break
		}
	}
	if fallback != "" {
		return fallback
	}
	return "app"
}

func sourceFromFile(path string) string {
	path = filepathSlash(path)
	for _, name := range []string{
		"animelib", "app", "browser", "catalog", "kodik", "kinopoisk",
		"library", "log", "metadata", "mpv", "party", "playback", "store",
		"token", "updater", "updates", "version", "ui",
	} {
		if strings.Contains(path, "/"+name+"/") {
			return name
		}
	}
	if strings.Contains(path, "/platform/") {
		return "platform"
	}
	if strings.Contains(path, "/cmd/") {
		return "app"
	}
	return ""
}

func sourceFromFunction(function string) string {
	name := function
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	if index := strings.IndexByte(name, '.'); index >= 0 {
		name = name[:index]
	}
	return strings.ToLower(name)
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
