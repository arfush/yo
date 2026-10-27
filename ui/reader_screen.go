package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
)

// readerSource is the slice of the library the reader needs. It is an
// interface so the screen can be tested with a stub and with a real database.
type readerSource interface {
	ReadChapter(context.Context, catalog.Title, string, string) (catalog.ReadingContent, error)
	ReadingImage(context.Context, catalog.Title, string) (image.Image, error)
	TrackViewing(catalog.Title, string, string) (int64, error)
	SaveSelection(int64, string, string) error
}

const (
	readerSidebarWidth  = unit.Dp(280)
	readerPanelPad      = unit.Dp(18)
	readerControlHeight = unit.Dp(34)
	readerChapterHeight = unit.Dp(40)
	readerSearchHeight  = unit.Dp(34)
	readerHeaderHeight  = unit.Dp(48)
	readerParaGap       = unit.Dp(12)
	// readerChapterWindow is the number of chapters kept ready on each side of
	// the current one. Moving one chapter forward/back does not wait for the
	// network because the next window was already requested.
	readerChapterWindow = 5
)

var (
	readerTitleColor    = color.NRGBA{R: 0xf4, G: 0xf2, B: 0xfa, A: 0xff}
	readerMutedColor    = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	readerPanelFill     = color.NRGBA{R: 0x14, G: 0x12, B: 0x1d, A: 0xff}
	readerPanelBorder   = color.NRGBA{R: 0x24, G: 0x21, B: 0x33, A: 0xff}
	readerChapterHover  = color.NRGBA{R: 0x22, G: 0x1d, B: 0x33, A: 0xff}
	readerChapterActive = color.NRGBA{R: 0x2b, G: 0x22, B: 0x45, A: 0xff}
	readerRowDivider    = color.NRGBA{R: 0x1b, G: 0x18, B: 0x26, A: 0xff}
	readerDisabledText  = color.NRGBA{R: 0x55, G: 0x4f, B: 0x66, A: 0xff}
)

// readerTranslation is one team and the number of chapters it published.
type readerTranslation struct {
	name  string
	count int
}

// readerPageState is one manga page in the virtualised list.
type readerPageState struct {
	url     string
	img     image.Image
	loading bool
	failed  bool
}

type readerImageResult struct {
	generation int
	index      int
	img        image.Image
	err        error
}

type readerChapterResult struct {
	voice   string
	key     string
	content catalog.ReadingContent
	err     error
}

// readerScreen is the manga and ranobe reader. It owns the chapter list, the
// translation selector, the lazy page cache and the ranobe paragraph renderer.
type readerScreen struct {
	typo *typography

	// item carries the title identity and the saved chapter/translation.
	item historyTitle
	// title is the catalog title loaded by the library before this reader
	// opened. It always carries the fresh chapter list and selection.
	title   catalog.Title
	section librarySection
	source  readerSource

	// trackedID is the history id used for the asynchronous selection writes.
	// A freshly fetched title starts at zero until TrackViewing creates its
	// row; keeping this out of the UI path is what keeps opening a manga from
	// blocking the frame loop.
	trackedID    atomic.Int64
	trackPending atomic.Bool

	selectedVoice   int
	selectedChapter int
	// chapters are the keys of the selected translation.
	chapters []string
	// translations is the ordered translation-team list with chapter counts.
	translations []readerTranslation

	// search filters the chapter list by display label or raw key.
	filtered  []string
	lastQuery string
	search    widget.Editor
	clear     widget.Clickable

	translationClick widget.Clickable
	openTranslations bool
	translationScrim widget.Clickable

	searchList            layout.List
	chapterClick          []widget.Clickable
	translationClickables []widget.Clickable
	chapterList           layout.List

	content        catalog.ReadingContent
	loading        bool
	loadErr        string
	loadGeneration int
	chapterResults chan readerChapterResult
	// chapterCache holds loaded chapter bodies for the current reader session.
	// The key includes the translation, because the same chapter number has a
	// different branch id and sometimes different pages in another team.
	chapterCache   map[string]catalog.ReadingContent
	chapterPending map[string]bool
	contentCtx     context.Context
	contentCancel  context.CancelFunc
	imageResults   chan readerImageResult
	imageCancel    context.CancelFunc
	imageSem       chan struct{}
	pages          []readerPageState
	mangaList      layout.List
	ranobeList     layout.List

	back footerNamedButton
	prev footerNamedButton
	next footerNamedButton

	focusCleared bool

	hidden     func() bool
	onBack     func()
	onError    func(error)
	invalidate func()
}

func newReaderScreen(typo *typography, item historyTitle, source readerSource) *readerScreen {
	contentCtx, contentCancel := context.WithCancel(context.Background())
	r := &readerScreen{
		typo:           typo,
		item:           item,
		source:         source,
		section:        item.section(),
		chapterResults: make(chan readerChapterResult, 32),
		chapterCache:   make(map[string]catalog.ReadingContent),
		chapterPending: make(map[string]bool),
		contentCtx:     contentCtx,
		contentCancel:  contentCancel,
		imageResults:   make(chan readerImageResult, 32),
		imageSem:       make(chan struct{}, 3),
	}
	r.trackedID.Store(item.id)
	r.search.SingleLine = true
	r.searchList.Axis = layout.Vertical
	r.chapterList.Axis = layout.Vertical
	r.mangaList.Axis = layout.Vertical
	r.ranobeList.Axis = layout.Vertical
	r.back = newBackFooterButton()
	r.prev = footerNamedButton{
		label: "Назад",
		icon:  footerButtonBackIcon,
		style: footerSecondaryStyle,
		size:  unit.Sp(13),
	}
	r.next = footerNamedButton{
		label: "Вперёд",
		icon:  footerButtonForwardIcon,
		style: footerSecondaryStyle,
		size:  unit.Sp(13),
	}
	return r
}

// prefill seeds the reader with a title that was just loaded by the library.
// The chapter list and the saved selection come from that load, so no extra
// fetch is needed on open.
func (r *readerScreen) prefill(title catalog.Title) {
	r.title = title
	if title.Source != "" {
		r.section = sectionForSource(title.Source)
	}
	r.item.source = title.Source
	r.item.title = title.Russian
	r.item.original = title.Original
	r.item.slug = title.Slug
	if title.ID != 0 {
		r.item.id = title.ID
		r.trackedID.Store(title.ID)
	}
	r.selectedVoice = voiceIndex(r.item, title.SelectedVoice)
	r.selectedChapter = episodeIndex(r.item, title.SelectedEpisode)
	r.rebuildTranslations()
	r.rebuildChapters()
	if r.selectedChapter < 0 {
		r.selectedChapter = 0
	}
	r.loadChapter()
}

func (r *readerScreen) rebuildTranslations() {
	r.translations = r.translations[:0]
	for _, voice := range r.title.Voiceovers {
		r.translations = append(r.translations, readerTranslation{
			name:  voice.Name,
			count: len(voice.Episodes),
		})
	}
	if r.selectedVoice >= len(r.translations) {
		r.selectedVoice = 0
	}
}

func (r *readerScreen) rebuildChapters() {
	var episodes []string
	if r.selectedVoice >= 0 && r.selectedVoice < len(r.title.Voiceovers) {
		episodes = r.title.Voiceovers[r.selectedVoice].Episodes
	}
	r.chapters = episodes
	r.chapterClick = make([]widget.Clickable, len(r.chapters))
	r.applySearchFilter()
}

func (r *readerScreen) refreshFiltered() {
	r.filtered = r.filtered[:0]
	query := strings.ToLower(strings.TrimSpace(r.search.Text()))
	for _, key := range r.chapters {
		if query == "" ||
			strings.Contains(strings.ToLower(key), query) ||
			strings.Contains(strings.ToLower(readingChapterLabel(key)), query) {
			r.filtered = append(r.filtered, key)
		}
	}
	r.chapterList.Position = layout.Position{}
}

// applySearchFilter rebuilds the filtered list only when the typed query
// changed, so keystrokes do not reset the scroll in every frame.
func (r *readerScreen) applySearchFilter() {
	current := strings.ToLower(strings.TrimSpace(r.search.Text()))
	if current == r.lastQuery {
		return
	}
	r.lastQuery = current
	r.refreshFiltered()
}

func (r *readerScreen) translationName() string {
	if r.selectedVoice >= 0 && r.selectedVoice < len(r.translations) {
		return r.translations[r.selectedVoice].name
	}
	return ""
}

func (r *readerScreen) currentChapterKey() string {
	if r.selectedChapter >= 0 && r.selectedChapter < len(r.chapters) {
		return r.chapters[r.selectedChapter]
	}
	return ""
}

// loadChapter switches the reader to the current chapter. A body that is
// already in the session cache paints immediately; otherwise the request is
// queued and the spinner stays until it arrives. Either way the surrounding
// window is requested so one more chapter forward/back is already ready.
func (r *readerScreen) loadChapter() {
	r.loadGeneration++
	r.loadErr = ""
	r.cancelImages()
	for len(r.imageResults) > 0 {
		<-r.imageResults
	}

	voice := r.translationName()
	key := r.currentChapterKey()
	title := r.title

	r.trackSelection(title, voice, key)

	if key == "" {
		r.loading = false
		r.content = catalog.ReadingContent{}
		r.pages = nil
		r.requestFrame()
		return
	}

	if content, ok := r.chapterCache[readerChapterCacheKey(voice, key)]; ok {
		r.loading = false
		r.applyChapterContent(content)
		r.prefetchChapters()
		r.requestFrame()
		return
	}

	r.loading = true
	r.content = catalog.ReadingContent{}
	r.pages = nil
	r.requestChapter(voice, key, title)
	r.prefetchChapters()
	r.requestFrame()
}

// trackSelection persists the reader position without ever doing database or
// network work on the UI goroutine. A new title first gets a history row, and
// later chapter moves only update that row.
func (r *readerScreen) trackSelection(title catalog.Title, voice, key string) {
	if r.source == nil {
		return
	}
	if id := r.trackedID.Load(); id != 0 {
		go func() {
			if err := r.source.SaveSelection(id, voice, key); err != nil {
				logger.Warn("cannot save reader selection", "slug", title.Slug, "chapter", key, "error", err)
			}
		}()
		return
	}
	if !r.trackPending.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer r.trackPending.Store(false)
		id, err := r.source.TrackViewing(title, voice, key)
		if err != nil {
			logger.Warn("cannot track reader position", "slug", title.Slug, "chapter", key, "error", err)
			return
		}
		if id != 0 {
			r.trackedID.Store(id)
		}
	}()
}

// selectChapter switches to a chapter in the current translation.
func (r *readerScreen) selectChapter(index int) {
	if index < 0 || index >= len(r.chapters) || index == r.selectedChapter {
		return
	}
	r.selectedChapter = index
	r.loadChapter()
}

// readerChapterCacheKey keeps translations apart: the same chapter number can
// point at a different branch and different pages in another team.
func readerChapterCacheKey(voice, key string) string {
	return voice + "\x00" + key
}

// requestChapter queues one chapter body. The cache and pending set are owned
// by the UI goroutine; the worker only sends the result back through the
// channel.
func (r *readerScreen) requestChapter(voice, key string, title catalog.Title) {
	if r.source == nil || key == "" {
		return
	}
	cacheKey := readerChapterCacheKey(voice, key)
	if _, ok := r.chapterCache[cacheKey]; ok {
		return
	}
	if r.chapterPending == nil {
		r.chapterPending = make(map[string]bool)
	}
	if r.chapterPending[cacheKey] {
		return
	}
	r.chapterPending[cacheKey] = true

	ctx := r.contentCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		content, err := r.source.ReadChapter(ctx, title, voice, key)
		if err != nil {
			err = errReaderChapter.wrap(err)
		}
		r.chapterResults <- readerChapterResult{
			voice:   voice,
			key:     key,
			content: content,
			err:     err,
		}
	}()
}

// prefetchChapters keeps the current chapter and five on each side ready. The
// previous side is useful when the user pages back; the forward side makes the
// next click instant. Chapters outside this window are fetched only after they
// become part of a new window.
func (r *readerScreen) prefetchChapters() {
	if r.source == nil || r.selectedChapter < 0 || len(r.chapters) == 0 {
		return
	}
	voice := r.translationName()
	title := r.title
	start := max(0, r.selectedChapter-readerChapterWindow)
	end := min(len(r.chapters)-1, r.selectedChapter+readerChapterWindow)
	for index := start; index <= end; index++ {
		r.requestChapter(voice, r.chapters[index], title)
	}
}

func (r *readerScreen) applyChapterContent(content catalog.ReadingContent) {
	r.content = content
	r.loadErr = ""
	r.pages = make([]readerPageState, len(content.Pages))
	for index, page := range content.Pages {
		r.pages[index] = readerPageState{url: page.URL}
	}
	r.mangaList.Position = layout.Position{}
	r.ranobeList.Position = layout.Position{}
}

// selectTranslation changes the team and keeps the nearest saved chapter. A
// saved chapter may not exist in the new team; it then falls back to its first.
func (r *readerScreen) selectTranslation(index int) {
	if index < 0 || index >= len(r.translations) || index == r.selectedVoice {
		return
	}
	saved := r.currentChapterKey()
	r.selectedVoice = index
	r.rebuildChapters()
	r.selectedChapter = -1
	for i := range r.chapters {
		if r.chapters[i] == saved {
			r.selectedChapter = i
			break
		}
	}
	if r.selectedChapter < 0 {
		r.selectedChapter = 0
	}
	r.loadChapter()
}

// dismissPopup closes the translation dropdown when the window loses focus.
func (r *readerScreen) dismissPopup() {
	r.openTranslations = false
}

func (r *readerScreen) animationsRun() bool {
	return r.hidden == nil || !r.hidden()
}

// requestFrame repaints the window after a background result arrives. The
// application injects the window invalidator; tests leave it nil.
func (r *readerScreen) requestFrame() {
	if r.invalidate != nil {
		r.requestFrame()
	}
}

func (r *readerScreen) Close() {
	r.cancelImages()
	if r.contentCancel != nil {
		r.contentCancel()
		r.contentCancel = nil
	}
}

func (r *readerScreen) cancelImages() {
	if r.imageCancel != nil {
		r.imageCancel()
		r.imageCancel = nil
	}
}

// updateChapterResult applies finished chapter fetches on the UI thread.
// Successful prefetches fill the session cache even when they are not the
// visible chapter; errors only reach the header for the current request.
func (r *readerScreen) updateChapterResult(gtx layout.Context) {
	for {
		select {
		case result := <-r.chapterResults:
			cacheKey := readerChapterCacheKey(result.voice, result.key)
			delete(r.chapterPending, cacheKey)
			current := result.voice == r.translationName() && result.key == r.currentChapterKey()
			if result.err != nil {
				if current {
					r.loading = false
					r.loadErr = userErrorText(result.err)
					if r.onError != nil {
						r.onError(result.err)
					}
					if r.animationsRun() {
						gtx.Execute(op.InvalidateCmd{})
					}
				} else {
					logger.Debug("reader prefetch failed", "slug", r.title.Slug, "translation", result.voice, "chapter", result.key, "error", result.err)
				}
				continue
			}
			if r.chapterCache == nil {
				r.chapterCache = make(map[string]catalog.ReadingContent)
			}
			r.chapterCache[cacheKey] = result.content
			if !current {
				continue
			}
			r.loading = false
			r.applyChapterContent(result.content)
			if r.animationsRun() {
				gtx.Execute(op.InvalidateCmd{})
			}
		default:
			return
		}
	}
}

// updateImages applies finished page downloads, discarding replies that belong
// to an older generation.
func (r *readerScreen) updateImages(gtx layout.Context) {
	for {
		select {
		case result := <-r.imageResults:
			if result.generation != r.loadGeneration || result.index < 0 || result.index >= len(r.pages) {
				continue
			}
			page := &r.pages[result.index]
			page.loading = false
			if result.err != nil {
				page.failed = true
			} else {
				page.img = result.img
				page.failed = false
			}
			if r.animationsRun() {
				gtx.Execute(op.InvalidateCmd{})
			}
		default:
			return
		}
	}
}

// downloadPage starts fetching one manga page. Concurrency is capped by the
// semaphore so a long chapter does not hammer the CDN.
func (r *readerScreen) downloadPage(index int) {
	generation := r.loadGeneration
	if r.pages[index].loading || r.pages[index].img != nil {
		return
	}
	r.pages[index].loading = true
	url := r.pages[index].url
	title := r.title
	go func() {
		r.imageSem <- struct{}{}
		defer func() { <-r.imageSem }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		img, err := r.source.ReadingImage(ctx, title, url)
		r.imageResults <- readerImageResult{generation: generation, index: index, img: img, err: err}
	}()
}

func (r *readerScreen) layout(gtx layout.Context) {
	if !r.focusCleared {
		r.focusCleared = true
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	r.updateChapterResult(gtx)
	r.updateImages(gtx)
	// Keyboard navigation: only when the search editor is not focused.
	if !gtx.Focused(&r.search) {
		r.handleReaderKeys(gtx)
	}
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	r.applySearchFilter()
	if r.clear.Clicked(gtx) {
		r.search.SetText("")
		r.lastQuery = ""
		r.refreshFiltered()
	}

	width := gtx.Dp(readerSidebarWidth)
	if width < 0 || width > size.X-120 {
		width = size.X * 2 / 3
	}
	reader := image.Rect(0, 0, size.X, size.Y)
	r.layoutSidebar(gtx, image.Rect(reader.Min.X, reader.Min.Y, reader.Min.X+width, reader.Max.Y))
	r.layoutReaderArea(gtx, image.Rect(reader.Min.X+width, reader.Min.Y, reader.Max.X, reader.Max.Y))

	if r.openTranslations {
		r.layoutTranslationMenu(gtx, size, image.Rect(reader.Min.X, reader.Min.Y, reader.Min.X+width, reader.Max.Y))
	}
}

// handleReaderKeys maps Left/Right to chapter navigation. Up/Down scroll the
// content list through the normal focus; PageUp/PageDown are left to the list.
func (r *readerScreen) handleReaderKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(key.Filter{
			Focus: &r,
			Name:  key.NameLeftArrow,
		}, key.Filter{
			Focus: &r,
			Name:  key.NameRightArrow,
		})
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameLeftArrow:
			r.selectChapter(r.selectedChapter - 1)
		case key.NameRightArrow:
			r.selectChapter(r.selectedChapter + 1)
		}
	}
}

// layoutSidebar draws the title, translation selector, search field and the
// scrollable chapter list.
func (r *readerScreen) layoutSidebar(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}
	border := gtx.Dp(unit.Dp(1))
	fillRounded(gtx, readerPanelBorder, local, 0)
	fill(gtx, readerPanelFill, local.Inset(border))
	pad := gtx.Dp(readerPanelPad)
	top := pad
	width := local.Dx() - 2*pad
	if width <= 0 {
		return
	}

	// Title.
	face := font.Font{Typeface: fontFamily, Weight: font.Bold}
	titleText := r.item.title
	if titleText == "" {
		titleText = "Тайтл"
	}
	_, ink := r.typo.measure(gtx, titleText, face, unit.Sp(15))
	if ink.Dy() > 0 {
		r.typo.draw(gtx, titleText, face, unit.Sp(15), readerTitleColor, image.Pt(pad, top))
		top += ink.Dy() + gtx.Dp(unit.Dp(4))
	}

	// Chapter counter caption.
	medium := font.Font{Typeface: fontFamily, Weight: font.Medium}
	countText := ""
	if r.selectedVoice >= 0 && r.selectedVoice < len(r.translations) {
		countText = fmt.Sprintf("%d гл.", r.translations[r.selectedVoice].count)
	}
	_, countInk := r.typo.measure(gtx, countText, medium, unit.Sp(11))
	if countInk.Dy() > 0 {
		r.typo.draw(gtx, countText, medium, unit.Sp(11), readerMutedColor, image.Pt(pad, top))
		top += countInk.Dy() + gtx.Dp(unit.Dp(8))
	}

	// Translation selector button.
	controlHeight := gtx.Dp(readerControlHeight)
	translationRect := image.Rect(pad, top, pad+width, top+controlHeight)
	if r.layoutTranslationTrigger(gtx, translationRect) {
		r.openTranslations = !r.openTranslations
		gtx.Execute(op.InvalidateCmd{})
	}
	top += controlHeight + gtx.Dp(unit.Dp(8))

	// Search field.
	searchHeight := gtx.Dp(readerSearchHeight)
	searchRect := image.Rect(pad, top, pad+width, top+searchHeight)
	r.layoutSearchField(gtx, searchRect)
	top += searchHeight + gtx.Dp(unit.Dp(6))

	// Chapter list.
	listRect := image.Rect(pad, top, pad+width, local.Dy()-pad)
	r.layoutChapterList(gtx, listRect)
}

// layoutTranslationMenu paints the translation dropdown as an overlay on the
// sidebar. Clicking anywhere outside closes it; selecting a row switches the
// team and reloads the chapter.
func (r *readerScreen) layoutTranslationMenu(gtx layout.Context, size image.Point, sidebar image.Rectangle) {
	if sidebar.Empty() || len(r.translations) == 0 {
		r.openTranslations = false
		return
	}
	if len(r.translationClickables) != len(r.translations) {
		r.translationClickables = make([]widget.Clickable, len(r.translations))
	}
	pad := gtx.Dp(readerPanelPad)
	itemHeight := gtx.Dp(unit.Dp(36))
	itemGap := gtx.Dp(unit.Dp(2))
	menuPad := gtx.Dp(unit.Dp(4))
	height := len(r.translations)*itemHeight + len(r.translations)*itemGap + 2*menuPad
	anchor := image.Rect(
		sidebar.Min.X+pad,
		sidebar.Min.Y+pad,
		sidebar.Max.X-pad,
		sidebar.Min.Y+pad+gtx.Dp(readerControlHeight),
	)
	menu := placePopupList(anchor, height, gtx.Dp(unit.Dp(4)), image.Rectangle{Max: size})
	if layoutOverlayScrim(gtx, image.Rectangle{Max: size}, &r.translationScrim) {
		r.openTranslations = false
		gtx.Execute(op.InvalidateCmd{})
		return
	}
	drawPopupMenu(gtx, menu)
	for index, translation := range r.translations {
		item := image.Rect(
			menu.Min.X+menuPad,
			menu.Min.Y+menuPad+index*(itemHeight+itemGap),
			menu.Max.X-menuPad,
			menu.Min.Y+menuPad+index*(itemHeight+itemGap)+itemHeight,
		)
		if r.layoutTranslationItem(gtx, item, index, translation) {
			r.selectTranslation(index)
			r.openTranslations = false
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (r *readerScreen) layoutTranslationItem(gtx layout.Context, rect image.Rectangle, index int, translation readerTranslation) bool {
	if rect.Empty() {
		return false
	}
	click := &r.translationClickables[index]
	clicked := layoutClickableRect(gtx, rect, click, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(6))
		if index == r.selectedVoice {
			fillRounded(gtx, settingsMenuSelected, local, radius)
		} else if click.Hovered() {
			fillRounded(gtx, tokenDomainFillHover, local, radius)
		}
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		name := translation.name
		if name == "" {
			name = "Без команды"
		}
		label := fmt.Sprintf("%s · %d", name, translation.count)
		padX := gtx.Dp(unit.Dp(10))
		_, ink := r.typo.measure(gtx, label, face, unit.Sp(12.5))
		textColor := mainTextColor
		if index == r.selectedVoice {
			textColor = badgeTextColor
		}
		r.typo.draw(gtx, label, face, unit.Sp(12.5), textColor, image.Pt(padX, (local.Dy()-ink.Dy())/2))
		if index == r.selectedVoice {
			check := image.Pt(local.Dx()-padX-gtx.Dp(unit.Dp(7)), local.Dy()/2)
			drawCheckIcon(gtx, check, gtx.Dp(unit.Dp(13)), badgeTextColor)
		}
	})
	return clicked
}

func (r *readerScreen) layoutTranslationTrigger(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	clicked := r.translationClick.Clicked(gtx)
	active := r.translationClick.Hovered() || r.openTranslations
	padX := gtx.Dp(unit.Dp(10))
	layoutClickableRect(gtx, rect, &r.translationClick, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(7))
		fill := tokenDomainFill
		if active {
			fill = tokenDomainFillHover
		}
		fillRounded(gtx, tokenDomainBorder, local, radius)
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		name := r.translationName()
		if name == "" {
			name = "Перевод"
		}
		_, ink := r.typo.measure(gtx, name, face, unit.Sp(12.5))
		r.typo.draw(gtx, name, face, unit.Sp(12.5), mainTextColor, image.Pt(padX, (local.Dy()-ink.Dy())/2))
		size := gtx.Dp(unit.Dp(9))
		center := image.Pt(local.Dx()-padX-size/2, local.Dy()/2)
		if active {
			drawChevronUp(gtx, center, size, gtx.Dp(unit.Dp(1.6)), settingsNoteColor)
		} else {
			drawChevronDown(gtx, center, size, gtx.Dp(unit.Dp(1.6)), settingsNoteColor)
		}
	})
	return clicked
}

// layoutSearchField draws the editable chapter filter with a magnifier icon and
// a clear button.
func (r *readerScreen) layoutSearchField(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	local := image.Rectangle{Max: rect.Size()}
	radius := gtx.Dp(unit.Dp(7))
	border := readerPanelBorder
	if gtx.Focused(&r.search) {
		border = mainInputFocusBorder
	}
	fillRounded(gtx, border, local, radius)
	fillRounded(gtx, mainInputColor, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	padX := gtx.Dp(unit.Dp(8))
	icon := image.Pt(padX+gtx.Dp(unit.Dp(8)), local.Dy()/2)
	drawSearchIcon(gtx, icon, gtx.Dp(unit.Dp(12)), mainMutedColor)

	textLeft := icon.X + gtx.Dp(unit.Dp(10))
	clearW := gtx.Dp(unit.Dp(28))
	editRect := image.Rect(textLeft, 0, local.Dx()-clearW, local.Dy())
	stack := op.Offset(editRect.Min).Push(gtx.Ops)
	editorGtx := gtx
	editorGtx.Constraints = layout.Exact(editRect.Size())
	r.search.Layout(
		editorGtx,
		r.typo.shaper,
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		unit.Sp(13),
		colorMaterial(gtx, mainTextColor),
		colorMaterial(gtx, mainSelectionColor),
	)
	stack.Pop()
	if r.search.Len() == 0 {
		r.typo.draw(gtx, "Глава или номер…", font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(13), mainMutedColor, image.Pt(textLeft, (local.Dy()-gtx.Dp(unit.Dp(16)))/2+gtx.Dp(unit.Dp(2))))
	}

	if len(r.search.Text()) > 0 {
		clearCenter := image.Pt(local.Dx()-clearW/2, local.Dy()/2)
		if layoutClickableRect(gtx, rectFromCenter(clearCenter, gtx.Dp(unit.Dp(22))), &r.clear, func(gtx layout.Context, local image.Rectangle) {
			drawCloseIcon(gtx, centerOf(local), gtx.Dp(unit.Dp(11)), mainMutedColor)
		}) {
			r.search.SetText("")
			r.lastQuery = ""
			r.refreshFiltered()
		}
	}
}

// layoutChapterList draws the filtered list of chapters.
func (r *readerScreen) layoutChapterList(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	if len(r.filtered) == 0 {
		r.typo.draw(gtx, "Ничего не найдено", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(8))))
		return
	}
	if len(r.chapterClick) != len(r.chapters) {
		r.chapterClick = make([]widget.Clickable, len(r.chapters))
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	r.chapterList.Layout(cgtx, len(r.filtered), func(gtx layout.Context, index int) layout.Dimensions {
		key := r.filtered[index]
		// Map back to the clickable for the real chapter index.
		chapterIndex := chapterIndexForKey(r.chapters, key)
		rowHeight := gtx.Dp(readerChapterHeight)
		item := image.Rect(0, 0, rect.Dx(), rowHeight)
		if r.layoutChapterRow(gtx, item, key, chapterIndex == r.selectedChapter) {
			r.selectChapter(chapterIndex)
		}
		return layout.Dimensions{Size: item.Size()}
	})
}

func (r *readerScreen) layoutChapterRow(gtx layout.Context, rect image.Rectangle, key string, selected bool) bool {
	if rect.Empty() {
		return false
	}
	index := chapterIndexForKey(r.chapters, key)
	if index < 0 {
		return false
	}
	click := &r.chapterClick[index]
	row := image.Rect(
		rect.Min.X+gtx.Dp(unit.Dp(4)),
		rect.Min.Y+gtx.Dp(unit.Dp(2)),
		rect.Max.X-gtx.Dp(unit.Dp(4)),
		rect.Max.Y-gtx.Dp(unit.Dp(2)),
	)
	clicked := layoutClickableRect(gtx, row, click, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(6))
		bg := readerChapterHover
		if selected {
			bg = readerChapterActive
		}
		fillRounded(gtx, bg, local, radius)
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		textColor := mainTextColor
		if selected {
			textColor = badgeTextColor
		}
		label := readingChapterLabel(key)
		padX := gtx.Dp(unit.Dp(10))
		_, ink := r.typo.measure(gtx, label, face, unit.Sp(12.5))
		r.typo.draw(gtx, label, face, unit.Sp(12.5), textColor, image.Pt(padX, (local.Dy()-ink.Dy())/2))
	})
	return clicked
}

// layoutReaderArea draws the right side: the chapter header (or loading/error)
// and the scrollable page or paragraph content.
func (r *readerScreen) layoutReaderArea(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	local := image.Rectangle{Max: rect.Size()}

	padX := gtx.Dp(readerPanelPad)
	headerH := gtx.Dp(readerHeaderHeight)

	// Header with the current chapter label.
	label := ""
	if r.selectedChapter >= 0 && r.selectedChapter < len(r.chapters) {
		label = readingChapterLabel(r.chapters[r.selectedChapter])
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Bold}
	r.typo.draw(gtx, label, face, unit.Sp(14), mainTextColor, image.Pt(padX, gtx.Dp(unit.Dp(10))))
	fill(gtx, readerRowDivider, image.Rect(padX, headerH-gtx.Dp(unit.Dp(1)), local.Dx()-padX, headerH))

	if r.loading {
		r.layoutLoading(gtx, local, headerH)
		return
	}
	if r.loadErr != "" {
		r.layoutLoadError(gtx, local, headerH)
		return
	}
	contentTop := headerH + gtx.Dp(unit.Dp(6))
	contentRect := image.Rect(padX, contentTop, local.Dx()-padX, local.Dy())
	if r.title.IsRanobe() {
		r.layoutRanobe(gtx, contentRect)
	} else {
		r.layoutManga(gtx, contentRect)
	}
}

func (r *readerScreen) layoutLoading(gtx layout.Context, rect image.Rectangle, top int) {
	center := image.Pt(rect.Dx()/2, top+(rect.Dy()-top)/2)
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 3)})
	now := float64(time.Now().UnixNano()) / 1e9
	phase := now*2.2 - float64(int(now*2.2))
	drawSpinner(gtx, center, gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(3)), phase, mainAccentColor1)
	r.typo.drawCentered(gtx, "Загружаем главу…", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12.5), mainMutedColor, image.Pt(rect.Dx()/2, center.Y+gtx.Dp(unit.Dp(30))))
}

func (r *readerScreen) layoutLoadError(gtx layout.Context, rect image.Rectangle, top int) {
	centerY := top + (rect.Dy()-top)/2
	r.typo.drawCentered(gtx, "Не удалось загрузить главу", font.Font{Typeface: fontFamily, Weight: font.Bold}, unit.Sp(15), mainDangerColor, image.Pt(rect.Dx()/2, centerY))
	return
}

// layoutManga renders the lazy page image list. Pages are fetched on demand as
// they enter the viewport.
func (r *readerScreen) layoutManga(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	if len(r.pages) == 0 {
		r.typo.draw(gtx, "В главе нет страниц", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(13), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(12))))
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	r.mangaList.Layout(cgtx, len(r.pages), func(gtx layout.Context, index int) layout.Dimensions {
		page := &r.pages[index]
		if page.img == nil {
			r.downloadPage(index)
		}
		width := rect.Dx()
		height := gtx.Dp(unit.Dp(20))
		if page.img != nil {
			b := page.img.Bounds()
			if b.Dx() > 0 {
				height = width * b.Dy() / b.Dx()
			}
		} else if page.loading {
			height = gtx.Dp(unit.Dp(120))
		}
		if height < gtx.Dp(unit.Dp(40)) {
			height = gtx.Dp(unit.Dp(40))
		}
		item := image.Rect(0, 0, width, height)
		if page.img != nil {
			drawImage(gtx, page.img, item)
		} else if page.failed {
			r.typo.drawCentered(gtx, "Не удалось загрузить", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12), mainMutedColor, centerOf(item))
		} else {
			phase := float64(time.Now().UnixNano()) / 1e9
			f := phase*2 - float64(int(phase*2))
			drawSpinner(gtx, centerOf(item), gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(2)), f, mainAccentColor1)
		}
		return layout.Dimensions{Size: item.Size()}
	})
}

// layoutRanobe renders the paragraphs wrapped at the content width.
func (r *readerScreen) layoutRanobe(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	paragraphs := r.content.Paragraphs
	if len(paragraphs) == 0 {
		r.typo.draw(gtx, "В главе нет текста", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(13), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(12))))
		return
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	width := rect.Dx()
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	gap := gtx.Dp(readerParaGap)
	r.ranobeList.Layout(cgtx, len(paragraphs), func(gtx layout.Context, index int) layout.Dimensions {
		text := paragraphs[index]
		if strings.TrimSpace(text) == "" {
			return layout.Dimensions{Size: image.Pt(width, gtx.Dp(unit.Dp(10)))}
		}
		_, ink := r.typo.measureWrapped(gtx, text, face, unit.Sp(14), width)
		h := ink.Dy()
		if h < gtx.Dp(unit.Dp(18)) {
			h = gtx.Dp(unit.Dp(18))
		}
		r.typo.drawWrapped(gtx, text, face, unit.Sp(14), mainTextColor, image.Pt(0, 0), width)
		return layout.Dimensions{Size: image.Pt(width, h+gap)}
	})
}

func (r *readerScreen) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(titlePageButtonHeight)
	top := (size.Y - height) / 2

	backWidth := gtx.Dp(unit.Dp(128))
	backRect := image.Rect(padX, top, padX+backWidth, top+height)
	if r.back.layout(gtx, r.typo, backRect) {
		if r.onBack != nil {
			r.onBack()
		}
		return
	}

	navWidth := gtx.Dp(unit.Dp(112))
	gap := gtx.Dp(unit.Dp(10))
	right := size.X - padX
	nextRect := image.Rect(right-navWidth, top, right, top+height)
	prevRect := image.Rect(nextRect.Min.X-gap-navWidth, top, nextRect.Min.X-gap, top+height)

	hasNext := r.selectedChapter >= 0 && r.selectedChapter+1 < len(r.chapters)
	hasPrev := r.selectedChapter > 0
	r.next.disabled = !hasNext
	r.prev.disabled = !hasPrev
	if !r.loading {
		if r.next.layout(gtx, r.typo, nextRect) {
			r.selectChapter(r.selectedChapter + 1)
		}
		if r.prev.layout(gtx, r.typo, prevRect) {
			r.selectChapter(r.selectedChapter - 1)
		}
	}
}
