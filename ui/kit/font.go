package kit

import (
	"sync"

	"github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/resource"
)

// faceKey identifies one loaded face: family, weight and logical pixel size.
type faceKey struct {
	family string
	weight qt6.QFont__Weight
	px     float32
}

var (
	facesOnce    sync.Once
	facesMu      sync.Mutex
	faceFamilies map[*resource.Resource]string
	faceCache    map[faceKey]*qt6.QFont
)

// face returns the Qt font for key at size logical pixels. The embedded faces
// are registered with Qt once per resource. The caches are built on first use
// and then only grow; facesMu serializes their readers and writers.
func face(res *resource.Resource, key resource.FontKey, size float32) *qt6.QFont {
	if res == nil {
		panic("kit: nil resources")
	}
	if size <= 0 {
		panic("kit: non-positive font size")
	}

	facesOnce.Do(func() {
		faceFamilies = make(map[*resource.Resource]string)
		faceCache = make(map[faceKey]*qt6.QFont)
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
		id := qt6.QFontDatabase_AddApplicationFontFromData(res.Font(key))
		if id < 0 {
			panic("kit: load font " + string(key))
		}
		if families := qt6.QFontDatabase_ApplicationFontFamilies(id); len(families) > 0 {
			family = families[0]
		}
	}
	if family == "" {
		panic("kit: no embedded font family")
	}
	return family
}

// faceWeight maps a font key onto the Qt weight of that face.
func faceWeight(key resource.FontKey) qt6.QFont__Weight {
	switch key {
	case resource.FontBold:
		return qt6.QFont__Bold
	case resource.FontSemiBold:
		return qt6.QFont__DemiBold
	case resource.FontMedium:
		return qt6.QFont__Medium
	default:
		return qt6.QFont__Normal
	}
}

// newFace builds one Inter face in logical pixels. Qt point sizes are 1/72 inch
// while the type scale is in 96-dpi pixels, hence the 0.75 factor.
func newFace(family string, px float32, weight qt6.QFont__Weight) *qt6.QFont {
	f := qt6.NewQFont()
	f.SetFamily(family)
	f.SetPointSizeF(float64(px) * 0.75)
	f.SetWeight(weight)
	f.SetHintingPreference(qt6.QFont__PreferNoHinting)
	f.SetStyleStrategy(qt6.QFont__PreferAntialias | qt6.QFont__PreferQuality)
	return f
}
