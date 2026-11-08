package resource

import (
	"io/fs"
	"strings"

	"gioui.org/font"
	"gioui.org/font/opentype"
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

// Font returns the cached font face for the key. A missing font panics: the
// embedded catalog is part of the build, not runtime data.
func (r *Resource) Font(key FontKey) font.FontFace {
	face, ok := r.fonts[key]
	if !ok {
		panic("resource: font \"" + string(key) + "\" is not embedded")
	}
	return face
}

func (r *Resource) loadFonts(fsys fs.FS) {
	for _, entry := range readDir(fsys, "font") {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ttf") {
			continue
		}
		key := FontKey(strings.TrimSuffix(entry.Name(), ".ttf"))
		weight, ok := fontWeight(key)
		if !ok {
			panic("resource: font/" + entry.Name() + " has no FontKey declared")
		}
		data := readFile(fsys, "font/"+entry.Name())
		r.fonts[key] = mustParseFont(data, string(key), weight)
	}
}

// fontWeight maps a declared font key to its Gio weight.
func fontWeight(key FontKey) (font.Weight, bool) {
	switch key {
	case FontRegular:
		return font.Normal, true
	case FontMedium:
		return font.Medium, true
	case FontSemiBold:
		return font.SemiBold, true
	case FontBold:
		return font.Bold, true
	default:
		return 0, false
	}
}

func mustParseFont(data []byte, name string, weight font.Weight) font.FontFace {
	face, err := opentype.Parse(data)
	if err != nil {
		panic("resource: parse font " + name + ": " + err.Error())
	}
	parsed := face.Font()
	return font.FontFace{
		Font: font.Font{
			Typeface: parsed.Typeface,
			Style:    parsed.Style,
			Weight:   weight,
		},
		Face: face,
	}
}
