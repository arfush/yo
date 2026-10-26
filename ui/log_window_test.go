package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
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
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}

	short := newLogWindow(typo)
	short.syncFollowMode(gtx, []string{"one", "two"}, 400, 300, face)
	if short.list.ScrollToEnd {
		t.Fatal("a short log should start at the top instead of bottom-aligning")
	}

	longLines := make([]string, 100)
	for index := range longLines {
		longLines[index] = "line"
	}
	long := newLogWindow(typo)
	long.syncFollowMode(gtx, longLines, 400, 40, face)
	if !long.list.ScrollToEnd {
		t.Fatal("a log taller than the viewport should open at the newest entry")
	}
	long.list.Position.BeforeEnd = true
	long.syncFollowMode(gtx, longLines, 400, 40, face)
	if long.list.ScrollToEnd {
		t.Fatal("scrolling into older entries should stop following the end")
	}
	long.jumpToEnd()
	long.syncFollowMode(gtx, longLines, 400, 40, face)
	if !long.list.ScrollToEnd {
		t.Fatal("jumpToEnd did not re-enable end-following")
	}
}

func TestLogLineClickExpandsTruncatedRow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	w := newLogWindow(typo)
	line := strings.Repeat("очень длинная строка журнала ", 12)
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
		dims := w.layoutLine(gtx, line, 220, state)
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

func TestLogErrorRowsAreFullyHighlighted(t *testing.T) {
	if !isLogError("2026-10-26 15:04:05.000 ERROR ui error=boom") {
		t.Fatal("error line was not recognized")
	}
	if isLogError("2026-10-26 15:04:05.000 INFO error=boom") {
		t.Fatal("an ordinary line mentioning error was treated as an error row")
	}
	if logLineColor("2026-10-26 15:04:05.000 ERROR ui error=boom") != mainDangerColor {
		t.Fatal("error text does not use the danger color")
	}
}
