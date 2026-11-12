package resource

import (
	"embed"
	"errors"
	"io/fs"
	"sync"
)

// assets is the embedded resource tree. The placeholders keep font/ and icon/
// present while their assets are added; loaders skip files they do not own.
//
//go:embed all:font all:icon lang/*.json
var assets embed.FS

// Resource owns the embedded assets loaded by New. It is safe to share across
// the UI goroutine but must not be copied.
type Resource struct {
	fonts    map[FontKey][]byte
	icons    map[IconKey]Path
	appIcons map[AppIconKey]appIcon
	lexicons map[string]lexicon

	// mu guards next, the per-key cursors for sequential translations. The
	// asset maps are read-only after New.
	mu   sync.Mutex
	next map[variantCursor]uint64
}

// New loads the embedded assets and validates the built-in languages. It
// panics when an asset is broken: that is a build invariant, not a runtime case.
func New() *Resource {
	r := newResource(assets)
	for _, code := range []string{"en", "ru"} {
		if _, ok := r.lexicons[code]; !ok {
			panic("resource: embedded lang \"" + code + "\" is missing")
		}
	}
	return r
}

// newResource loads assets from fsys. Tests pass a small fake filesystem; New
// passes the embedded one.
func newResource(fsys fs.FS) *Resource {
	r := &Resource{
		fonts:    make(map[FontKey][]byte),
		icons:    make(map[IconKey]Path),
		appIcons: make(map[AppIconKey]appIcon),
		lexicons: make(map[string]lexicon),
		next:     make(map[variantCursor]uint64),
	}
	r.loadFonts(fsys)
	r.loadIcons(fsys)
	r.loadAppIcons(fsys)
	r.loadLangs(fsys)
	return r
}

// readDir returns the entries of dir or nil when the directory is absent. The
// embedded tree always has the directories; tests may omit them.
func readDir(fsys fs.FS, dir string) []fs.DirEntry {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		panic("resource: read dir " + dir + ": " + err.Error())
	}
	return entries
}

func readFile(fsys fs.FS, path string) []byte {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		panic("resource: read " + path + ": " + err.Error())
	}
	return data
}
