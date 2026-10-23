package ui

import (
	"embed"
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
)

//go:embed assets/Inter-Regular.ttf assets/Inter-Medium.ttf assets/Inter-SemiBold.ttf assets/Inter-Bold.ttf
var fontAssets embed.FS

type typography struct {
	*textRenderer
}

// headerFonts returns the Inter faces that mpv's OSD uses for the custom
// window header. The four static faces keep the title bold and the version
// badge medium, matching the application's own chrome.
func headerFonts() map[string][]byte {
	names := []string{
		"Inter-Regular.ttf",
		"Inter-Medium.ttf",
		"Inter-SemiBold.ttf",
		"Inter-Bold.ttf",
	}
	fonts := make(map[string][]byte, len(names))
	for _, name := range names {
		data, err := fontAssets.ReadFile("assets/" + name)
		if err != nil {
			continue
		}
		fonts[name] = data
	}
	return fonts
}

func newTypography() (*typography, error) {
	specs := []struct {
		name   string
		weight font.Weight
	}{
		{"assets/Inter-Regular.ttf", font.Normal},
		{"assets/Inter-Medium.ttf", font.Medium},
		{"assets/Inter-SemiBold.ttf", font.SemiBold},
		{"assets/Inter-Bold.ttf", font.Bold},
	}

	var collection []font.FontFace
	for _, spec := range specs {
		data, err := fontAssets.ReadFile(spec.name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", spec.name, err)
		}
		faces, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", spec.name, err)
		}
		for i := range faces {
			faces[i].Font = font.Font{
				Typeface: fontFamily,
				Style:    font.Regular,
				Weight:   spec.weight,
			}
		}
		collection = append(collection, faces...)
	}

	shaper := text.NewShaper(
		text.NoSystemFonts(),
		text.WithCollection(collection),
	)
	return &typography{
		textRenderer: &textRenderer{shaper: shaper},
	}, nil
}
