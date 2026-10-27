package ui

import (
	"image"
	"log/slog"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	applog "github.com/arfush/yo/log"
)

var testLogger = applog.For(applog.SourceUI)

func TestLogScreenStartsFollowingNewestAndKeysScroll(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	if !w.autoScroll {
		t.Fatal("a new log window must follow the newest entry")
	}

	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 444)),
	}
	frame := func() {
		ops.Reset()
		w.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	w.maxScroll = 900
	w.bodyHeight = 300
	w.target = 0
	w.scroll = 0
	w.autoScroll = false
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame()
	if !w.autoScroll || w.target != 900 {
		t.Fatalf("Enter did not jump to the end: auto=%v target=%v", w.autoScroll, w.target)
	}

	router.Queue(key.Event{Name: key.NameHome, State: key.Press})
	frame()
	if w.autoScroll || w.target != 0 {
		t.Fatalf("Home did not leave the end: auto=%v target=%v", w.autoScroll, w.target)
	}

	router.Queue(key.Event{Name: key.NamePageDown, State: key.Press})
	frame()
	if w.autoScroll {
		t.Fatal("PageDown must stop end-following")
	}
	if w.target <= 0 {
		t.Fatalf("PageDown did not scroll: target=%v", w.target)
	}
}

func TestLogWheelScrollingLeavesEndFollow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	w.maxScroll = 1000
	w.scroll = 1000
	w.target = 1000
	w.autoScroll = true

	var router input.Router
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	w.scrollStep(gtx, 1000)
	router.Frame(gtx.Ops)

	router.Queue(pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(100, 100),
		Scroll:   f32.Pt(0, -200),
	})
	gtx.Ops = new(op.Ops)
	w.scrollStep(gtx, 1000)
	router.Frame(gtx.Ops)

	if w.autoScroll {
		t.Fatal("scrolling up did not stop end-following")
	}
	if w.target >= 1000 {
		t.Fatalf("wheel did not move the target away from the end: %v", w.target)
	}
}

func TestLogLineClickExpandsTruncatedRow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	line := strings.Repeat("очень длинная строка журнала ", 12)
	entry := applog.Entry{
		ID:      1,
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Source:  "ui",
		Message: line,
	}
	state := w.lineState(entry.ID)

	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(220, 120)),
	}
	frame := func() layout.Dimensions {
		ops.Reset()
		// Clicks are consumed in the same row-scoped measurement pass the real
		// table uses, so the height measured below already reflects the click.
		height := w.logRowHeight(gtx, entry, 220, state.expanded)
		w.consumeLineClick(gtx, 220, height, state, entry)
		height = w.logRowHeight(gtx, entry, 220, state.expanded)
		dims := w.layoutLine(gtx, entry, 220, height, 0, state)
		router.Frame(ops)
		return dims
	}
	collapsed := frame()

	router.Queue(
		pointer.Event{
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Kind:     pointer.Press,
			Position: f32.Pt(100, 10),
		},
		pointer.Event{
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Kind:     pointer.Release,
			Position: f32.Pt(100, 10),
		},
	)
	expanded := frame()
	if !state.expanded {
		t.Fatal("clicking a truncated line did not expand it")
	}
	if expanded.Size.Y <= collapsed.Size.Y {
		t.Fatalf("expanded row height = %d, want more than %d", expanded.Size.Y, collapsed.Size.Y)
	}
}

func TestWrapLogTextKeepsTheWholeLine(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	line := strings.Repeat("длинная строка ", 8) + strings.Repeat("x", 120)
	lines := wrapLogText(gtx, typo, line, face, logLineSize, 180)
	if len(lines) < 2 {
		t.Fatalf("wrapped %q into %d line(s)", line, len(lines))
	}
	joined := strings.ReplaceAll(strings.Join(lines, ""), " ", "")
	original := strings.ReplaceAll(line, " ", "")
	if joined != original {
		t.Fatalf("wrapped text lost runes:\n got %q\nwant %q", joined, original)
	}
	for _, text := range lines {
		if _, ink := typo.measure(gtx, text, face, logLineSize); ink.Dx() > 180 {
			t.Fatalf("wrapped line is %d px wide, want at most 180: %q", ink.Dx(), text)
		}
		if strings.Contains(text, "…") {
			t.Fatalf("expanded line is still truncated: %q", text)
		}
	}
}

func TestLogFilterAndSearch(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	w.entries = []applog.Entry{
		{ID: 1, Level: slog.LevelInfo, Source: "store", Message: "history loaded"},
		{ID: 2, Level: slog.LevelWarn, Source: "party", Message: "party anchor lost", Detail: `relay="eu"`},
		{ID: 3, Level: slog.LevelError, Source: "playback", Message: "playback failed", Detail: `error="mpv exited"`},
		{ID: 4, Level: slog.LevelDebug, Source: "updates", Message: "update check started"},
	}

	w.filter = logLevelFilter{level: slog.LevelWarn}
	visible := w.visibleEntries()
	if len(visible) != 2 {
		t.Fatalf("warning filter returned %d entries, want warn+error = 2", len(visible))
	}

	w.filter = logLevelFilter{all: true}
	w.search.SetText("MPV")
	visible = w.visibleEntries()
	if len(visible) != 1 || visible[0].Message != "playback failed" {
		t.Fatalf("search over detail returned %d entries, want the playback error", len(visible))
	}

	w.search.SetText("relay")
	visible = w.visibleEntries()
	if len(visible) != 1 || visible[0].Source != "party" {
		t.Fatalf("search over detail returned %d entries, want the party warning", len(visible))
	}

	w.search.SetText("")
	w.filter = logLevelFilter{level: slog.LevelInfo}
	visible = w.visibleEntries()
	if len(visible) != 3 {
		t.Fatalf("info filter returned %d entries, want info+warn+error = 3", len(visible))
	}

	w.filter = logLevelFilter{level: applog.LevelTrace}
	if visible := w.visibleEntries(); len(visible) != 4 {
		t.Fatalf("trace filter returned %d entries, want the whole journal", len(visible))
	}
}

func TestLogLevelLabelsAndColours(t *testing.T) {
	cases := []struct {
		level slog.Level
		label string
	}{
		{applog.LevelTrace, "trace"},
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{applog.LevelNotice, "notice"},
		{slog.LevelWarn, "warn"},
		{slog.LevelError, "error"},
		{applog.LevelPanic, "panic"},
		{slog.Level(100), "unknown"},
	}
	for _, test := range cases {
		if got := applog.LevelLabel(test.level); got != test.label {
			t.Errorf("LevelLabel(%v) = %q, want %q", test.level, got, test.label)
		}
		if got := logLevelColor(test.level); got.A == 0 {
			t.Errorf("logLevelColor(%v) is transparent", test.level)
		}
	}
	if logLevelColor(slog.LevelError) == logLevelColor(slog.LevelWarn) {
		t.Fatal("error and warning rows share the same colour")
	}
}

func TestLogPauseAndClear(t *testing.T) {
	if err := applog.Init(""); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = applog.Init("") }()

	testLogger.Info("first entry")
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	w.refreshEntries()
	if len(w.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(w.entries))
	}

	w.togglePause()
	testLogger.Info("second entry")
	w.refreshEntries()
	if len(w.entries) != 1 {
		t.Fatal("a paused log window consumed a new entry")
	}

	w.togglePause()
	if len(w.entries) != 2 {
		t.Fatal("resuming the log window did not load new entries")
	}

	w.clearEntries()
	if len(w.entries) != 0 {
		t.Fatal("clear did not empty the visible entries")
	}
	if len(applog.Snapshot()) != 0 {
		t.Fatal("clear did not empty the logger ring")
	}
}

func TestLogScreenLayoutAtMinimumContentSize(t *testing.T) {
	if err := applog.Init(""); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = applog.Init("") }()
	testLogger.Info("history loaded", "count", 3)
	testLogger.Warn("party relay lag", "delay", "1.2s")
	testLogger.Error("playback failed", "error", "mpv exited")

	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogScreen(typo)
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 444)),
	}
	frame := func() {
		ops.Reset()
		w.layout(gtx)
		router.Frame(ops)
	}
	frame()
	w.levelMenu.open = true
	frame()
}
