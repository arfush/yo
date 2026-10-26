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
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/browser"
	"github.com/arfush/yonime/internal/token"
)

const (
	tokenCardWidth    = firstRunCardWidth
	tokenCardHeight   = firstRunCardHeight
	tokenCardPadding  = firstRunCardPadding
	tokenTitleTop     = firstRunTitleTop
	tokenDescTop      = firstRunSubtitleTop
	tokenButtonTop    = firstRunButtonTop
	tokenButtonHeight = firstRunButtonHeight

	// The domain field replaces the GO button and keeps the same visual center.
	// Its 60dp height fits the same 40dp Next button used by the main window's
	// link input.
	tokenDomainFieldTop    = tokenButtonTop - 6
	tokenDomainFieldHeight = 60

	tokenDomainChipHeight = 22
	tokenDomainChipPadX   = 8
)

const (
	tokenTitleSize  = unit.Sp(24)
	tokenDescSize   = unit.Sp(13)
	tokenDomainSize = unit.Sp(11)
	tokenButtonSize = unit.Sp(13.5)

	tokenTitle             = "Требуется токен"
	tokenDescription       = "Пройдите авторизацию на AnimeLIB"
	tokenDomainTitle       = "AnimeLIB сломался?"
	tokenDomainDescription = "Введите домен, который у вас открывается"
)

const (
	tokenErrorDuration    = 5 * time.Second
	tokenShakeDuration    = 520 * time.Millisecond
	tokenShakeAmplitudeDp = 2.4
)

var (
	tokenErrorColor1     = color.NRGBA{R: 0xf2, G: 0x55, B: 0x64, A: 0xff}
	tokenErrorColor2     = color.NRGBA{R: 0xc7, G: 0x28, B: 0x3c, A: 0xff}
	tokenErrorGlow       = color.NRGBA{R: 0xf2, G: 0x55, B: 0x64, A: 0x0c}
	tokenDomainFill      = color.NRGBA{R: 0x1d, G: 0x1a, B: 0x27, A: 0xff}
	tokenDomainFillHover = color.NRGBA{R: 0x25, G: 0x21, B: 0x33, A: 0xff}
	tokenDomainBorder    = color.NRGBA{R: 0x35, G: 0x2f, B: 0x47, A: 0xff}
)

type tokenPromptState int

const (
	tokenIdle tokenPromptState = iota
	tokenAuthorizing
	tokenFailed
	tokenDone
)

type tokenPromptMode int

const (
	tokenModeMain tokenPromptMode = iota
	tokenModeDomain
)

// tokenPrompt asks the user to sign in on AnimeLIB and opens the project site
// in the installed Chromium. It reuses the first-run card, button and progress
// helpers so both preparation screens look alike.
type tokenPrompt struct {
	typo    *typography
	browser *browser.Manager
	window  *app.Window

	tokenPath  string
	profileDir string

	button       widget.Clickable
	domainButton widget.Clickable
	nextButton   widget.Clickable
	nextHover    hoverAnim
	editor       widget.Editor

	domain     string
	mode       tokenPromptMode
	focusField bool
	// pxPerDp is captured from the last frame so the browser window can be
	// placed over the application window in device-independent pixels.
	pxPerDp float64

	// onDone is called on the UI goroutine once a token has been saved.
	onDone   func()
	notified bool

	mu            sync.Mutex
	state         tokenPromptState
	errorAt       time.Time
	domainErrorAt time.Time
	closed        bool
	cancel        context.CancelFunc
}

func newTokenPrompt(typo *typography, manager *browser.Manager, w *app.Window, tokenPath, profileDir string) *tokenPrompt {
	prompt := &tokenPrompt{
		typo:       typo,
		browser:    manager,
		window:     w,
		tokenPath:  tokenPath,
		profileDir: profileDir,
		domain:     browser.DefaultDomain,
	}
	prompt.editor.SingleLine = true
	prompt.editor.SetText(prompt.domain)
	return prompt
}

func (tp *tokenPrompt) layout(gtx layout.Context) {
	state, errorAt, closed := tp.snapshot()
	if closed {
		return
	}
	if state == tokenDone {
		if !tp.notified {
			tp.notified = true
			if tp.onDone != nil {
				tp.onDone()
			}
		}
		return
	}

	tp.handleKeys(gtx)

	scale := gtx.Metric.PxPerDp
	if scale == 0 {
		scale = 1
	}
	tp.pxPerDp = float64(scale)
	px := func(v float32) int {
		return int(math.Round(float64(v) * float64(scale)))
	}

	contentSize := gtx.Constraints.Max
	cardWidth := px(tokenCardWidth)
	cardHeight := px(tokenCardHeight)
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

	pad := px(tokenCardPadding)
	drawDownloadCard(gtx, card.Size(), px)

	if tp.mode == tokenModeDomain {
		tp.layoutDomainCard(gtx, cardWidth, pad, px)
		return
	}
	tp.layoutMainCard(gtx, cardWidth, pad, px, state, errorAt)
}

func (tp *tokenPrompt) layoutMainCard(gtx layout.Context, cardWidth, pad int, px func(float32) int, state tokenPromptState, errorAt time.Time) {
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}

	tp.typo.draw(gtx, tokenTitle, titleFace, tokenTitleSize, titleColor, image.Pt(pad, px(tokenTitleTop)))
	tp.layoutDomainChip(gtx, cardWidth, pad, px, titleFace)
	tp.typo.draw(gtx, tokenDescription, face, tokenDescSize, firstRunSubtitleColor, image.Pt(pad, px(tokenDescTop)))

	button := image.Rect(pad, px(tokenButtonTop), cardWidth-pad, px(tokenButtonTop)+px(tokenButtonHeight))
	if button.Empty() {
		return
	}
	if state == tokenAuthorizing {
		tp.drawWaiting(gtx, button)
		return
	}

	errored, shake := tokenErrorFeedback(gtx.Now, errorAt)
	if tp.layoutButton(gtx, button, errored, shake) {
		tp.startAuthorize()
	}
	if errored {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
	}
}

// layoutDomainChip draws the current domain next to the title. Clicking it
// switches the card to the small domain-edit form.
func (tp *tokenPrompt) layoutDomainChip(gtx layout.Context, cardWidth, pad int, px func(float32) int, titleFace font.Font) {
	_, titleInk := tp.typo.measure(gtx, tokenTitle, titleFace, tokenTitleSize)
	chipHeight := px(tokenDomainChipHeight)
	if chipHeight <= 0 {
		return
	}
	chipTop := px(tokenTitleTop) + (titleInk.Dy()-chipHeight)/2

	domainFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, domainInk := tp.typo.measure(gtx, tp.domain, domainFace, tokenDomainSize)
	chipWidth := domainInk.Dx() + 2*px(tokenDomainChipPadX)
	maxWidth := cardWidth - 2*pad - titleInk.Dx() - px(12)
	if maxWidth <= 0 {
		return
	}
	if chipWidth > maxWidth {
		chipWidth = maxWidth
	}
	chip := image.Rect(cardWidth-pad-chipWidth, chipTop, cardWidth-pad, chipTop+chipHeight)
	if chip.Empty() {
		return
	}

	clicked := tp.domainButton.Clicked(gtx)
	hovered := tp.domainButton.Hovered()

	stack := op.Offset(chip.Min).Push(gtx.Ops)
	rect := image.Rectangle{Max: chip.Size()}
	radius := chipHeight / 2
	fillRounded(gtx, tokenDomainBorder, rect, radius)
	fill := tokenDomainFill
	if hovered {
		fill = tokenDomainFillHover
	}
	fillRounded(gtx, fill, rect.Inset(px(1)), max(1, radius-px(1)))
	textWidth := chipWidth - 2*px(tokenDomainChipPadX)
	if domainInk.Dx() <= textWidth {
		tp.typo.drawCentered(gtx, tp.domain, domainFace, tokenDomainSize, firstRunProgressText, image.Pt(chipWidth/2, chipHeight/2))
	} else {
		drawTruncatedText(gtx, tp.typo, tp.domain, domainFace, tokenDomainSize, firstRunProgressText, image.Pt(px(tokenDomainChipPadX), (chipHeight-domainInk.Dy())/2), textWidth)
	}
	stack.Pop()

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(chip.Size())
	stack = op.Offset(chip.Min).Push(gtx.Ops)
	tp.domainButton.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: chip.Size()}
	})
	stack.Pop()

	if clicked {
		tp.mode = tokenModeDomain
		tp.editor.SetText(tp.domain)
		tp.editor.SetCaret(0, len(tp.domain))
		tp.focusField = true
		tp.window.Invalidate()
	}
}

func (tp *tokenPrompt) layoutDomainCard(gtx layout.Context, cardWidth, pad int, px func(float32) int) {
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}

	tp.typo.draw(gtx, tokenDomainTitle, titleFace, tokenTitleSize, titleColor, image.Pt(pad, px(tokenTitleTop)))
	tp.typo.draw(gtx, tokenDomainDescription, face, tokenDescSize, firstRunSubtitleColor, image.Pt(pad, px(tokenDescTop)))

	if tp.focusField {
		tp.focusField = false
		gtx.Execute(key.FocusCmd{Tag: &tp.editor})
	}

	field := image.Rect(pad, px(tokenDomainFieldTop), cardWidth-pad, px(tokenDomainFieldTop)+px(tokenDomainFieldHeight))
	if !field.Empty() {
		tp.layoutDomainField(gtx, field)
	}
}

func (tp *tokenPrompt) layoutDomainField(gtx layout.Context, rect image.Rectangle) {
	field := actionField{
		typo:        tp.typo,
		editor:      &tp.editor,
		submit:      &tp.nextButton,
		submitHover: &tp.nextHover,
		box: fieldBoxStyle{
			fill:        mainInputColor,
			border:      mainInputBorder,
			focusBorder: mainInputFocusBorder,
			errorBorder: mainDangerColor,
			radius:      mainInputRadius,
		},
		textSize:      mainInputTextSize,
		paddingX:      unit.Dp(12),
		gap:           mainInputGap,
		errorAt:       tp.domainErrorAt,
		animationsRun: func() bool { return true },
		onSubmit:      func(layout.Context) { tp.applyDomain() },
	}
	field.layout(gtx, rect, gtx.Focused(&tp.editor))
}

func (tp *tokenPrompt) drawNextButton(gtx layout.Context, size image.Point, hovered bool) {
	rect := image.Rectangle{Max: size}
	if rect.Empty() {
		return
	}
	c1, c2 := mainAccentColor1, mainAccentColor2
	if hovered {
		c1 = lighten(c1, 0x08)
		c2 = lighten(c2, 0x08)
	}
	fillRoundedGradient(gtx, rect, rect, rect.Dy()/2, c1, c2)
	drawChevronRight(gtx, centerOf(rect), rect.Dx()/3, gtx.Dp(unit.Dp(2)), color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
}

func (tp *tokenPrompt) handleKeys(gtx layout.Context) {
	if tp.mode != tokenModeDomain {
		return
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameReturn},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: key.NameEscape},
		)
		if !ok {
			break
		}
		event, ok := ev.(key.Event)
		if !ok || event.State != key.Press {
			continue
		}
		if event.Name == key.NameEscape {
			tp.mode = tokenModeMain
			tp.focusField = false
			tp.window.Invalidate()
			continue
		}
		tp.applyDomain()
	}
}

// applyDomain extracts the host from the input, remembers it and returns to
// the main card. Invalid input keeps the editor open so it can be corrected
// without any visible error message.
func (tp *tokenPrompt) applyDomain() {
	domain, err := browser.NormalizeDomain(strings.TrimSpace(tp.editor.Text()))
	if err != nil || domain == "" {
		tp.domainErrorAt = time.Now()
		tp.editor.SetCaret(0, len(tp.editor.Text()))
		tp.focusField = true
		tp.window.Invalidate()
		return
	}
	tp.domainErrorAt = time.Time{}
	tp.domain = domain
	tp.mode = tokenModeMain
	tp.focusField = false
	tp.window.Invalidate()
}

func (tp *tokenPrompt) layoutButton(gtx layout.Context, rect image.Rectangle, errored bool, shake float32) bool {
	clicked := tp.button.Clicked(gtx)

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	tp.drawButton(gtx, rect.Size(), tp.button.Hovered(), errored, shake)
	stack.Pop()

	btnGtx := gtx
	btnGtx.Constraints = layout.Exact(rect.Size())
	stack = op.Offset(rect.Min).Push(gtx.Ops)
	tp.button.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: rect.Size()}
	})
	stack.Pop()
	return clicked
}

func (tp *tokenPrompt) drawButton(gtx layout.Context, size image.Point, hovered, errored bool, shake float32) {
	rect := image.Rectangle{Max: size}
	radius := size.Y / 2

	if errored {
		drawGlow(gtx, rect, radius, tokenErrorGlow)
	} else {
		drawFirstRunGlow(gtx, rect, radius)
	}

	c1, c2 := firstRunButtonColor1, firstRunButtonColor2
	if errored {
		c1, c2 = tokenErrorColor1, tokenErrorColor2
	}
	if hovered {
		c1 = lighten(c1, 0x08)
		c2 = lighten(c2, 0x08)
	}
	fillRoundedGradient(gtx, rect, rect, radius, c1, c2)

	center := image.Pt(size.X/2, size.Y/2)
	if shake != 0 {
		center.X += int(math.Round(float64(shake) * float64(gtx.Dp(unit.Dp(tokenShakeAmplitudeDp)))))
	}
	tp.typo.drawCentered(gtx, "ГО", font.Font{Typeface: fontFamily, Weight: font.SemiBold}, tokenButtonSize, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, center)
}

// drawWaiting reuses the first-run progress bar shape for the authorization
// wait, so the card does not jump when the browser is open.
func (tp *tokenPrompt) drawWaiting(gtx layout.Context, rect image.Rectangle) {
	radius := rect.Dy() / 2
	border := max(1, int(math.Round(float64(rect.Dy())*0.02)))

	fillRounded(gtx, firstRunProgressBorder, rect, radius)
	inner := rect.Inset(border)
	if inner.Empty() {
		return
	}
	fillRounded(gtx, firstRunProgressTrack, inner, inner.Dy()/2)
	tp.typo.drawCentered(gtx, "Ожидание авторизации...", font.Font{Typeface: fontFamily, Weight: font.Medium}, tokenButtonSize, firstRunProgressText, image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2))
}

func (tp *tokenPrompt) snapshot() (tokenPromptState, time.Time, bool) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.state, tp.errorAt, tp.closed
}

func (tp *tokenPrompt) startAuthorize() {
	tp.mu.Lock()
	if tp.closed || tp.state == tokenAuthorizing || tp.state == tokenDone {
		tp.mu.Unlock()
		return
	}
	tp.state = tokenAuthorizing
	tp.errorAt = time.Time{}
	ctx, cancel := context.WithCancel(context.Background())
	tp.cancel = cancel
	tp.mu.Unlock()

	scale := tp.pxPerDp
	if scale <= 0 {
		scale = 1
	}
	var bounds *browser.WindowBounds
	if value, ok := appWindowBounds(scale); ok {
		bounds = &value
	}

	site := tp.domain
	tp.window.Invalidate()
	go tp.authorize(ctx, site, bounds)
}

func (tp *tokenPrompt) authorize(ctx context.Context, site string, bounds *browser.WindowBounds) {
	value, err := tp.browser.Authorize(ctx, site, tp.profileDir, bounds)
	if err == nil {
		err = token.Save(tp.tokenPath, value)
	}

	tp.mu.Lock()
	if tp.closed {
		tp.mu.Unlock()
		return
	}
	tp.cancel = nil
	if err != nil {
		tp.state = tokenFailed
		tp.errorAt = time.Now()
		tp.mu.Unlock()
		logTokenError(err)
		tp.window.Invalidate()
		return
	}
	tp.state = tokenDone
	tp.mu.Unlock()
	tp.window.Invalidate()
}

// tokenErrorFeedback reports whether the GO button is in its three-second red
// state and returns the current label shake amount.
func tokenErrorFeedback(now, errorAt time.Time) (bool, float32) {
	if errorAt.IsZero() {
		return false, 0
	}
	elapsed := now.Sub(errorAt)
	if elapsed < 0 || elapsed >= tokenErrorDuration {
		return false, 0
	}
	return true, tokenLabelShake(now, errorAt)
}

// tokenLabelShake is a short decaying wobble. It starts and ends at zero, so
// repeated errors never leave the label displaced.
func tokenLabelShake(now, start time.Time) float32 {
	if start.IsZero() {
		return 0
	}
	elapsed := now.Sub(start)
	if elapsed < 0 || elapsed >= tokenShakeDuration {
		return 0
	}
	p := float32(elapsed) / float32(tokenShakeDuration)
	return (1 - p) * float32(math.Sin(6*math.Pi*float64(p)))
}

// Close cancels a pending authorization. The browser manager closes the
// Chromium context as part of application shutdown.
func (tp *tokenPrompt) Close() {
	if tp == nil {
		return
	}
	tp.mu.Lock()
	if tp.closed {
		tp.mu.Unlock()
		return
	}
	tp.closed = true
	cancel := tp.cancel
	tp.cancel = nil
	tp.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func logTokenError(err error) {
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
	file, openErr := os.OpenFile(filepath.Join(logDir, "token-error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s\n%v\n\n", time.Now().Format(time.RFC3339), err)
}
