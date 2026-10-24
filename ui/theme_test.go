package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestFooterKeepsSettingsVisibleNextToUpdate(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 545)),
	}

	g := makeGeometry(gtx, false, true, true)
	if g.settingsRect.Empty() {
		t.Fatal("settings button is missing when an update is available")
	}
	if g.updateRect.Empty() {
		t.Fatal("update button is missing when an update is available")
	}
	if g.settingsRect.Min.X <= g.updateRect.Max.X {
		t.Fatalf("settings must stay to the right of update: settings=%v update=%v", g.settingsRect, g.updateRect)
	}

	g = makeGeometry(gtx, false, true, false)
	if g.settingsRect.Empty() {
		t.Fatal("settings button is missing without an update")
	}
	if !g.updateRect.Empty() {
		t.Fatalf("update button must stay hidden without an update: %v", g.updateRect)
	}
}
