package ui

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	xdraw "golang.org/x/image/draw"
)

var posterImages = struct {
	sync.Mutex
	byPath map[string]image.Image
}{
	byPath: make(map[string]image.Image),
}

func loadPosterImage(path string) image.Image {
	if path == "" {
		return nil
	}
	posterImages.Lock()
	if img, ok := posterImages.byPath[path]; ok {
		posterImages.Unlock()
		return img
	}
	posterImages.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}

	posterImages.Lock()
	posterImages.byPath[path] = img
	posterImages.Unlock()
	return img
}

// posterFitKey identifies a cover resized for one concrete pixel rectangle.
type posterFitKey struct {
	path          string
	width, height int
}

var posterFits = struct {
	sync.Mutex
	byKey map[posterFitKey]image.Image
}{
	byKey: make(map[posterFitKey]image.Image),
}

// posterForRect returns the cover cropped and resampled to the pixel size of
// the target rectangle. Gio only samples textures bilinearly, so drawing a
// 460x650 JPEG inside a 38x48dp row makes it look mushy and aliased. Scaling
// once with CatmullRom keeps the preview sharp and avoids resampling the full
// cover on every frame. The result is cached per pixel size because the same
// cover is often drawn by several rows at once.
func posterForRect(path string, width, height int) image.Image {
	if path == "" || width <= 0 || height <= 0 {
		return nil
	}
	key := posterFitKey{path: path, width: width, height: height}
	posterFits.Lock()
	if img, ok := posterFits.byKey[key]; ok {
		posterFits.Unlock()
		return img
	}
	posterFits.Unlock()

	src := loadPosterImage(path)
	if src == nil {
		return nil
	}
	if src.Bounds().Empty() {
		return nil
	}

	cropped := cropPoster(src, width, height)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), cropped, cropped.Bounds(), xdraw.Src, nil)

	posterFits.Lock()
	posterFits.byKey[key] = dst
	posterFits.Unlock()
	return dst
}

// cropPoster trims the source to the target aspect ratio so the subsequent
// resize does not have to stretch the artwork. The crop is centered, matching
// the cover fill used by drawImageCoverRounded.
func cropPoster(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	sourceAspect := float64(bounds.Dx()) / float64(bounds.Dy())
	targetAspect := float64(width) / float64(height)
	var cropped image.Rectangle
	if sourceAspect > targetAspect {
		croppedWidth := int(math.Round(float64(bounds.Dy()) * targetAspect))
		croppedWidth = min(max(croppedWidth, 1), bounds.Dx())
		x := bounds.Min.X + (bounds.Dx()-croppedWidth)/2
		cropped = image.Rect(x, bounds.Min.Y, x+croppedWidth, bounds.Max.Y)
	} else {
		croppedHeight := int(math.Round(float64(bounds.Dx()) / targetAspect))
		croppedHeight = min(max(croppedHeight, 1), bounds.Dy())
		y := bounds.Min.Y + (bounds.Dy()-croppedHeight)/2
		cropped = image.Rect(bounds.Min.X, y, bounds.Max.X, y+croppedHeight)
	}
	dst := image.NewRGBA(image.Rect(0, 0, cropped.Dx(), cropped.Dy()))
	xdraw.Draw(dst, dst.Bounds(), src, cropped.Min, xdraw.Src)
	return dst
}

// forgetPoster drops the decoded and resized copies of a cover that was removed
// from disk. The UI lifetime caches would otherwise keep the old image when a
// title with the same file name is added again.
func forgetPoster(path string) {
	if path == "" {
		return
	}
	posterImages.Lock()
	delete(posterImages.byPath, path)
	posterImages.Unlock()

	posterFits.Lock()
	for key := range posterFits.byKey {
		if key.path == path {
			delete(posterFits.byKey, key)
		}
	}
	posterFits.Unlock()
}

func drawPosterArt(gtx layout.Context, typo *typography, rect image.Rectangle, item historyTitle, initialSize unit.Sp) {
	if rect.Empty() {
		return
	}
	if img := posterForRect(item.coverPath, rect.Dx(), rect.Dy()); img != nil {
		radius := gtx.Dp(unit.Dp(6))
		drawImageCoverRounded(gtx, img, rect, radius)
		strokeRoundedRect(gtx, rect, float32(radius), float32(gtx.Dp(unit.Dp(1))), color.NRGBA{R: 255, G: 255, B: 255, A: 0x18})
		return
	}
	drawGeneratedPoster(gtx, typo, rect, item, initialSize)
}
