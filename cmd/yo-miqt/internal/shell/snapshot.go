package shell

import (
	"image"

	"yo-miqt/internal/snapshot"
)

// Snapshot renders the widget offscreen at one pixel per logical pixel.
func (s *Shell) Snapshot() (*image.NRGBA, error) {
	return snapshot.Capture(s.widget, snapshot.Options{Scale: 1})
}
