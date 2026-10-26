package ui

import (
	"image"

	"github.com/arfush/yonime/ui/assets"
)

func loadAppIcon() (image.Image, error) {
	return assets.AppIcon()
}
