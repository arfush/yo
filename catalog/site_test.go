package catalog_test

import (
	"testing"

	"github.com/arfush/yo/catalog"
)

func TestSites(t *testing.T) {
	t.Parallel()

	want := []catalog.Site{catalog.SiteYoanime, catalog.SiteYomanga, catalog.SiteYoranobe}
	got := catalog.Sites()
	if len(got) != len(want) {
		t.Fatalf("Sites() returned %d sites; want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Sites()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestSiteString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		site catalog.Site
		want string
	}{
		{catalog.SiteYoanime, "yoanime"},
		{catalog.SiteYomanga, "yomanga"},
		{catalog.SiteYoranobe, "yoranobe"},
		{catalog.Site("unknown"), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.site.String(); got != tt.want {
			t.Fatalf("Site(%q).String() = %q; want %q", tt.site, got, tt.want)
		}
	}
}
