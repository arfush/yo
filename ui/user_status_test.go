package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/library"
)

func TestUserStatusLabel(t *testing.T) {
	tests := []struct {
		status library.UserStatus
		want   string
	}{
		{0, "В планах"},
		{library.UserStatusWatching, "Смотрю"},
		{library.UserStatusPlanned, "В планах"},
		{library.UserStatusFavorite, "Обожаю"},
		{library.UserStatusCompleted, "Досмотрел"},
		{library.UserStatusDropped, "Брошено"},
		{library.UserStatus(99), "В планах"},
	}
	for _, test := range tests {
		if got := userStatusLabel(test.status); got != test.want {
			t.Fatalf("userStatusLabel(%d) = %q, want %q", test.status, got, test.want)
		}
	}
}

func TestUserStatusFilterMatches(t *testing.T) {
	if !filterAll.matches(0) || !filterAll.matches(library.UserStatusDropped) {
		t.Fatal("filterAll must match every status")
	}
	if !filterPlanned.matches(0) {
		t.Fatal("filterPlanned must match the default status")
	}
	if filterWatching.matches(library.UserStatusPlanned) {
		t.Fatal("filterWatching matched a planned title")
	}
	if !filterCompleted.matches(library.UserStatusCompleted) {
		t.Fatal("filterCompleted did not match a completed title")
	}
}

func TestUserStatusFilterOrder(t *testing.T) {
	want := []string{"Все", "Смотрю", "В планах", "Обожаю", "Досмотрел", "Брошено"}
	if len(userStatusFilterOptions) != len(want) {
		t.Fatalf("filter count = %d, want %d", len(userStatusFilterOptions), len(want))
	}
	for index, label := range want {
		if got := userStatusFilterOptions[index].label; got != label {
			t.Fatalf("filter %d = %q, want %q", index, got, label)
		}
	}
}

func TestUserStatusMenuOptionsSkipCurrent(t *testing.T) {
	tests := []struct {
		name    string
		current library.UserStatus
		want    []library.UserStatus
	}{
		{
			name:    "watching",
			current: library.UserStatusWatching,
			want: []library.UserStatus{
				library.UserStatusPlanned,
				library.UserStatusFavorite,
				library.UserStatusCompleted,
				library.UserStatusDropped,
			},
		},
		{
			name:    "unknown normalizes to planned",
			current: library.UserStatus(99),
			want: []library.UserStatus{
				library.UserStatusWatching,
				library.UserStatusFavorite,
				library.UserStatusCompleted,
				library.UserStatusDropped,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := userStatusMenuOptionsFor(test.current)
			if len(got) != len(test.want) {
				t.Fatalf("option count = %d, want %d", len(got), len(test.want))
			}
			for index := range test.want {
				if got[index] != test.want[index] {
					t.Fatalf("option %d = %d, want %d", index, got[index], test.want[index])
				}
			}
		})
	}
}

func TestPlaceUserStatusMenuFlipsAbove(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(100, 520, 180, 543)
	menu := placeUserStatusMenu(anchor, 120, 100, 7, 8, bounds)
	if menu.Max.Y > bounds.Max.Y-8 {
		t.Fatalf("menu = %v, want it inside the bottom margin", menu)
	}
	if menu.Max.Y >= anchor.Min.Y {
		t.Fatalf("menu = %v, want it above anchor %v", menu, anchor)
	}
}

func TestPlaceUserStatusMenuCentersOnAnchor(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 600)
	anchor := image.Rect(100, 100, 180, 123)
	menu := placeUserStatusMenu(anchor, 120, 100, 7, 8, bounds)
	anchorCenter := anchor.Min.X + anchor.Dx()/2
	menuCenter := menu.Min.X + menu.Dx()/2
	if menuCenter != anchorCenter {
		t.Fatalf("menu center = %d, want anchor center %d (menu %v, anchor %v)", menuCenter, anchorCenter, menu, anchor)
	}
	if menu.Min.Y <= anchor.Max.Y {
		t.Fatalf("menu = %v, want it below anchor %v", menu, anchor)
	}
}

// TestPillLabelsShareBaseline guards the pill text alignment: labels with
// accents, descenders or round overshoots must not shift relative to labels
// made only of flat capitals.
func TestPillLabelsShareBaseline(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	labels := []string{
		"Смотрю", "В планах", "Обожаю", "Досмотрел", "Брошено", "Все",
		"Завершён", "Онгоинг", "Анонс",
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		gtx := badgeTestContext(scale)
		_, refInk := typo.measure(gtx, "Н", face, mainStatusSize)
		if refInk.Empty() {
			t.Fatalf("scale %v: reference cap is empty", scale)
		}
		capHeight := refInk.Dy()
		var want float32
		for index, label := range labels {
			glyphs, _ := typo.measure(gtx, label, face, mainStatusSize)
			if len(glyphs) == 0 {
				t.Fatalf("scale %v: %q has no glyphs", scale, label)
			}
			got := baselineCenteredOffset(20, int(glyphs[0].Y), capHeight)
			if index == 0 {
				want = got
				continue
			}
			if got != want {
				t.Fatalf("scale %v: baseline offset for %q = %.2f, want %.2f", scale, label, got, want)
			}
		}
	}
}

// TestOverlayScrimReportsClick covers the menu dismissal path: the scrim must
// see a click even though Layout is called on the same frame to register it.
func TestOverlayScrimReportsClick(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Constraints: layout.Exact(image.Pt(120, 80)),
	}
	var click widget.Clickable
	frame := func() bool {
		ops.Reset()
		clicked := layoutOverlayScrim(gtx, image.Rect(0, 0, 120, 80), &click)
		router.Frame(ops)
		return clicked
	}
	if frame() {
		t.Fatal("scrim reported a click before any pointer event")
	}
	router.Queue(
		pointer.Event{
			Kind:      pointer.Press,
			Source:    pointer.Mouse,
			PointerID: 1,
			Buttons:   pointer.ButtonPrimary,
			Position:  f32.Pt(60, 40),
		},
		pointer.Event{
			Kind:      pointer.Release,
			Source:    pointer.Mouse,
			PointerID: 1,
			Position:  f32.Pt(60, 40),
		},
	)
	if !frame() {
		t.Fatal("scrim did not report an outside click")
	}
}

// TestUserStatusMenuSelectsOption guards the option hit areas: a click on a
// status must reach the menu rather than the scrim underneath it.
func TestUserStatusMenuSelectsOption(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(300, 300)),
	}
	anchor := image.Rect(10, 10, 80, 32)
	menu := userStatusMenu{}
	menu.show(anchor, library.UserStatusWatching, 7, 70, 22)
	var selected library.UserStatus
	frame := func() {
		ops.Reset()
		menu.layout(gtx, typo, image.Point{}, func(status library.UserStatus) {
			selected = status
		})
		router.Frame(ops)
	}
	frame()

	menuRect := placeUserStatusMenu(
		anchor,
		70+2*gtx.Dp(userStatusMenuPad),
		4*22+2*gtx.Dp(userStatusMenuPad)+3*gtx.Dp(userStatusMenuGap),
		gtx.Dp(userStatusMenuDrop),
		gtx.Dp(unit.Dp(8)),
		image.Rectangle{Max: gtx.Constraints.Max},
	)
	option := image.Rect(
		menuRect.Min.X+gtx.Dp(userStatusMenuPad),
		menuRect.Min.Y+gtx.Dp(userStatusMenuPad),
		menuRect.Min.X+gtx.Dp(userStatusMenuPad)+70,
		menuRect.Min.Y+gtx.Dp(userStatusMenuPad)+22,
	)
	center := f32.Pt(float32(option.Min.X+option.Dx()/2), float32(option.Min.Y+option.Dy()/2))
	router.Queue(
		pointer.Event{
			Kind:      pointer.Press,
			Source:    pointer.Mouse,
			PointerID: 1,
			Buttons:   pointer.ButtonPrimary,
			Position:  center,
		},
		pointer.Event{
			Kind:      pointer.Release,
			Source:    pointer.Mouse,
			PointerID: 1,
			Position:  center,
		},
	)
	frame()
	if selected != library.UserStatusPlanned {
		t.Fatalf("selected status = %d, want planned", selected)
	}
	if menu.open {
		t.Fatal("menu stayed open after a status was selected")
	}
}

func TestEscapeClosesHistoryStatusMenu(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	m := &mainWindow{
		selected:   2,
		statusMenu: userStatusMenu{open: true},
	}
	frame := func() {
		ops.Reset()
		m.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()
	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	if m.statusMenu.open {
		t.Fatal("Escape did not close the history status menu")
	}
	if m.selected != 2 {
		t.Fatalf("Escape also ran its fallback handler, selected = %d", m.selected)
	}
}

func TestEscapeClosesTitleStatusMenuBeforeBack(t *testing.T) {
	var router input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: router.Source()}
	back := false
	w := &titleWindow{
		statusMenu: userStatusMenu{open: true},
		onBack:     func() { back = true },
	}
	frame := func() {
		ops.Reset()
		w.handleKeys(gtx)
		router.Frame(ops)
	}
	frame()
	router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	if w.statusMenu.open {
		t.Fatal("Escape did not close the title status menu")
	}
	if back {
		t.Fatal("Escape left the title page instead of only closing the menu")
	}
}
