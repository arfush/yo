package ui

import (
	"net/url"
	"strings"

	"github.com/arfush/yo/catalog"
)

// librarySectionKey is the settings row that remembers the selected library
// section, so yoanime reopens on the screen the user left.
const librarySectionKey = "library_section"

// librarySection is the header mode of the application. Every history row and
// every loaded title belongs to exactly one section, even though the database
// stores them in one ordered list.
type librarySection int

const (
	sectionAnime librarySection = iota
	sectionManga
	sectionRanobe
)

func sectionFromSetting(value string) librarySection {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manga":
		return sectionManga
	case "ranobe":
		return sectionRanobe
	default:
		return sectionAnime
	}
}

func (s librarySection) key() string {
	switch s {
	case sectionManga:
		return "manga"
	case sectionRanobe:
		return "ranobe"
	default:
		return "anime"
	}
}

func (s librarySection) appName() string {
	switch s {
	case sectionManga:
		return "yomanga"
	case sectionRanobe:
		return "yoranobe"
	default:
		return "yoanime"
	}
}

func sectionForSource(source string) librarySection {
	switch source {
	case catalog.SourceMangaLib, catalog.SourceYaoiLib, catalog.SourceHentaiLib:
		return sectionManga
	case catalog.SourceRanobeLib:
		return sectionRanobe
	default:
		return sectionAnime
	}
}

func (s librarySection) matchesSource(source string) bool {
	return sectionForSource(source) == s
}

func (t historyTitle) section() librarySection {
	return sectionForSource(t.source)
}

// setSection switches the visible library without touching the other sections.
// Selection and scroll belong to the previous list and are dropped.
func (m *mainScreen) setSection(section librarySection) {
	if m.section == section {
		return
	}
	m.section = section
	m.applyFilter()
}

// loadInput turns a saved reading row back into a full site URL. Library.Load
// needs the host to tell LibSocial titles apart from AnimeLIB slugs.
func (t historyTitle) loadInput() string {
	if !t.reading() {
		return t.slug
	}
	host := ""
	switch t.section() {
	case sectionManga:
		switch t.source {
		case catalog.SourceYaoiLib:
			host = "slashlib.me"
		case catalog.SourceHentaiLib:
			host = "hentailib.me"
		default:
			host = "mangalib.me"
		}
	case sectionRanobe:
		host = "ranobelib.me"
	}
	if host == "" || strings.TrimSpace(t.slug) == "" {
		return t.slug
	}
	return "https://" + host + "/manga/" + url.PathEscape(t.slug)
}
