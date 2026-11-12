package resource

import (
	"encoding/xml"
	"errors"
	"io/fs"
	"strings"
)

// IconKey is an icon selector: its value is the symbol id in icons.svg.
type IconKey string

// Path is the SVG path data of one icon, in a 24x24 box.
type Path string

// Icon* declares the embedded vector glyphs. Keep this list in sync with the
// symbols in icons.svg: Icon panics on a key that has no symbol.
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

// Icon returns the SVG path for the key. A missing icon panics: the embedded
// catalog is part of the build, not runtime data.
func (r *Resource) Icon(key IconKey) Path {
	path, ok := r.icons[key]
	if !ok {
		panic("resource: icon \"" + string(key) + "\" is not embedded")
	}
	return path
}

type iconSprite struct {
	Symbols []iconSymbol `xml:"symbol"`
}

type iconSymbol struct {
	ID    string     `xml:"id,attr"`
	Paths []iconPath `xml:"path"`
}

type iconPath struct {
	D string `xml:"d,attr"`
}

func (r *Resource) loadIcons(fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "icons.svg")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		panic("resource: read icons.svg: " + err.Error())
	}
	var sprite iconSprite
	if err := xml.Unmarshal(data, &sprite); err != nil {
		panic("resource: parse icons.svg: " + err.Error())
	}
	for _, symbol := range sprite.Symbols {
		var d strings.Builder
		for _, path := range symbol.Paths {
			if d.Len() > 0 {
				d.WriteByte(' ')
			}
			d.WriteString(path.D)
		}
		if d.Len() == 0 {
			panic("resource: icon \"" + symbol.ID + "\" has no path")
		}
		r.icons[IconKey(symbol.ID)] = Path(d.String())
	}
}
