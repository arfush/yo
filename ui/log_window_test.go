package ui

import (
	"image"
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

	"github.com/arfush/yoanime/log"
)

func TestLogWindowStartsAtNewestAndEnterJumpsToEnd(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
	if !w.list.ScrollToEnd {
		t.Fatal("a new log window must open on the newest entry")
	}

	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	frame := func() {
		ops.Reset()
		w.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()

	w.list.Position.BeforeEnd = true
	router.Queue(key.Event{Name: key.NameEnter, State: key.Press})
	frame()
	if w.list.Position.BeforeEnd {
		t.Fatal("Enter did not leave the older entries")
	}
	if !w.list.ScrollToEnd {
		t.Fatal("Enter disabled end-following")
	}
}

func TestLogFollowModeTracksShortAndLongLogs(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}

	short := newLogWindow(typo)
	short.syncFollowMode(gtx, []logEntry{
		{raw: "one", message: "one"},
		{raw: "two", message: "two"},
	}, 400, 300)
	if short.list.ScrollToEnd {
		t.Fatal("a short log should start at the top instead of bottom-aligning")
	}
	if short.list.Position.First != 0 || short.list.Position.Offset != 0 {
		t.Fatal("a short log should keep the list at the top")
	}

	longEntries := make([]logEntry, 100)
	for index := range longEntries {
		longEntries[index] = logEntry{raw: "line", message: "line"}
	}
	long := newLogWindow(typo)
	long.syncFollowMode(gtx, longEntries, 400, 40)
	if !long.list.ScrollToEnd {
		t.Fatal("a log taller than the viewport should open at the newest entry")
	}
	long.list.Position.BeforeEnd = true
	long.syncFollowMode(gtx, longEntries, 400, 40)
	if long.list.ScrollToEnd || long.autoScroll {
		t.Fatal("scrolling into older entries should stop following the end")
	}
	long.jumpToEnd()
	long.syncFollowMode(gtx, longEntries, 400, 40)
	if !long.list.ScrollToEnd || !long.autoScroll {
		t.Fatal("jumpToEnd did not re-enable end-following")
	}

	long.setAutoScroll(false)
	long.list.Position.BeforeEnd = false
	long.syncFollowMode(gtx, longEntries, 400, 40)
	if long.list.ScrollToEnd {
		t.Fatal("turning autoscroll off did not stop end-following")
	}
	long.setAutoScroll(true)
	if !long.list.ScrollToEnd || long.list.Position.BeforeEnd {
		t.Fatal("turning autoscroll on did not jump to the newest entry")
	}
}

func TestLogLineClickExpandsTruncatedRow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
	line := strings.Repeat("очень длинная строка журнала ", 12)
	entry := parseLogEntry(line)
	state := w.lineState(line)

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
		dims := w.layoutLine(gtx, entry, 220, state)
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
	lines := wrapLogText(
		gtx,
		typo,
		line,
		face,
		logLineSize,
		180,
	)
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

func TestParseLogEntrySplitsColumnsAndAttributes(t *testing.T) {
	entry := parseLogEntry(`2026-10-27 10:11:12.345 WARN playback failed to resolve episode session="abc" attempt="2"`)
	if got := entry.at.Format("2006-01-02 15:04:05.000"); got != "2026-10-27 10:11:12.345" {
		t.Fatalf("timestamp = %q", got)
	}
	if entry.level != logLevelWarning {
		t.Fatalf("level = %v, want warning", entry.level)
	}
	if entry.source != "Плеер" {
		t.Fatalf("source = %q, want Плеер", entry.source)
	}
	if entry.message != "playback failed to resolve episode" {
		t.Fatalf("message = %q", entry.message)
	}
	if entry.detail != `playback failed to resolve episode session="abc" attempt="2"` {
		t.Fatalf("detail = %q", entry.detail)
	}
	if !strings.Contains(entry.searchText(), "abc") || !strings.Contains(entry.searchText(), "attempt") {
		t.Fatalf("search text lost structured attributes: %q", entry.searchText())
	}

	debug := parseLogEntry(`2026-10-27 10:11:12.345 DEBUG update check started`)
	if debug.level != logLevelInfo {
		t.Fatalf("DEBUG level = %v, want information", debug.level)
	}
	if debug.source != "Обновления" {
		t.Fatalf("DEBUG source = %q, want Обновления", debug.source)
	}

	plain := parseLogEntry("ordinary message")
	if !plain.at.IsZero() || plain.level != logLevelInfo || plain.message != "ordinary message" || plain.source != "Приложение" {
		t.Fatalf("plain line parsed as %+v", plain)
	}

	unquoted := parseLogEntry(`2026-10-27 10:11:12.345 INFO request error=boom`)
	if unquoted.detail != unquoted.message {
		t.Fatalf("unquoted tail was treated as structured attributes: %q", unquoted.detail)
	}
}

func TestLogSourceLabels(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"update check started", "Обновления"},
		{"downloading update", "Обновления"},
		{"github release downloaded", "Обновления"},
		{"discord presence updated", "Приложение"},
		{"party anchor received", "Комната"},
		{"room heartbeat sent", "Комната"},
		{"mpv episode started", "Плеер"},
		{"Kodik resolved stream", "Плеер"},
		{"window attached", "Приложение"},
	}
	for _, test := range cases {
		if got := logSource(test.message); got != test.want {
			t.Errorf("logSource(%q) = %q, want %q", test.message, got, test.want)
		}
	}
}

func TestLogFilterSearchAndCounts(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
	w.entries = []logEntry{
		parseLogEntry(`2026-10-27 10:11:12.345 INFO history loaded`),
		parseLogEntry(`2026-10-27 10:11:13.345 WARN party anchor lost relay="eu"`),
		parseLogEntry(`2026-10-27 10:11:14.345 ERROR playback failed error="mpv exited"`),
		parseLogEntry(`2026-10-27 10:11:15.345 DEBUG update check started`),
	}

	w.filter = logFilterWarning
	visible := w.visibleEntries()
	if len(visible) != 1 || visible[0].level != logLevelWarning {
		t.Fatalf("warning filter returned %d entries, want 1", len(visible))
	}

	w.filter = logFilterAll
	w.search.SetText("MPV")
	visible = w.visibleEntries()
	if len(visible) != 1 || visible[0].message != "playback failed" {
		t.Fatalf("search over attributes returned %d entries, want the playback error", len(visible))
	}

	w.search.SetText("relay")
	visible = w.visibleEntries()
	if len(visible) != 1 || visible[0].source != "Комната" {
		t.Fatalf("search over detail returned %d entries, want the party warning", len(visible))
	}

	w.search.SetText("")
	w.filter = logFilterInfo
	visible = w.visibleEntries()
	if len(visible) != 2 {
		t.Fatalf("information filter returned %d entries, want 2", len(visible))
	}
	counts := countLogEntries(visible)
	if counts.total != 2 || counts.info != 2 || counts.warning != 0 || counts.error != 0 {
		t.Fatalf("counts = %+v", counts)
	}

	labels := logStatsLabels(logCounts{total: 10, info: 7, warning: 2, error: 1})
	want := [4]string{"10 записей", "7 информационных", "2 предупреждения", "1 ошибка"}
	if labels != want {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
}

func TestPluralRussian(t *testing.T) {
	cases := []struct {
		n    int
		one  string
		few  string
		many string
		want string
	}{
		{1, "запись", "записи", "записей", "запись"},
		{2, "запись", "записи", "записей", "записи"},
		{5, "запись", "записи", "записей", "записей"},
		{11, "запись", "записи", "записей", "записей"},
		{21, "запись", "записи", "записей", "запись"},
		{0, "ошибка", "ошибки", "ошибок", "ошибок"},
		{22, "ошибка", "ошибки", "ошибок", "ошибки"},
	}
	for _, test := range cases {
		if got := pluralRU(test.n, test.one, test.few, test.many); got != test.want {
			t.Errorf("pluralRU(%d) = %q, want %q", test.n, got, test.want)
		}
	}
}

func TestFormatLogDate(t *testing.T) {
	at := time.Date(2026, time.October, 27, 12, 0, 0, 0, time.Local)
	if got := formatLogDate(at); got != "27 октября 2026" {
		t.Fatalf("formatLogDate = %q", got)
	}
}

func TestLogPauseAndClear(t *testing.T) {
	if err := log.Init(""); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Init("") }()

	log.Info("first entry")
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
	w.refreshEntries()
	if len(w.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(w.entries))
	}

	w.togglePause()
	log.Info("second entry")
	w.refreshEntries()
	if len(w.entries) != 1 {
		t.Fatal("a paused log window consumed a new entry")
	}

	w.togglePause()
	if len(w.entries) != 2 {
		t.Fatal("resuming the log window did not load new entries")
	}

	w.clearEntries()
	if len(w.entries) != 0 || len(w.raw) != 0 {
		t.Fatal("clear did not empty the visible entries")
	}
	if len(log.Snapshot()) != 0 {
		t.Fatal("clear did not empty the logger ring")
	}
}

func TestLogErrorRowsAreHighlightedByLevel(t *testing.T) {
	if !isLogError(`2026-10-26 15:04:05.000 ERROR ui error="boom"`) {
		t.Fatal("error line was not recognized")
	}
	if isLogError(`2026-10-26 15:04:05.000 INFO error="boom"`) {
		t.Fatal("an ordinary line mentioning error was treated as an error row")
	}
	if got := logLineColor(`2026-10-26 15:04:05.000 ERROR ui error="boom"`); got != logErrorText {
		t.Fatalf("error color = %v, want %v", got, logErrorText)
	}
	if got := logLineColor(`2026-10-26 15:04:05.000 WARN ui error="boom"`); got != logWarningText {
		t.Fatalf("warning color = %v, want %v", got, logWarningText)
	}
	if got := logLineColor(`2026-10-26 15:04:05.000 INFO ui error="boom"`); got != logInfoText {
		t.Fatalf("info color = %v, want %v", got, logInfoText)
	}
}

func TestLogWindowLayoutAtMinimumContentSize(t *testing.T) {
	if err := log.Init(""); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Init("") }()
	log.Info("history loaded", "count", 3)
	log.Warn("party relay lag", "delay", "1.2s")
	log.Error("playback failed", "error", "mpv exited")

	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
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
