// Package snapshot captures a QWidget into a Go image, either by rendering it
// offscreen through QWidget.Render or by grabbing the real window pixels.
package snapshot

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"unsafe"

	qt "github.com/mappu/miqt/qt6"
)

// Options configures an offscreen capture.
type Options struct {
	// Scale multiplies the logical widget size. Zero and negative mean 1.
	Scale int
}

// Capture renders the widget into a QImage at opt.Scale. The widget does not
// need to be shown, so the result is deterministic under QT_QPA_PLATFORM=offscreen.
func Capture(w *qt.QWidget, opt Options) (*image.NRGBA, error) {
	if w == nil {
		return nil, errors.New("snapshot: nil widget")
	}
	scale := opt.Scale
	if scale <= 0 {
		scale = 1
	}
	width, height := w.Width(), w.Height()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("snapshot: empty widget %dx%d", width, height)
	}

	img := qt.NewQImage3(width*scale, height*scale, qt.QImage__Format_RGBA8888)
	if img == nil {
		return nil, errors.New("snapshot: allocate image")
	}
	defer img.Delete()
	transparent := qt.NewQColor11(0, 0, 0, 0)
	img.FillWithColor(transparent)
	transparent.Delete()

	painter := qt.NewQPainter2(img.QPaintDevice)
	if !painter.IsActive() {
		painter.Delete()
		return nil, errors.New("snapshot: painter is not active")
	}
	painter.Scale(float64(scale), float64(scale))
	w.RenderWithPainter(painter)
	painter.End()
	painter.Delete()
	return toNRGBA(img)
}

// CaptureScreen grabs the real pixels of the shown window, window frame
// included. Use it to compare against an external screenshot.
func CaptureScreen(w *qt.QWidget) (*image.NRGBA, error) {
	if w == nil {
		return nil, errors.New("snapshot: nil widget")
	}
	screen := w.Screen()
	if screen == nil {
		return nil, errors.New("snapshot: widget has no screen")
	}
	pixmap := screen.GrabWindowWithWindow(w.WinId())
	if pixmap == nil {
		return nil, errors.New("snapshot: grab failed")
	}
	img := pixmap.ToImage()
	if img == nil {
		return nil, errors.New("snapshot: empty grab")
	}
	converted := img.ConvertToFormat(qt.QImage__Format_RGBA8888)
	if converted == nil {
		return nil, errors.New("snapshot: convert grab")
	}
	return toNRGBA(converted)
}

// Save writes img as a PNG.
func Save(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// Report is the result of Compare.
type Report struct {
	Different int
	Total     int
	MaxDelta  int
	Worst     image.Point
}

// Compare counts pixels whose worst channel differs by more than tolerance.
func Compare(got, want image.Image, tolerance int) (Report, error) {
	if got.Bounds().Size() != want.Bounds().Size() {
		return Report{}, fmt.Errorf("snapshot: size mismatch %v vs %v", got.Bounds().Size(), want.Bounds().Size())
	}
	report := Report{Total: got.Bounds().Dx() * got.Bounds().Dy()}
	gotMin, wantMin := got.Bounds().Min, want.Bounds().Min
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			a := color.NRGBAModel.Convert(got.At(gotMin.X+x, gotMin.Y+y)).(color.NRGBA)
			b := color.NRGBAModel.Convert(want.At(wantMin.X+x, wantMin.Y+y)).(color.NRGBA)
			delta := max(int(a.R)-int(b.R), int(b.R)-int(a.R))
			delta = max(delta, max(int(a.G)-int(b.G), int(b.G)-int(a.G)))
			delta = max(delta, max(int(a.B)-int(b.B), int(b.B)-int(a.B)))
			if delta > report.MaxDelta {
				report.MaxDelta = delta
				report.Worst = image.Pt(gotMin.X+x, gotMin.Y+y)
			}
			if delta > tolerance {
				report.Different++
			}
		}
	}
	return report, nil
}

// toNRGBA copies a QImage into Go memory. The caller keeps the QImage alive
// until this returns.
func toNRGBA(img *qt.QImage) (*image.NRGBA, error) {
	width, height := img.Width(), img.Height()
	if width <= 0 || height <= 0 {
		return nil, errors.New("snapshot: empty image")
	}
	if img.Format() != qt.QImage__Format_RGBA8888 {
		return nil, fmt.Errorf("snapshot: unexpected format %d", img.Format())
	}
	stride := int(img.BytesPerLine())
	bits := img.ConstBits()
	if bits == nil {
		return nil, errors.New("snapshot: image has no pixels")
	}
	src := unsafe.Slice(bits, stride*height)
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		copy(out.Pix[y*out.Stride:y*out.Stride+width*4], src[y*stride:y*stride+width*4])
	}
	return out, nil
}
