package resource

import (
	"io/fs"
	"strings"

	"gioui.org/widget"
)

// IconKey is an icon selector: its value is the file name in
// icon/<name>.iconvg.
type IconKey string

// Icon* declares the embedded vector glyphs. Keep this list in sync with the
// .iconvg files in icon/: Icon panics on a key that has no asset.
const (
	IconClose      IconKey = "close"
	IconDownload   IconKey = "download"
	IconMaximize   IconKey = "maximize"
	IconMinimize   IconKey = "minimize"
	IconTorii      IconKey = "torii"
	IconUnmaximize IconKey = "unmaximize"
)

// Icon returns the cached vector icon for the key. A missing icon panics: the
// embedded catalog is part of the build, not runtime data.
func (r *Resource) Icon(key IconKey) *widget.Icon {
	icon, ok := r.icons[key]
	if !ok {
		panic("resource: icon \"" + string(key) + "\" is not embedded")
	}
	return icon
}

func (r *Resource) loadIcons(fsys fs.FS) {
	for _, entry := range readDir(fsys, "icon") {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".iconvg") {
			continue
		}
		key := IconKey(strings.TrimSuffix(entry.Name(), ".iconvg"))
		icon, err := widget.NewIcon(readFile(fsys, "icon/"+entry.Name()))
		if err != nil {
			panic("resource: build icon " + entry.Name() + ": " + err.Error())
		}
		r.icons[key] = icon
	}
}
