package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestSettingsWindowLayoutDoesNotOverflow(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		settings := newSettingsWindow(typo, enabled)
		settings.testMessage = "Отправлено: 3"

		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(833, 442)),
		}
		gtx = gtx.Disabled()
		settings.layout(gtx)

		footer := gtx
		footer.Constraints = layout.Exact(image.Pt(833, 49))
		settings.layoutFooter(footer)
	}
}
