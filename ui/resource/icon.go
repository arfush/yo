package resource

import (
	"encoding/xml"
	"io/fs"
	"strings"

	"github.com/mappu/miqt/qt6"
)

// IconKey is an icon selector: its value is the SVG file name in icon/
// without the extension.
type IconKey string

// IconBox is the side of the square coordinate box every embedded vector icon
// is authored in.
const IconBox = 24

// Icon* declares the embedded vector glyphs. Keep this list in sync with the
// SVG files in icon/: IconPath panics on a key that has no file.
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

// IconPath returns the vector glyph for key, parsed once by New. The path is
// authored in an IconBox x IconBox coordinate box and is shared: callers must
// not mutate or delete it, and must draw it only on the UI goroutine. A
// missing icon panics: the embedded catalog is part of the build, not runtime
// data.
func (r *Resource) IconPath(key IconKey) *qt6.QPainterPath {
	path, ok := r.icons[key]
	if !ok || path == nil {
		panic("resource: icon \"" + string(key) + "\" is not embedded")
	}
	return path
}

// svgFile is one SVG file in icon/: the path elements of a single glyph.
type svgFile struct {
	Paths []svgPathElement `xml:"path"`
}

type svgPathElement struct {
	D string `xml:"d,attr"`
}

func (r *Resource) loadIcons(fsys fs.FS) {
	for _, entry := range readDir(fsys, "icon") {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".svg") {
			continue
		}
		key := IconKey(strings.TrimSuffix(name, ".svg"))
		var file svgFile
		if err := xml.Unmarshal(readFile(fsys, "icon/"+name), &file); err != nil {
			panic("resource: parse icon/" + name + ": " + err.Error())
		}
		var d strings.Builder
		for _, element := range file.Paths {
			if d.Len() > 0 {
				d.WriteByte(' ')
			}
			d.WriteString(element.D)
		}
		if d.Len() == 0 {
			panic("resource: icon \"" + string(key) + "\" has no path")
		}
		path, err := parsePath(d.String())
		if err != nil {
			panic("resource: icon \"" + string(key) + "\": " + err.Error())
		}
		r.icons[key] = path
	}
}
