package ui

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
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

func drawPosterArt(gtx layout.Context, typo *typography, rect image.Rectangle, item historyTitle, initialSize unit.Sp) {
	if img := loadPosterImage(item.coverPath); img != nil {
		radius := gtx.Dp(unit.Dp(6))
		drawImageCoverRounded(gtx, img, rect, radius)
		strokeRoundedRect(gtx, rect, float32(radius), float32(gtx.Dp(unit.Dp(1))), color.NRGBA{R: 255, G: 255, B: 255, A: 0x18})
		return
	}
	drawGeneratedPoster(gtx, typo, rect, item, initialSize)
}
