//go:build !windows

package playback

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
)

func configureMPVCommand(*exec.Cmd) {}

func mpvPipePath(name string) string {
	return filepath.Join(os.TempDir(), name+".sock")
}

func (m *mpvProcess) windowRect() (image.Rectangle, bool) {
	return image.Rectangle{}, false
}
