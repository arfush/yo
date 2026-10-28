package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

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
	readerHeaderHeight = unit.Dp(48)
	// readerParaGap separates ranobe paragraphs; the concept leaves one
	// generous blank line between them.
	readerParaGap = unit.Dp(18)
	// readerTextSize and readerTextLeading define the ranobe body: a roomy
	// 15sp with a few extra pixels after every wrapped line.
	readerTextSize    = unit.Sp(15)
	readerTextLeading = unit.Dp(3)
	// readerMaxTextureSide keeps one GPU texture below the Direct3D limit.
	// Webtoon pages are often 760x25000; uploading them as one texture makes
	// Gio panic with CreateTexture2D 0x80070057 before anything is visible.
	readerMaxTextureSide = 4096
	// readerPageMaxWidth keeps manga pages in a readable column instead of
	// stretching a 760px scan across a wide window.
	readerPageMaxWidth = unit.Dp(920)
	// readerTextMaxWidth caps the ranobe column like the concept mockup,
	// where the text sits on ~73% of the window width with even margins.
	readerTextMaxWidth = unit.Dp(720)
	// readerChapterWindow is the number of chapters kept ready on each side of
	// the current one. Moving one chapter forward/back does not wait for the
	// network because the next window was already requested.
	readerChapterWindow = 5
	// chapterFetchConcurrency caps how many library chapter requests the
	// prefetch runs at once, so a window load does not burst the AnimeLIB
	// rate limiter with the whole window at the same time.
	chapterFetchConcurrency = 3
)

var (
	// readerTextColor and readerFaintColor follow the ranobe concept: the body
	// uses a soft white, while the header, the intro block before the first
	// separator and the star rules stay in the faint grey.
	readerTextColor  = color.NRGBA{R: 0xd5, G: 0xd3, B: 0xdc, A: 0xff}
	readerFaintColor = color.NRGBA{R: 0x89, G: 0x83, B: 0x91, A: 0xff}
)

// starSeparator reports whether a paragraph is a horizontal rule made of
// asterisks ("***", "* * *", "**"). Ranobe chapters use them between scenes.
func starSeparator(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed != "" && strings.Trim(trimmed, "* \t") == ""
}

// introEndIndex returns the index of the first star separator. Paragraphs
// before it are the chapter intro and keep the faint colour, like the concept
// paints the credit block.
func introEndIndex(paragraphs []string) int {
	for index, text := range paragraphs {
		if starSeparator(text) {
			return index
		}
	}
	return len(paragraphs)
}

// buildRanobeRows interleaves the text paragraphs and the illustrations into
// the row list the reader paints. Every picture lands after the paragraphs
// that precede it in the document.
func buildRanobeRows(paragraphs int, images []catalog.ReadingImage) []readerRanobeRow {
	if len(images) > 1 {
		sort.SliceStable(images, func(i, j int) bool { return images[i].After < images[j].After })
	}
	rows := make([]readerRanobeRow, 0, paragraphs+len(images))
	cursor := 0
	for index := 0; index < paragraphs; index++ {
		for cursor < len(images) && images[cursor].After <= index {
			rows = append(rows, readerRanobeRow{image: true, index: cursor})
			cursor++
		}
		rows = append(rows, readerRanobeRow{index: index})
	}
	for ; cursor < len(images); cursor++ {
		rows = append(rows, readerRanobeRow{image: true, index: cursor})
	}
	return rows
}

// fitPageWidth narrows a page so its full height fits the viewport when the
// aspect allows it. Very tall webtoon strips keep the column width: shrinking
// them to a sliver would not make them readable.
func fitPageWidth(width, height, columnWidth, viewportHeight int) int {
	if width <= 0 || height <= 0 || viewportHeight <= 0 {
		return columnWidth
	}
	fitted := viewportHeight * width / height
	if fitted <= 0 || fitted < columnWidth*50/100 {
		return columnWidth
	}
	return min(fitted, columnWidth)
}

// readerNavZone turns the left and right thirds of the reader into invisible
// chapter-turn areas, keeping the reader free of buttons like the concept. A
// press only navigates when the pointer barely moved, so scroll drags are not
// mistaken for clicks.
type readerNavZone struct {
	tag     int
	pressed bool
	start   f32.Point
}

// readerPageState is one manga page in the virtualised list.
type readerPageState struct {
	url     string
	img     image.Image
	chunks  []image.Image
	width   int
	height  int
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

// readerIllustration is one ranobe picture in the current chapter, mirroring
// the lazy page state of the manga reader.
type readerIllustration struct {
	url     string
	img     image.Image
	chunks  []image.Image
	width   int
	height  int
	loading bool
	failed  bool
}

type readerIllustrationResult struct {
	generation int
	index      int
	img        image.Image
	err        error
}

// readerRanobeRow is one row of the ranobe list: either a text paragraph or an
// illustration. For text rows index addresses content.Paragraphs, for image
// rows it addresses the illustrations slice.
type readerRanobeRow struct {
	image bool
	index int
}

// readerMangaRow is one virtualised list row. Tall scans are split into
// several chunk rows so the GPU only receives textures that are near the
// viewport, instead of uploading a whole 25000px page at once.
type readerMangaRow struct {
	page  int
	chunk int
}

// readerScreen is the manga and ranobe reader. Per the concept it shows only
// the chapter itself: a small header with the current volume and chapter and
// the full-width page column or paragraph list. Navigation is keyboard-based
// (arrows for chapters, Space/PageUp/PageDown for scrolling, Escape to leave);
// chapter picking happens on the reading title screen.
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
	// chapters are the keys available in the selected translation.
	chapters []string

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
	chapterSem     chan struct{}
	pages          []readerPageState
	mangaRows      []readerMangaRow
	mangaRowsDirty bool
	mangaList      layout.List
	ranobeList     layout.List
	// illustrations holds the ranobe pictures of the current chapter; the
	// results channel and generation guard mirror the manga page machinery.
	illustrations       []readerIllustration
	illustrationResults chan readerIllustrationResult
	// mangaVisRows and ranobeVisRows are the number of document rows the last
	// visible layout drew; Space/PageUp/PageDown page by nearly that amount.
	mangaVisRows  int
	ranobeVisRows int

	// prevZone and nextZone are the invisible chapter-turn halves of the
	// reader; the concept leaves the reader without any buttons.
	prevZone readerNavZone
	nextZone readerNavZone

	focusCleared bool

	hidden     func() bool
	onBack     func()
	onError    func(error)
	invalidate func()
}

func newReaderScreen(typo *typography, item historyTitle, source readerSource) *readerScreen {
	contentCtx, contentCancel := context.WithCancel(context.Background())
	r := &readerScreen{
		typo:                typo,
		item:                item,
		source:              source,
		section:             item.section(),
		chapterResults:      make(chan readerChapterResult, 32),
		chapterCache:        make(map[string]catalog.ReadingContent),
		chapterPending:      make(map[string]bool),
		contentCtx:          contentCtx,
		contentCancel:       contentCancel,
		imageResults:        make(chan readerImageResult, 32),
		imageSem:            make(chan struct{}, 3),
		chapterSem:          make(chan struct{}, chapterFetchConcurrency),
		illustrationResults: make(chan readerIllustrationResult, 32),
	}
	r.trackedID.Store(item.id)
	r.mangaList.Axis = layout.Vertical
	r.ranobeList.Axis = layout.Vertical
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
	if r.selectedVoice < 0 || r.selectedVoice >= len(title.Voiceovers) {
		r.selectedVoice = 0
	}
	if r.selectedVoice >= 0 && r.selectedVoice < len(title.Voiceovers) {
		r.chapters = title.Voiceovers[r.selectedVoice].Episodes
	} else {
		r.chapters = nil
	}
	r.selectedChapter = episodeIndex(r.item, title.SelectedEpisode)
	if r.selectedChapter < 0 {
		r.selectedChapter = 0
	}
	r.loadChapter()
}

// fullHeight claims the shell footer strip: the reader must show nothing but
// the chapter, so the empty chrome band is given to the content.
func (r *readerScreen) fullHeight() {}

func (r *readerScreen) translationName() string {
	if r.selectedVoice >= 0 && r.selectedVoice < len(r.title.Voiceovers) {
		return r.title.Voiceovers[r.selectedVoice].Name
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
	for len(r.illustrationResults) > 0 {
		<-r.illustrationResults
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
		// Fetching chapters in a burst (the whole prefetch window at once)
		// hammered the library API and the AnimeLIB rate limiter. The
		// semaphore keeps a window load to a few in-flight requests; the
		// rest queue in the worker.
		r.chapterSem <- struct{}{}
		defer func() { <-r.chapterSem }()
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
		r.pages[index] = readerPageState{
			url:    page.URL,
			width:  page.Width,
			height: page.Height,
		}
	}
	r.illustrations = make([]readerIllustration, 0, len(content.Images))
	for _, image := range content.Images {
		r.illustrations = append(r.illustrations, readerIllustration{url: image.URL})
	}
	r.mangaRowsDirty = true
	r.mangaList.Position = layout.Position{}
	r.ranobeList.Position = layout.Position{}
}

func (r *readerScreen) animationsRun() bool {
	return r.hidden == nil || !r.hidden()
}

// requestFrame repaints the window after a background result arrives. The
// application injects the window invalidator; tests leave it nil.
func (r *readerScreen) requestFrame() {
	if r.invalidate != nil {
		r.invalidate()
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
			if current {
				r.loading = false
				r.applyChapterContent(result.content)
				if r.animationsRun() {
					gtx.Execute(op.InvalidateCmd{})
				}
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
				if result.img != nil {
					bounds := result.img.Bounds()
					page.width = bounds.Dx()
					page.height = bounds.Dy()
				}
				page.chunks = splitReaderImage(result.img, readerMaxTextureSide)
				// The chunks own their pixels. Keeping the decoded parent as
				// well would double the memory of every webtoon page.
				page.img = nil
				page.failed = false
				r.mangaRowsDirty = true
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
	if r.pages[index].loading || r.pages[index].img != nil || len(r.pages[index].chunks) > 0 {
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

// requestIllustration starts fetching one ranobe picture. The same semaphore
// bounds the requests as for manga pages.
func (r *readerScreen) requestIllustration(index int) {
	generation := r.loadGeneration
	if r.source == nil || index < 0 || index >= len(r.illustrations) {
		return
	}
	ill := &r.illustrations[index]
	if ill.loading || ill.img != nil || len(ill.chunks) > 0 {
		return
	}
	ill.loading = true
	url := ill.url
	title := r.title
	go func() {
		r.imageSem <- struct{}{}
		defer func() { <-r.imageSem }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		img, err := r.source.ReadingImage(ctx, title, url)
		r.illustrationResults <- readerIllustrationResult{generation: generation, index: index, img: img, err: err}
	}()
}

// updateIllustrationResults applies finished ranobe pictures, discarding
// replies that belong to an older chapter.
func (r *readerScreen) updateIllustrationResults(gtx layout.Context) {
	for {
		select {
		case result := <-r.illustrationResults:
			if result.generation != r.loadGeneration || result.index < 0 || result.index >= len(r.illustrations) {
				continue
			}
			ill := &r.illustrations[result.index]
			ill.loading = false
			if result.err != nil {
				ill.failed = true
			} else {
				if result.img != nil {
					bounds := result.img.Bounds()
					ill.width = bounds.Dx()
					ill.height = bounds.Dy()
				}
				ill.chunks = splitReaderImage(result.img, readerMaxTextureSide)
				ill.img = nil
				ill.failed = false
			}
			if r.animationsRun() {
				gtx.Execute(op.InvalidateCmd{})
			}
		default:
			return
		}
	}
}

func (r *readerScreen) layout(gtx layout.Context) {
	if !r.focusCleared {
		r.focusCleared = true
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	r.updateChapterResult(gtx)
	r.updateImages(gtx)
	r.updateIllustrationResults(gtx)
	r.handleReaderKeys(gtx)
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	headerH := gtx.Dp(readerHeaderHeight)
	r.layoutHeader(gtx, size, headerH)
	contentTop := headerH + gtx.Dp(unit.Dp(6))
	r.layoutNavZones(gtx, size, contentTop)
	if r.loading {
		r.layoutLoading(gtx, image.Rectangle{Max: size}, headerH)
		return
	}
	if r.loadErr != "" {
		r.layoutLoadError(gtx, image.Rectangle{Max: size}, headerH)
		return
	}
	content := image.Rect(0, contentTop, size.X, size.Y)
	if r.title.IsRanobe() {
		r.layoutRanobe(gtx, content)
	} else {
		r.layoutManga(gtx, content)
	}
}

// layoutHeader draws the current volume and chapter centred on the strip, with
// a section-coloured dot in front. The concept keeps the line thin and faint:
// a normal-weight label in the muted grey, so it does not compete with the
// content.
func (r *readerScreen) layoutHeader(gtx layout.Context, size image.Point, headerH int) {
	label := ""
	if r.selectedChapter >= 0 && r.selectedChapter < len(r.chapters) {
		label = readingChapterLabel(r.chapters[r.selectedChapter])
	}
	if label == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	_, ink := r.typo.measure(gtx, label, face, unit.Sp(12.5))
	if ink.Dy() <= 0 {
		return
	}
	dot := gtx.Dp(unit.Dp(4))
	gap := gtx.Dp(unit.Dp(6))
	startX := (size.X - (ink.Dx() + gap + dot)) / 2
	labelTop := (headerH - ink.Dy()) / 2
	r.typo.draw(gtx, label, face, unit.Sp(12.5), readerFaintColor, image.Pt(startX+dot+gap, labelTop))
	radius := dot / 2
	fillRounded(gtx, mainAccentColor1, image.Rect(startX, headerH/2-radius, startX+dot, headerH/2+radius), radius)
}

// layoutNavZones makes the left and right thirds of the reader switch
// chapters, as the concept leaves the reader without visible buttons.
func (r *readerScreen) layoutNavZones(gtx layout.Context, size image.Point, top int) {
	if size.Y <= top {
		return
	}
	zoneWidth := size.X * 30 / 100
	if zoneWidth <= 0 {
		return
	}
	r.layoutNavZone(gtx, image.Rect(0, top, zoneWidth, size.Y), &r.prevZone, -1)
	r.layoutNavZone(gtx, image.Rect(size.X-zoneWidth, top, size.X, size.Y), &r.nextZone, 1)
}

func (r *readerScreen) layoutNavZone(gtx layout.Context, rect image.Rectangle, zone *readerNavZone, direction int) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	event.Op(cgtx.Ops, &zone.tag)
	for {
		ev, ok := cgtx.Event(pointer.Filter{
			Target: &zone.tag,
			Kinds:  pointer.Press | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			zone.pressed = true
			zone.start = e.Position
		case pointer.Release:
			dx := e.Position.X - zone.start.X
			dy := e.Position.Y - zone.start.Y
			if zone.pressed && dx*dx+dy*dy < 12*12 {
				r.selectChapter(r.selectedChapter + direction)
			}
			zone.pressed = false
		case pointer.Cancel:
			zone.pressed = false
		}
	}
}

// handleReaderKeys maps the minimal reader controls: arrows switch chapters,
// Space and PageUp/PageDown scroll the document and Escape leaves the reader.
// The filters are global (no Focus tag) like on the other screens, so the keys
// work as soon as the reader is visible.
func (r *readerScreen) handleReaderKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(key.Filter{
			Name: key.NameLeftArrow,
		}, key.Filter{
			Name: key.NameRightArrow,
		}, key.Filter{
			Name: key.NameEscape,
		}, key.Filter{
			Name: key.NameSpace,
		}, key.Filter{
			Name: key.NamePageDown,
		}, key.Filter{
			Name: key.NamePageUp,
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
		case key.NameEscape:
			if r.onBack != nil {
				r.onBack()
			}
		case key.NameSpace, key.NamePageDown:
			r.scrollKeys(1)
		case key.NamePageUp:
			r.scrollKeys(-1)
		}
	}
}

// scrollKeys pages the document list by almost a full viewport, using the row
// count captured during the last visible layout.
func (r *readerScreen) scrollKeys(direction int) {
	list := &r.mangaList
	rows := r.mangaVisRows
	if r.title.IsRanobe() {
		list = &r.ranobeList
		rows = r.ranobeVisRows
		if len(r.content.Paragraphs) == 0 {
			return
		}
	} else if len(r.pages) == 0 {
		return
	}
	if rows <= 0 {
		// No visible layout yet: the list has no size information, and
		// ScrollBy would divide by the uninitialised item count.
		return
	}
	list.ScrollBy(float32(direction) * float32(rows) * 0.9)
	r.requestFrame()
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
}

// rebuildMangaRows flattens source pages and their texture chunks into the
// virtualised list. The rebuild is deferred to layout, so image results never
// mutate the slice that layout.List is currently walking.
func (r *readerScreen) rebuildMangaRows() {
	r.mangaRows = r.mangaRows[:0]
	for pageIndex := range r.pages {
		page := &r.pages[pageIndex]
		chunks := len(page.chunks)
		if chunks == 0 && page.width > 0 && page.height > 0 {
			// Reserve the final number of chunk rows from the known page size.
			// Otherwise every loaded image would shift all following rows and
			// the list would jump under the reader.
			chunks = (page.height + readerMaxTextureSide - 1) / readerMaxTextureSide
		}
		if chunks == 0 {
			r.mangaRows = append(r.mangaRows, readerMangaRow{page: pageIndex, chunk: -1})
			continue
		}
		for chunk := 0; chunk < chunks; chunk++ {
			r.mangaRows = append(r.mangaRows, readerMangaRow{page: pageIndex, chunk: chunk})
		}
	}
	r.mangaRowsDirty = false
}

// evictPagesOutside releases decoded pixels for pages that are far from the
// viewport. Page metadata stays, so the list keeps its geometry and a page is
// downloaded again when the reader scrolls back to it.
func (r *readerScreen) evictPagesOutside(firstRow, lastRow int) {
	if firstRow < 0 || lastRow < firstRow || len(r.mangaRows) == 0 {
		return
	}
	firstRow = max(0, firstRow)
	lastRow = min(lastRow, len(r.mangaRows)-1)
	startPage := r.mangaRows[firstRow].page
	endPage := r.mangaRows[lastRow].page
	for index := range r.pages {
		if index >= startPage-1 && index <= endPage+1 {
			continue
		}
		page := &r.pages[index]
		if page.img == nil && len(page.chunks) == 0 {
			continue
		}
		page.img = nil
		page.chunks = nil
		page.loading = false
		r.mangaRowsDirty = true
	}
}

// layoutManga renders the lazy, centred page column. Every visible chunk is a
// separate list row, so Gio uploads only the textures the user can actually see.
func (r *readerScreen) layoutManga(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	if len(r.pages) == 0 {
		r.typo.draw(gtx, "В главе нет страниц", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(13), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(12))))
		return
	}
	if r.mangaRowsDirty || len(r.mangaRows) == 0 {
		r.rebuildMangaRows()
	}
	// The column is width-capped like the concept: the page keeps even margins
	// on a small window and stops growing once it is large enough.
	pageWidth := min(rect.Dx()*73/100, gtx.Dp(readerPageMaxWidth))
	if pageWidth <= 0 {
		return
	}
	column := image.Rect(
		rect.Min.X+(rect.Dx()-pageWidth)/2,
		rect.Min.Y,
		rect.Min.X+(rect.Dx()-pageWidth)/2+pageWidth,
		rect.Max.Y,
	)
	stack := op.Offset(column.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(column.Size())
	visibleFirst, visibleLast := -1, -1
	r.mangaList.Layout(cgtx, len(r.mangaRows), func(gtx layout.Context, index int) layout.Dimensions {
		if visibleFirst < 0 {
			visibleFirst = index
		}
		visibleLast = index
		row := r.mangaRows[index]
		page := &r.pages[row.page]
		// A page is narrowed so the whole strip fits the viewport height,
		// whenever the aspect allows it; the row stays centred in the column.
		width := pageWidth
		if page.width > 0 && page.height > 0 {
			width = fitPageWidth(page.width, page.height, pageWidth, rect.Dy())
		}
		height := gtx.Dp(unit.Dp(20))
		if page.img == nil && len(page.chunks) == 0 {
			r.downloadPage(row.page)
		}
		switch {
		case row.chunk >= 0 && row.chunk < len(page.chunks):
			b := page.chunks[row.chunk].Bounds()
			if b.Dx() > 0 {
				height = width * b.Dy() / b.Dx()
			}
		case row.chunk >= 0 && page.width > 0 && page.height > 0:
			chunkHeight := min(readerMaxTextureSide, page.height-row.chunk*readerMaxTextureSide)
			if chunkHeight > 0 {
				height = width * chunkHeight / page.width
			}
		case row.chunk < 0 && page.width > 0 && page.height > 0:
			height = width * page.height / page.width
		case page.loading:
			height = gtx.Dp(unit.Dp(120))
		}
		if height < gtx.Dp(unit.Dp(40)) {
			height = gtx.Dp(unit.Dp(40))
		}
		left := (pageWidth - width) / 2
		item := image.Rect(left, 0, left+width, height)
		switch {
		case row.chunk >= 0 && row.chunk < len(page.chunks):
			drawImage(gtx, page.chunks[row.chunk], item)
		case page.img != nil:
			drawImage(gtx, page.img, item)
		case page.failed:
			r.typo.drawCentered(gtx, "Не удалось загрузить", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12), mainMutedColor, centerOf(item))
		default:
			phase := float64(time.Now().UnixNano()) / 1e9
			f := phase*2 - float64(int(phase*2))
			drawSpinner(gtx, centerOf(item), gtx.Dp(unit.Dp(9)), gtx.Dp(unit.Dp(2)), f, mainAccentColor1)
		}
		return layout.Dimensions{Size: item.Size()}
	})
	if visibleFirst >= 0 {
		r.mangaVisRows = visibleLast - visibleFirst + 1
	}
	r.evictPagesOutside(visibleFirst, visibleLast)

	// The tiny page counter floats over the bottom of the reader and follows
	// the current page, muted enough not to pull the eye from the art.
	if len(r.pages) > 0 {
		page := 0
		if first := r.mangaList.Position.First; first >= 0 && first < len(r.mangaRows) {
			page = r.mangaRows[first].page
		}
		counter := fmt.Sprintf("%d / %d", min(page+1, len(r.pages)), len(r.pages))
		r.layoutPageCounter(gtx, counter, rect)
	}
}

// layoutPageCounter paints the "page / total" chip at the bottom centre of the
// manga reader: a translucent dark pill with the faint grey label.
func (r *readerScreen) layoutPageCounter(gtx layout.Context, counter string, rect image.Rectangle) {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := r.typo.measure(gtx, counter, face, unit.Sp(11))
	if ink.Dy() <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(10))
	padY := gtx.Dp(unit.Dp(4))
	chip := image.Rect(0, 0, ink.Dx()+2*padX, ink.Dy()+2*padY)
	chip = chip.Add(image.Pt(rect.Min.X+(rect.Dx()-chip.Dx())/2, rect.Max.Y-chip.Dy()-gtx.Dp(unit.Dp(12))))
	fillRounded(gtx, color.NRGBA{R: 0x0d, G: 0x0e, B: 0x14, A: 0xb0}, chip, gtx.Dp(unit.Dp(7)))
	r.typo.drawCentered(gtx, counter, face, unit.Sp(11), readerFaintColor, centerOf(chip))
}

// splitReaderImage cuts a tall webtoon page into GPU-safe textures. Each chunk
// is copied into its own RGBA buffer: subimages would keep the whole decoded
// page alive, and a chapter of 25000px pages would then hold hundreds of
// megabytes after the user has scrolled past it.
func splitReaderImage(src image.Image, maxSide int) []image.Image {
	if src == nil || maxSide <= 0 {
		return nil
	}
	bounds := src.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil
	}
	chunks := make([]image.Image, 0, (bounds.Dy()+maxSide-1)/maxSide)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += maxSide {
		end := min(y+maxSide, bounds.Max.Y)
		chunk := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), end-y))
		stddraw.Draw(chunk, chunk.Bounds(), src, image.Pt(bounds.Min.X, y), stddraw.Src)
		chunks = append(chunks, chunk)
	}
	return chunks
}

// layoutRanobe renders the paragraphs and illustrations on a centred column
// capped at the reading width. Star separators ("***") are centred and faint,
// the intro block before the first separator keeps the faint colour, and the
// body uses the soft white with extra leading between lines — exactly like the
// concept.
func (r *readerScreen) layoutRanobe(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	paragraphs := r.content.Paragraphs
	if len(paragraphs) == 0 && len(r.illustrations) == 0 {
		r.typo.draw(gtx, "В главе нет текста", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(13), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(12))))
		return
	}
	width := min(rect.Dx()*73/100, gtx.Dp(readerTextMaxWidth))
	if width <= 0 {
		return
	}
	column := image.Rect(
		rect.Min.X+(rect.Dx()-width)/2,
		rect.Min.Y,
		rect.Min.X+(rect.Dx()-width)/2+width,
		rect.Max.Y,
	)
	stack := op.Offset(column.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(column.Size())
	rows := buildRanobeRows(len(paragraphs), r.content.Images)
	face := font.Font{Typeface: fontFamily, Weight: font.Normal}
	gap := gtx.Dp(readerParaGap)
	leading := gtx.Dp(readerTextLeading)
	starSize := unit.Sp(13)
	introEnd := introEndIndex(paragraphs)
	visibleFirst, visibleLast := -1, -1
	r.ranobeList.Layout(cgtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		if visibleFirst < 0 {
			visibleFirst = index
		}
		visibleLast = index
		row := rows[index]
		if row.image {
			return r.layoutIllustrationRow(gtx, row.index, width, rect.Dy())
		}
		text := paragraphs[row.index]
		if strings.TrimSpace(text) == "" {
			return layout.Dimensions{Size: image.Pt(width, gtx.Dp(unit.Dp(10)))}
		}
		if starSeparator(text) {
			trimmed := strings.TrimSpace(text)
			rowH := gtx.Dp(unit.Dp(16))
			r.typo.drawCentered(gtx, trimmed, face, starSize, readerFaintColor, image.Pt(width/2, rowH/2))
			return layout.Dimensions{Size: image.Pt(width, rowH+gap/2)}
		}
		col := readerTextColor
		if row.index < introEnd {
			col = readerFaintColor
		}
		_, ink := r.typo.measureWrappedLines(gtx, text, face, readerTextSize, width, leading)
		h := ink.Dy()
		if h < gtx.Dp(unit.Dp(18)) {
			h = gtx.Dp(unit.Dp(18))
		}
		r.typo.drawWrappedLines(gtx, text, face, readerTextSize, col, image.Pt(0, 0), width, leading)
		return layout.Dimensions{Size: image.Pt(width, h+gap)}
	})
	if visibleFirst >= 0 {
		r.ranobeVisRows = visibleLast - visibleFirst + 1
	}
}

// layoutIllustrationRow paints one ranobe picture: it is fetched lazily like a
// manga page, narrowed when the full height does not fit the viewport, and the
// texture chunks are stacked to keep the GPU uploads bounded.
func (r *readerScreen) layoutIllustrationRow(gtx layout.Context, index, columnWidth, viewportHeight int) layout.Dimensions {
	ill := &r.illustrations[index]
	drawWidth := columnWidth
	if ill.width > 0 && ill.height > 0 {
		drawWidth = fitPageWidth(ill.width, ill.height, columnWidth, viewportHeight)
	}
	left := (columnWidth - drawWidth) / 2
	height := gtx.Dp(unit.Dp(120))
	chunksReady := false
	if len(ill.chunks) > 0 {
		chunksReady = true
		height = 0
		for _, chunk := range ill.chunks {
			bounds := chunk.Bounds()
			if bounds.Dx() > 0 {
				height += drawWidth * bounds.Dy() / bounds.Dx()
			}
		}
	} else if ill.width > 0 && ill.height > 0 {
		height = drawWidth * ill.height / ill.width
	} else if ill.loading {
		height = gtx.Dp(unit.Dp(120))
	}
	if len(ill.chunks) == 0 && ill.img == nil && !ill.failed && !ill.loading {
		r.requestIllustration(index)
	}
	if height < gtx.Dp(unit.Dp(40)) {
		height = gtx.Dp(unit.Dp(40))
	}
	switch {
	case chunksReady:
		y := 0
		for _, chunk := range ill.chunks {
			bounds := chunk.Bounds()
			chunkHeight := 0
			if bounds.Dx() > 0 {
				chunkHeight = drawWidth * bounds.Dy() / bounds.Dx()
			}
			drawImage(gtx, chunk, image.Rect(left, y, left+drawWidth, y+chunkHeight))
			y += chunkHeight
		}
	case ill.img != nil:
		drawImage(gtx, ill.img, image.Rect(left, 0, left+drawWidth, height))
	case ill.failed:
		r.typo.drawCentered(gtx, "Не удалось загрузить иллюстрацию", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12), mainMutedColor, image.Pt(columnWidth/2, height/2))
	default:
		phase := float64(time.Now().UnixNano()) / 1e9
		f := phase*2 - float64(int(phase*2))
		drawSpinner(gtx, image.Pt(columnWidth/2, height/2), gtx.Dp(unit.Dp(10)), gtx.Dp(unit.Dp(2)), f, mainAccentColor1)
	}
	return layout.Dimensions{Size: image.Pt(columnWidth, height)}
}
