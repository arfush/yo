package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeTestPoster(t *testing.T, width, height int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "poster.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create poster: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 0x80, A: 0xff})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode poster: %v", err)
	}
	return path
}

func TestPosterForRectScalesToExactSize(t *testing.T) {
	path := writeTestPoster(t, 400, 600)
	img := posterForRect(path, 40, 60)
	if img == nil {
		t.Fatal("posterForRect returned nil")
	}
	if got, want := img.Bounds(), image.Rect(0, 0, 40, 60); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestPosterForRectCropsToTargetAspect(t *testing.T) {
	path := writeTestPoster(t, 400, 600)
	img := posterForRect(path, 40, 40)
	if img == nil {
		t.Fatal("posterForRect returned nil")
	}
	if got, want := img.Bounds(), image.Rect(0, 0, 40, 40); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestPosterForRectCachesBySize(t *testing.T) {
	path := writeTestPoster(t, 400, 600)
	first := posterForRect(path, 40, 60)
	second := posterForRect(path, 40, 60)
	if first == nil || second == nil {
		t.Fatal("posterForRect returned nil")
	}
	if first != second {
		t.Fatal("expected the same cached image for the same size")
	}
	if other := posterForRect(path, 40, 61); other == nil || other.Bounds() != image.Rect(0, 0, 40, 61) {
		t.Fatalf("different size was not resampled: %v", other)
	}
}

func TestPosterForRectRejectsInvalidInput(t *testing.T) {
	path := writeTestPoster(t, 40, 60)
	cases := []struct {
		name          string
		path          string
		width, height int
	}{
		{name: "empty path", path: "", width: 40, height: 60},
		{name: "zero width", path: path, width: 0, height: 60},
		{name: "zero height", path: path, width: 40, height: 0},
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.jpg"), width: 40, height: 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if img := posterForRect(tc.path, tc.width, tc.height); img != nil {
				t.Fatalf("posterForRect returned %v, want nil", img.Bounds())
			}
		})
	}
}

func TestCropPosterKeepsWideSourceCentered(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 300, 100))
	for x := 0; x < 300; x++ {
		src.SetRGBA(x, 0, color.RGBA{R: uint8(x % 255), A: 0xff})
	}
	cropped := cropPoster(src, 50, 50)
	if got, want := cropped.Bounds(), image.Rect(0, 0, 100, 100); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
	if got := cropped.(*image.RGBA).RGBAAt(0, 0).R; got != 100 {
		t.Fatalf("left edge = %d, want 100 (centered crop)", got)
	}
}
