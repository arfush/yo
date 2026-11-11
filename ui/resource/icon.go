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
	IconMaximize   IconKey = "maximize"
	IconMinimize   IconKey = "minimize"
	IconTorii      IconKey = "torii"
	IconUnmaximize IconKey = "unmaximize"

	// Variants of pack glyphs for a smaller optical box. The pack draws on a
	// 24x24 grid with a 1.6px stroke; these keep the stroke and shrink only
	// the glyph, because design.svg specifies the smaller box for them.
	IconCloseDialog    IconKey = "close-dialog"
	IconDownloadButton IconKey = "download-button"

	IconArrowRight        IconKey = "arrow-right"
	IconArrowUpRight      IconKey = "arrow-up-right"
	IconBell              IconKey = "bell"
	IconBookOpen          IconKey = "book-open"
	IconBookmark          IconKey = "bookmark"
	IconCalendarClock     IconKey = "calendar-clock"
	IconChevronDown       IconKey = "chevron-down"
	IconChevronLeft       IconKey = "chevron-left"
	IconCopy              IconKey = "copy"
	IconDownload          IconKey = "download"
	IconGlobe             IconKey = "globe"
	IconGripVertical      IconKey = "grip-vertical"
	IconHistory           IconKey = "history"
	IconKeyRound          IconKey = "key-round"
	IconLink              IconKey = "link"
	IconLockKeyhole       IconKey = "lock-keyhole"
	IconLockOpen          IconKey = "lock-open"
	IconNotebookText      IconKey = "notebook-text"
	IconPlay              IconKey = "play"
	IconPlayFilled        IconKey = "play-filled"
	IconPlus              IconKey = "plus"
	IconRefreshCW         IconKey = "refresh-cw"
	IconSkipForward       IconKey = "skip-forward"
	IconSlidersHorizontal IconKey = "sliders-horizontal"
	IconTrash             IconKey = "trash"
	IconTrash2            IconKey = "trash-2"
	IconUsers             IconKey = "users"
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
