package ui

import (
	"image"

	"github.com/arfush/yo/ui/assets"
)

func loadAppIcon() (image.Image, error) {
	return assets.AppIcon()
}

// loadSectionIcons takes the shared application icon and rotates its violet
// hue for the manga and ranobe sections. Keeping one source image guarantees
// that the logo and the shape of the artwork stay identical everywhere.
func loadSectionIcons() (map[librarySection]image.Image, error) {
	anime, err := assets.AppIcon()
	if err != nil {
		return nil, err
	}
	icons := map[librarySection]image.Image{sectionAnime: anime}
	for _, section := range []librarySection{sectionManga, sectionRanobe} {
		icons[section] = recolorPurpleIcon(anime, section.palette().accent1)
	}
	return icons, nil
}
