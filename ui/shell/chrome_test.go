package shell

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/core/stdx"
	"github.com/arfush/yo/ui/appearance"
	"github.com/arfush/yo/ui/resource"
)

func TestSiteAnchorsStayStatic(t *testing.T) {
	t.Parallel()

	apps := stdx.NewHolder(appearance.New(appearance.Options{
		Resource: resource.New(),
		Language: "ru",
	}))
	site := stdx.NewHolder(catalog.SiteYoanime)
	shell := New(Options{Screen: testScreen{}, Site: site, Appearance: apps})
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(833, 545)},
	}
	g := shell.newGeometry(gtx)
	first := shell.brandLayout(gtx, g)

	site.Update(func(v *catalog.Site) { *v = catalog.SiteYoranobe })
	second := shell.brandLayout(gtx, g)

	if first.chevron != second.chevron {
		t.Fatalf("chevron moved from %v to %v", first.chevron, second.chevron)
	}
	if first.versionX != second.versionX {
		t.Fatalf("version x moved from %d to %d", first.versionX, second.versionX)
	}
	if got, want := first.trigger.Max.X, first.chevron.Max.X+4; got != want {
		t.Fatalf("menu anchor right = %d; want chevron right + overhang %d", got, want)
	}
}
