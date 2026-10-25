package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

type downloadProgress struct {
	Percent   float64
	Preparing bool
}

const (
	firstRunCardWidth    = 416
	firstRunCardHeight   = 268
	firstRunCardRadius   = 20
	firstRunCardPadding  = 32
	firstRunTitleTop     = 36
	firstRunSubtitleTop  = 100
	firstRunButtonTop    = 153
	firstRunButtonHeight = 48

	firstRunProgressAnimationRate = 8.0
	firstRunProgressSnap          = 0.01
)

const (
	firstRunTitleSize    = unit.Sp(24)
	firstRunSubtitleSize = unit.Sp(13)
	firstRunButtonSize   = unit.Sp(13.5)
	firstRunProgressSize = unit.Sp(13.5)
	firstRunErrorSize    = unit.Sp(12.5)
)

var (
	firstRunCardColor      = color.NRGBA{R: 0x17, G: 0x15, B: 0x1f, A: 0xff}
	firstRunCardBorder     = color.NRGBA{R: 0x27, G: 0x23, B: 0x33, A: 0xff}
	firstRunShadowColor    = color.NRGBA{A: 0x24}
	firstRunSubtitleColor  = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	firstRunErrorColor     = color.NRGBA{R: 0xff, G: 0x64, B: 0x73, A: 0xff}
	firstRunButtonColor1   = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}
	firstRunButtonColor2   = color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff}
	firstRunGlowColor      = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0x0a}
	firstRunProgressBorder = color.NRGBA{R: 0x3d, G: 0x31, B: 0x58, A: 0xff}
	firstRunProgressTrack  = color.NRGBA{R: 0x15, G: 0x10, B: 0x20, A: 0xff}
	firstRunProgressText   = color.NRGBA{R: 0xcf, G: 0xc6, B: 0xe5, A: 0xff}
)

type firstRunState int

const (
	firstRunIdle firstRunState = iota
	firstRunPreparing
	firstRunDownloading
	firstRunError
	firstRunDone
)

// firstRun is a preparation screen that downloads a required component. It
// draws itself manually so that the card geometry stays independent from the
// rest of the shell.
type firstRun struct {
	typo        *typography
	window      *app.Window
	button      widget.Clickable
	title       string
	description string
	downloadFn  func(context.Context, func(downloadProgress)) error
	// buttonText, retryText and errorText customize the generic download card
	// for the update screen. Empty values fall back to the component wording.
	buttonText string
	retryText  string
	errorText  string
	// autoStart begins the download as soon as the card is shown. Updates use
	// this instead of asking the user to press "Скачать" again.
	autoStart bool
	started   bool

	// onDone is called on the UI goroutine once the component is installed.
	onDone   func()
	notified bool

	mu       sync.Mutex
	state    firstRunState
	progress float64
	closed   bool
	cancel   context.CancelFunc

	// Animation state is only touched from the UI goroutine.
	shownProgress float64
	animTarget    float64
	animLast      time.Time

	// The description is wrapped once per card width; layout runs every frame
	// while the progress bar animates.
	wrappedLines []string
	wrappedWidth int
}

func newFirstRun(typo *typography, w *app.Window, title, description string, download func(context.Context, func(downloadProgress)) error) *firstRun {
	return &firstRun{
		typo:        typo,
		window:      w,
		title:       title,
		description: description,
		downloadFn:  download,
		buttonText:  "Скачивай",
		retryText:   "Повторить",
		errorText:   "Не удалось скачать. Попробуй ещё раз.",
	}
}

func (fr *firstRun) layout(gtx layout.Context) {
	state, progress, closed := fr.snapshot()
	if closed {
		return
	}
	if fr.autoStart && state == firstRunIdle && !fr.started {
		fr.started = true
		fr.startDownload()
		state = firstRunPreparing
	}
	if state == firstRunDone {
		if !fr.notified {
			fr.notified = true
			if fr.onDone != nil {
				fr.onDone()
			}
		}
		return
	}

	scale := gtx.Metric.PxPerDp
	if scale == 0 {
		scale = 1
	}
	px := func(v float32) int {
		return int(math.Round(float64(v) * float64(scale)))
	}

	contentSize := gtx.Constraints.Max
	cardWidth := px(firstRunCardWidth)
	cardHeight := px(firstRunCardHeight)
	if maxWidth := contentSize.X - px(32); cardWidth > maxWidth {
		cardWidth = maxWidth
	}
	if maxHeight := contentSize.Y - px(32); cardHeight > maxHeight {
		cardHeight = maxHeight
	}
	if cardWidth <= 0 || cardHeight <= 0 {
		return
	}

	card := image.Rect(
		(contentSize.X-cardWidth)/2,
		(contentSize.Y-cardHeight)/2,
		(contentSize.X-cardWidth)/2+cardWidth,
		(contentSize.Y-cardHeight)/2+cardHeight,
	)

	stack := op.Offset(card.Min).Push(gtx.Ops)
	defer stack.Pop()

	drawFirstRunCard(gtx, card.Size(), px)
	fr.typo.draw(gtx, fr.title, font.Font{Typeface: fontFamily, Weight: font.Bold}, firstRunTitleSize, titleColor, image.Pt(px(firstRunCardPadding), px(firstRunTitleTop)))

	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	descriptionWidth := cardWidth - 2*px(firstRunCardPadding)
	lines := fr.descriptionLines(gtx, descriptionWidth)
	lineAdvance := int(math.Round(float64(gtx.Sp(firstRunSubtitleSize)) * 1.25))
	if lineAdvance < 1 {
		lineAdvance = 1
	}
	descriptionTop := px(firstRunSubtitleTop)
	if len(lines) > 1 {
		descriptionTop -= lineAdvance / 4
	}
	for index, line := range lines {
		fr.typo.draw(gtx, line, face, firstRunSubtitleSize, firstRunSubtitleColor, image.Pt(px(firstRunCardPadding), descriptionTop+index*lineAdvance))
	}

	errorTop := descriptionTop + len(lines)*lineAdvance + px(6)
	buttonTop := px(firstRunButtonTop)
	errorHeight := int(math.Round(float64(gtx.Sp(firstRunErrorSize))))
	if errorTop+errorHeight+px(4) > buttonTop {
		buttonTop = errorTop + errorHeight + px(4)
	}

	if state == firstRunError {
		fr.typo.draw(gtx, upperFirst(fr.errorText), face, firstRunErrorSize, firstRunErrorColor, image.Pt(px(firstRunCardPadding), errorTop))
	}

	button := image.Rect(
		px(firstRunCardPadding),
		buttonTop,
		cardWidth-px(firstRunCardPadding),
		buttonTop+px(firstRunButtonHeight),
	)
	if button.Empty() {
		return
	}

	switch state {
	case firstRunIdle, firstRunError:
		if fr.layoutButton(gtx, button, state) {
			fr.startDownload()
		}
	default:
		if state == firstRunPreparing {
			fr.shownProgress = 0
			fr.animTarget = 0
			fr.animLast = gtx.Now
		} else {
			fr.animateProgress(gtx, progress)
		}
		fr.layoutProgress(gtx, button, fr.shownProgress, state == firstRunPreparing)
	}
}

func (fr *firstRun) descriptionLines(gtx layout.Context, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{fr.description}
	}
	if fr.wrappedWidth == maxWidth && len(fr.wrappedLines) > 0 {
		return fr.wrappedLines
	}
	fr.wrappedLines = wrapFirstRunText(
		gtx,
		fr.typo,
		fr.description,
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		firstRunSubtitleSize,
		maxWidth,
	)
	fr.wrappedWidth = maxWidth
	return fr.wrappedLines
}

// wrapFirstRunText breaks text at word boundaries so every line fits into
// maxWidth. A word that is wider than the card is kept on its own line rather
// than dropped.
func wrapFirstRunText(gtx layout.Context, typo *typography, text string, face font.Font, size unit.Sp, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}
	lines := make([]string, 0, 2)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		_, ink := typo.measure(gtx, candidate, face, size)
		if current != "" && ink.Dx() > maxWidth {
			lines = append(lines, current)
			current = word
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

func drawFirstRunCard(gtx layout.Context, size image.Point, px func(float32) int) {
	card := image.Rectangle{Max: size}

	// A few very transparent layers give the card a soft edge without needing
	// an offscreen blur. The shadow is deliberately subtle against the dark
	// content surface.
	for i := 10; i >= 2; i -= 2 {
		r := card.Inset(-px(float32(i))).Add(image.Pt(0, px(3)))
		fillRounded(gtx, firstRunShadowColor, r, px(firstRunCardRadius+float32(i)))
	}

	fillRounded(gtx, firstRunCardBorder, card, px(firstRunCardRadius))
	fillRounded(gtx, firstRunCardColor, card.Inset(px(1)), px(firstRunCardRadius-1))
}

func (fr *firstRun) layoutButton(gtx layout.Context, rect image.Rectangle, state firstRunState) bool {
	clicked := fr.button.Clicked(gtx)

	// Draw before Clickable.Layout: Clickable clips its widget to the button
	// bounds, which would cut off the glow.
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	fr.drawButton(gtx, rect.Size(), fr.button.Hovered(), state)
	stack.Pop()

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(rect.Size())
	stack = op.Offset(rect.Min).Push(gtx.Ops)
	fr.button.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()
	return clicked
}

func (fr *firstRun) drawButton(gtx layout.Context, size image.Point, hovered bool, state firstRunState) {
	rect := image.Rectangle{Max: size}
	radius := size.Y / 2

	drawFirstRunGlow(gtx, rect, radius)

	c1, c2 := firstRunButtonColor1, firstRunButtonColor2
	if hovered {
		c1 = lighten(c1, 0x08)
		c2 = lighten(c2, 0x08)
	}
	fillRoundedGradient(gtx, rect, rect, radius, c1, c2)

	label := fr.buttonText
	if state == firstRunError {
		label = fr.retryText
	}
	if label == "" {
		label = "Скачивай"
		if state == firstRunError {
			label = "Повторить"
		}
	}
	fr.typo.drawCentered(gtx, label, font.Font{Typeface: fontFamily, Weight: font.SemiBold}, firstRunButtonSize, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, image.Pt(size.X/2, size.Y/2))
}

// animateProgress eases the drawn percentage toward the latest reported value.
// The backend reports progress in coarse steps, so animating on the UI thread
// keeps the bar moving smoothly between those updates.
func (fr *firstRun) animateProgress(gtx layout.Context, target float64) {
	if target < fr.shownProgress {
		fr.shownProgress = target
	}
	if target != fr.animTarget {
		fr.animTarget = target
		fr.animLast = gtx.Now
	}
	if fr.animLast.IsZero() {
		fr.animLast = gtx.Now
	}
	elapsed := gtx.Now.Sub(fr.animLast)
	if elapsed < 0 {
		elapsed = 0
	}
	// A delayed frame should not produce a large visible jump.
	if elapsed > 100*time.Millisecond {
		elapsed = 100 * time.Millisecond
	}
	fr.animLast = gtx.Now
	fr.shownProgress = smoothFirstRunProgress(fr.shownProgress, target, elapsed)
	if math.Abs(target-fr.shownProgress) < firstRunProgressSnap {
		fr.shownProgress = target
		return
	}
	gtx.Execute(op.InvalidateCmd{})
}

func smoothFirstRunProgress(current, target float64, elapsed time.Duration) float64 {
	if target <= current {
		return target
	}
	if elapsed <= 0 {
		return current
	}
	alpha := 1 - math.Exp(-elapsed.Seconds()*firstRunProgressAnimationRate)
	next := current + (target-current)*alpha
	if target-next < firstRunProgressSnap {
		return target
	}
	return next
}

func (fr *firstRun) layoutProgress(gtx layout.Context, rect image.Rectangle, progress float64, preparing bool) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	fr.drawProgress(gtx, rect.Size(), progress, preparing)
	stack.Pop()
}

func (fr *firstRun) drawProgress(gtx layout.Context, size image.Point, progress float64, preparing bool) {
	rect := image.Rectangle{Max: size}
	radius := size.Y / 2
	border := max(1, int(math.Round(float64(size.Y)*0.02)))

	fillRounded(gtx, firstRunProgressBorder, rect, radius)
	inner := rect.Inset(border)
	if inner.Empty() {
		return
	}
	innerRadius := inner.Dy() / 2
	fillRounded(gtx, firstRunProgressTrack, inner, innerRadius)

	progress = math.Max(0, math.Min(100, progress))
	if progress > 0 && !preparing {
		fillWidth := int(float64(inner.Dx()) * progress / 100)
		if fillWidth > 0 {
			fillRect := image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+fillWidth, inner.Max.Y)
			fillRadius := innerRadius
			if fillWidth < 2*fillRadius {
				fillRadius = fillWidth / 2
			}
			drawFirstRunGlow(gtx, fillRect, fillRadius)
			fillRoundedGradient(gtx, fillRect, inner, fillRadius, firstRunButtonColor1, firstRunButtonColor2)
		}
	}

	label := fmt.Sprintf("%d%%", int(math.Round(progress)))
	labelColor := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	if preparing {
		label = "Подготовка..."
		labelColor = firstRunProgressText
	}
	fr.typo.drawCentered(gtx, label, font.Font{Typeface: fontFamily, Weight: font.Medium}, firstRunProgressSize, labelColor, image.Pt(inner.Min.X+inner.Dx()/2, inner.Min.Y+inner.Dy()/2))
}

// drawFirstRunGlow draws the same tight glow used by the main download
// button. Callers pass only the shape that should glow.
func drawFirstRunGlow(gtx layout.Context, rect image.Rectangle, radius int) {
	drawGlow(gtx, rect, radius, firstRunGlowColor)
}

// drawGlow draws a soft edge using several low-alpha layers.
func drawGlow(gtx layout.Context, rect image.Rectangle, radius int, col color.NRGBA) {
	// Several low-alpha layers approximate a soft glow. A single translucent
	// ring reads as a border rather than a glow at this size.
	// 2.5x tighter than the original 9/7/5/3 px rings.
	for _, inset := range []int{4, 3, 2, 1} {
		fillRounded(gtx, col, rect.Inset(-inset), radius+inset)
	}
}

func fillRoundedGradient(gtx layout.Context, clipRect, gradientRect image.Rectangle, radius int, c1, c2 color.NRGBA) {
	if clipRect.Empty() {
		return
	}
	stack := clip.UniformRRect(clipRect, radius).Push(gtx.Ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(gradientRect.Min.X), float32(gradientRect.Min.Y)),
		Color1: c1,
		Stop2:  f32.Pt(float32(gradientRect.Max.X), float32(gradientRect.Max.Y)),
		Color2: c2,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

func lighten(c color.NRGBA, amount uint8) color.NRGBA {
	add := func(v uint8) uint8 {
		if int(v)+int(amount) > 255 {
			return 255
		}
		return v + amount
	}
	return color.NRGBA{R: add(c.R), G: add(c.G), B: add(c.B), A: c.A}
}

func (fr *firstRun) snapshot() (firstRunState, float64, bool) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	return fr.state, fr.progress, fr.closed
}

func (fr *firstRun) startDownload() {
	fr.mu.Lock()
	if fr.closed || fr.state == firstRunPreparing || fr.state == firstRunDownloading || fr.state == firstRunDone {
		fr.mu.Unlock()
		return
	}
	fr.state = firstRunPreparing
	fr.progress = 0
	ctx, cancel := context.WithCancel(context.Background())
	fr.cancel = cancel
	fr.mu.Unlock()

	fr.shownProgress = 0
	fr.animTarget = 0
	fr.animLast = time.Time{}
	fr.window.Invalidate()
	go fr.download(ctx)
}

func (fr *firstRun) download(ctx context.Context) {
	err := fr.downloadFn(ctx, func(progress downloadProgress) {
		fr.mu.Lock()
		if fr.closed {
			fr.mu.Unlock()
			return
		}
		if progress.Preparing {
			fr.state = firstRunPreparing
		} else {
			fr.state = firstRunDownloading
			if progress.Percent > fr.progress {
				fr.progress = progress.Percent
			}
		}
		fr.mu.Unlock()
		fr.window.Invalidate()
	})

	fr.mu.Lock()
	if fr.closed {
		fr.mu.Unlock()
		return
	}
	fr.cancel = nil
	if err != nil {
		fr.state = firstRunError
		fr.mu.Unlock()
		logFirstRunError(err)
		fr.window.Invalidate()
		return
	}

	fr.state = firstRunDownloading
	fr.progress = 100
	fr.mu.Unlock()
	fr.window.Invalidate()

	// Let the completed bar paint before the card disappears.
	time.Sleep(900 * time.Millisecond)

	fr.mu.Lock()
	if fr.closed {
		fr.mu.Unlock()
		return
	}
	fr.state = firstRunDone
	fr.mu.Unlock()
	fr.window.Invalidate()
}

// Close cancels an in-flight download.
func (fr *firstRun) Close() {
	if fr == nil {
		return
	}
	fr.mu.Lock()
	if fr.closed {
		fr.mu.Unlock()
		return
	}
	fr.closed = true
	cancel := fr.cancel
	fr.cancel = nil
	fr.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func logFirstRunError(err error) {
	if err == nil {
		return
	}
	dir, cacheErr := os.UserCacheDir()
	if cacheErr != nil {
		return
	}
	logDir := filepath.Join(dir, "Yonime")
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr != nil {
		return
	}
	file, openErr := os.OpenFile(filepath.Join(logDir, "download-error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s\n%v\n\n", time.Now().Format(time.RFC3339), err)
}
