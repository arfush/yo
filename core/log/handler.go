package log

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const contextSize = 24

// handler implements the journal. The source lives in its own field, so
// WithAttrs and WithGroup can add context but cannot replace the module name.
type handler struct {
	journal *Journal
	source  Source
	attrs   []slog.Attr
	groups  []string
}

func (h *handler) Enabled(context.Context, slog.Level) bool { return true }

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	at := record.Time
	if at.IsZero() {
		at = time.Now()
	}
	entry := Entry{
		Time:    at,
		Level:   record.Level,
		Source:  h.source,
		Message: record.Message,
		Detail:  formatAttrs(h.attrs, h.groups, record),
	}

	j := h.journal
	j.mu.Lock()
	defer j.mu.Unlock()

	j.sequence++
	entry.ID = j.sequence
	entry.Line = entry.formatLine()

	var previous []string
	if record.Level >= slog.LevelError {
		start := len(j.ring) - contextSize
		if start < 0 {
			start = 0
		}
		previous = make([]string, 0, len(j.ring)-start)
		for _, item := range j.ring[start:] {
			previous = append(previous, item.Line)
		}
	}

	j.ring = append(j.ring, entry)
	if len(j.ring) > j.ringSize {
		copy(j.ring, j.ring[len(j.ring)-j.ringSize:])
		j.ring = j.ring[:j.ringSize]
	}
	if record.Level >= slog.LevelError {
		j.lastErr = &errorRecord{line: entry.Line, context: previous}
	}
	return j.writeLocked(entry)
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
	b.WriteString(e.Source.String())
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.Detail != "" {
		b.WriteByte(' ')
		b.WriteString(e.Detail)
	}
	return b.String()
}

func formatAttrs(base []slog.Attr, groups []string, record slog.Record) string {
	attrs := make([]slog.Attr, 0, len(base)+record.NumAttrs())
	attrs = append(attrs, base...)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	var b strings.Builder
	for _, attr := range expandGroups(attrs, groups) {
		if attr.Key == "" {
			continue
		}
		value := attr.Value
		if redactKey(attr.Key) {
			value = slog.StringValue("[redacted]")
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(attr.Key)
		b.WriteByte('=')
		b.WriteString(formatValue(value))
	}
	return b.String()
}

// expandGroups flattens group attributes into dotted keys. An empty group name
// inlines its members into the current prefix, matching slog semantics.
func expandGroups(attrs []slog.Attr, groups []string) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	var walk func(list []slog.Attr, prefix []string)
	walk = func(list []slog.Attr, prefix []string) {
		for _, src := range list {
			key := src.Key
			if key != "" && len(prefix) > 0 {
				key = strings.Join(append(append([]string(nil), prefix...), key), ".")
			}
			if src.Value.Kind() != slog.KindGroup {
				out = append(out, slog.Attr{Key: key, Value: src.Value})
				continue
			}
			nested := prefix
			if src.Key != "" {
				nested = append(append([]string(nil), prefix...), src.Key)
			}
			walk(src.Value.Group(), nested)
		}
	}
	walk(attrs, groups)
	return out
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
		parts := make([]string, 0, len(value.Group()))
		for _, attr := range value.Group() {
			parts = append(parts, attr.Key+"="+formatValue(attr.Value))
		}
		return "{" + strings.Join(parts, " ") + "}"
	default:
		return fmt.Sprint(value.Any())
	}
}
