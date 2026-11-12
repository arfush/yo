package kit

import (
	"sync"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/resource"
)

// faceKey identifies one loaded face: family, weight and logical pixel size.
type faceKey struct {
	family string
	weight qt.QFont__Weight
	px     float32
}

var (
	facesOnce    sync.Once
	facesMu      sync.Mutex
	faceFamilies map[*resource.Resource]string
	faceCache    map[faceKey]*qt.QFont
)

// face returns the Qt font for key at size logical pixels. The embedded faces
// are registered with Qt once per resource. The caches are built on first use
// and then only grow; facesMu serializes their readers and writers.
func face(res *resource.Resource, key resource.FontKey, size float32) *qt.QFont {
	if res == nil {
		panic("kit: nil resources")
	}
	if size <= 0 {
		panic("kit: non-positive font size")
	}

	facesOnce.Do(func() {
		faceFamilies = make(map[*resource.Resource]string)
		faceCache = make(map[faceKey]*qt.QFont)
	})

	facesMu.Lock()
	defer facesMu.Unlock()

	family, ok := faceFamilies[res]
	if !ok {
		family = registerFaces(res)
		faceFamilies[res] = family
	}

	id := faceKey{family: family, weight: faceWeight(key), px: size}
	if cached, ok := faceCache[id]; ok {
		return cached
	}
	font := newFace(family, size, id.weight)
	faceCache[id] = font
	return font
}

// registerFaces loads every embedded Inter face into Qt and returns the family
// name Qt assigned to them.
func registerFaces(res *resource.Resource) string {
	keys := [...]resource.FontKey{
		resource.FontRegular,
		resource.FontMedium,
		resource.FontSemiBold,
		resource.FontBold,
	}
	family := ""
	for _, key := range keys {
		id := qt.QFontDatabase_AddApplicationFontFromData(res.Font(key))
		if id < 0 {
			panic("kit: load font " + string(key))
		}
		if families := qt.QFontDatabase_ApplicationFontFamilies(id); len(families) > 0 {
			family = families[0]
		}
	}
	if family == "" {
		panic("kit: no embedded font family")
	}
	return family
}

// faceWeight maps a font key onto the Qt weight of that face.
func faceWeight(key resource.FontKey) qt.QFont__Weight {
	switch key {
	case resource.FontBold:
		return qt.QFont__Bold
	case resource.FontSemiBold:
		return qt.QFont__DemiBold
	case resource.FontMedium:
		return qt.QFont__Medium
	default:
		return qt.QFont__Normal
	}
}

// newFace builds one Inter face in logical pixels. Qt point sizes are 1/72 inch
// while the type scale is in 96-dpi pixels, hence the 0.75 factor.
func newFace(family string, px float32, weight qt.QFont__Weight) *qt.QFont {
	f := qt.NewQFont()
	f.SetFamily(family)
	f.SetPointSizeF(float64(px) * 0.75)
	f.SetWeight(weight)
	f.SetHintingPreference(qt.QFont__PreferNoHinting)
	f.SetStyleStrategy(qt.QFont__PreferAntialias | qt.QFont__PreferQuality)
	return f
}
