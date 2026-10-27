package ui

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"

	"github.com/arfush/yoanime/ui/assets"
)

type typography struct {
	*textRenderer
}

func newTypography() (*typography, error) {
	specs := []struct {
		name   string
		weight font.Weight
	}{
		{"Inter-Regular.ttf", font.Normal},
		{"Inter-Medium.ttf", font.Medium},
		{"Inter-SemiBold.ttf", font.SemiBold},
		{"Inter-Bold.ttf", font.Bold},
	}

	var collection []font.FontFace
	for _, spec := range specs {
		data, err := assets.ReadFont(spec.name)
		if err != nil {
			return nil, err
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
