package resource

import (
	"bytes"
	"image"
	_ "image/png"
	"io/fs"
	"strings"
)

// AppIconKey is an application icon selector: its value is the base file name
// in icon/<name>.png and icon/<name>.ico.
type AppIconKey string

// AppIcon* declares the embedded application icons.
const (
	AppIconYobase   AppIconKey = "yobase-icon"
	AppIconYoanime  AppIconKey = "yoanime-icon"
	AppIconYomanga  AppIconKey = "yomanga-icon"
	AppIconYoranobe AppIconKey = "yoranobe-icon"
)

// appIcon holds both representations of one application icon: the decoded PNG
// used by native window APIs and the raw ICO used as the executable resource.
type appIcon struct {
	image image.Image
	ico   []byte
}

// AppIcon returns the decoded PNG application icon. The image is shared and
// must not be modified. A missing icon panics.
func (r *Resource) AppIcon(key AppIconKey) image.Image {
	icon, ok := r.appIcons[key]
	if !ok || icon.image == nil {
		panic("resource: app icon \"" + string(key) + "\" is not embedded")
	}
	return icon.image
}

// AppIconICO returns a copy of the raw ICO bytes for the key. A missing icon
// panics.
func (r *Resource) AppIconICO(key AppIconKey) []byte {
	icon, ok := r.appIcons[key]
	if !ok || len(icon.ico) == 0 {
		panic("resource: app icon \"" + string(key) + "\" is not embedded")
	}
	out := make([]byte, len(icon.ico))
	copy(out, icon.ico)
	return out
}

func (r *Resource) loadAppIcons(fsys fs.FS) {
	for _, entry := range readDir(fsys, "icon") {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, ".png"):
			key := AppIconKey(strings.TrimSuffix(name, ".png"))
			icon := r.appIcons[key]
			icon.image = mustDecodeImage(readFile(fsys, "icon/"+name), name)
			r.appIcons[key] = icon
		case strings.HasSuffix(name, ".ico"):
			key := AppIconKey(strings.TrimSuffix(name, ".ico"))
			icon := r.appIcons[key]
			icon.ico = readFile(fsys, "icon/"+name)
			r.appIcons[key] = icon
		}
	}
}

func mustDecodeImage(data []byte, name string) image.Image {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic("resource: decode " + name + ": " + err.Error())
	}
	return img
}
