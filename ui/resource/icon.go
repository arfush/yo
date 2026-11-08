package resource

import (
	"io/fs"
	"strings"

	"gioui.org/widget"
)

// IconKey is an icon selector: its value is the file name in
// icon/<name>.iconvg.
type IconKey string

// Icon* declares the embedded vector glyphs.
const (
	IconBell         IconKey = "bell"
	IconCheck        IconKey = "check"
	IconChevron      IconKey = "chevron"
	IconChevronHeavy IconKey = "chevron_heavy"
	IconChevronLeft  IconKey = "chevron_left"
	IconChevronRight IconKey = "chevron_right"
	IconChevronThick IconKey = "chevron_thick"
	IconChevronThin  IconKey = "chevron_thin"
	IconChevronUp    IconKey = "chevron_up"
	IconClose        IconKey = "close"
	IconConfig       IconKey = "config"
	IconCopy         IconKey = "copy"
	IconKeyboard     IconKey = "keyboard"
	IconLink         IconKey = "link"
	IconLog          IconKey = "log"
	IconMaximize     IconKey = "maximize"
	IconMinimize     IconKey = "minimize"
	IconPause        IconKey = "pause"
	IconPeople       IconKey = "people"
	IconPlay         IconKey = "play"
	IconScroll       IconKey = "scroll"
	IconSearch       IconKey = "search"
	IconTorii        IconKey = "torii"
	IconTrash        IconKey = "trash"
	IconUndo         IconKey = "undo"
	IconUnmaximize   IconKey = "unmaximize"
	IconUpdate       IconKey = "update"
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
