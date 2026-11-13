package resource

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mappu/miqt/qt6"
)

func TestParsePathAcceptsSpriteCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    string
	}{
		{"absolute", "M0 0 L24 0 L24 24 Z"},
		{"horizontal-vertical", "M1 2 H10 V20 Z"},
		{"cubic", "M1 1 C2 2 3 3 4 4 Z"},
		{"implicit-lineto", "M0 0 1 1 2 2"},
		{"relative", "m1 1 l2 0 l0 2 z"},
		{"subpaths", "M0 0 L1 0 Z M2 2 L3 2 Z"},
		{"exponent", "M1e1 1 L2e1 1"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, err := parsePath(tt.d)
			if err != nil {
				t.Fatalf("parsePath(%q): %v", tt.d, err)
			}
			defer path.Delete()
			if path.IsEmpty() {
				t.Fatalf("parsePath(%q) is empty", tt.d)
			}
			if got := path.FillRule(); got != qt6.WindingFill {
				t.Fatalf("parsePath(%q).FillRule() = %v; want WindingFill", tt.d, got)
			}
		})
	}
}

func TestParsePathBounds(t *testing.T) {
	t.Parallel()

	path, err := parsePath("M0 0 L24 0 L24 24 Z")
	if err != nil {
		t.Fatalf("parsePath: %v", err)
	}
	defer path.Delete()

	bounds := path.BoundingRect()
	if bounds.Left() != 0 || bounds.Top() != 0 ||
		bounds.Right() != IconBox || bounds.Bottom() != IconBox {
		t.Fatalf("bounds = (%g,%g)-(%g,%g); want (0,0)-(%d,%d)",
			bounds.Left(), bounds.Top(), bounds.Right(), bounds.Bottom(), IconBox, IconBox)
	}
}

func TestParsePathRejectsInvalidData(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"L0 0",
		"M0",
		"M0 0 L",
		"M0 0 L1",
		"M0 0 Z 1",
		"M0 0 A1 1 0 0 0 2 2",
		"M0 0 Q1 1 2 2",
		"M0 0 X",
		"M0 0 L1 1 #",
		"M0 0 L1e 1",
		"M0 0 L+",
		"M0 0 C1 2 3 4 5",
	}
	for _, d := range tests {
		path, err := parsePath(d)
		if err == nil {
			path.Delete()
			t.Errorf("parsePath(%q) = nil error; want error", d)
		}
	}
}

func TestEmbeddedIcons(t *testing.T) {
	t.Parallel()

	r := New()
	entries, err := fs.ReadDir(assets, "icon")
	if err != nil {
		t.Fatalf("read icon dir: %v", err)
	}
	seen := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".svg") {
			continue
		}
		key := IconKey(strings.TrimSuffix(name, ".svg"))
		path := r.IconPath(key)
		if path == nil || path.IsEmpty() {
			t.Fatalf("IconPath(%q) is empty", key)
		}
		bounds := path.BoundingRect()
		if bounds.Left() < -1 || bounds.Top() < -1 ||
			bounds.Right() > IconBox+1 || bounds.Bottom() > IconBox+1 {
			t.Fatalf("IconPath(%q) bounds = (%g,%g)-(%g,%g); want inside IconBox±1",
				key, bounds.Left(), bounds.Top(), bounds.Right(), bounds.Bottom())
		}
		seen++
	}
	if seen == 0 {
		t.Fatal("no embedded icons")
	}
}

func TestLoadIconsRejectsUnsupportedCommand(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"icon/bad.svg": {Data: []byte(`<svg><path d="M0 0 A1 1 0 0 0 2 2"/></svg>`)},
	}
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("newResource did not panic")
		}
		msg, ok := got.(string)
		if !ok || !strings.Contains(msg, `icon "bad"`) {
			t.Fatalf("panic = %v; want the icon name", got)
		}
	}()
	newResource(fsys)
}

func TestIconPathReturnsSharedPath(t *testing.T) {
	t.Parallel()

	r := New()
	first := r.IconPath(IconClose)
	if first == nil || first.IsEmpty() {
		t.Fatal("IconPath returned an empty path")
	}
	if got := r.IconPath(IconClose); got != first {
		t.Fatal("IconPath returned a different path for the same key")
	}
}
