package resource

import (
	"io/fs"
	"strings"
)

// FontKey is a font selector: its value is the file name in font/<name>.ttf.
type FontKey string

// Font* declares the embedded Inter faces.
const (
	FontRegular  FontKey = "Inter-Regular"
	FontMedium   FontKey = "Inter-Medium"
	FontSemiBold FontKey = "Inter-SemiBold"
	FontBold     FontKey = "Inter-Bold"
)

// Font returns the raw TTF bytes for the key. A missing font panics: the
// embedded catalog is part of the build, not runtime data. The slice is shared
// and must not be modified.
func (r *Resource) Font(key FontKey) []byte {
	data, ok := r.fonts[key]
	if !ok {
		panic("resource: font \"" + string(key) + "\" is not embedded")
	}
	return data
}

func (r *Resource) loadFonts(fsys fs.FS) {
	for _, entry := range readDir(fsys, "font") {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ttf") {
			continue
		}
		key := FontKey(strings.TrimSuffix(entry.Name(), ".ttf"))
		if !fontDeclared(key) {
			panic("resource: font/" + entry.Name() + " has no FontKey declared")
		}
		r.fonts[key] = readFile(fsys, "font/"+entry.Name())
	}
}

// fontDeclared reports whether key names one of the embedded faces.
func fontDeclared(key FontKey) bool {
	switch key {
	case FontRegular, FontMedium, FontSemiBold, FontBold:
		return true
	default:
		return false
	}
}
