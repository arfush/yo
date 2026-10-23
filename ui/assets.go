package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
)

// appIconPNG is the pre-rendered header variant. The header draws the icon at
// 24dp, so the 48px asset covers 2x DPI natively and still downsamples cleanly
// at 1x.
//
//go:embed assets/app-icon-48.png
var appIconPNG []byte

func loadAppIcon() (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		return nil, err
	}
	return img, nil
}
