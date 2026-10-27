package ui

import (
	"image"

	"github.com/arfush/yoanime/ui/assets"
)

func loadAppIcon() (image.Image, error) {
	return assets.AppIcon()
}
