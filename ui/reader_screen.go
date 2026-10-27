package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
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
	// readerMaxTextureSide keeps one GPU texture below the Direct3D limit.
	// Webtoon pages are often 760x25000; uploading them as one texture makes
	// Gio panic with CreateTexture2D 0x80070057 before anything is visible.
	readerMaxTextureSide = 4096
	// readerPageMaxWidth keeps manga pages in a readable column instead of
	// stretching a 760px scan across a wide window.
	readerPageMaxWidth = unit.Dp(920)
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
	// missing is how many chapters of the title this team does not have.
	missing int
	// hasCurrent is false when the reader is on a chapter this team skipped.
	// The translation menu disables such a row instead of loading a 404.
	hasCurrent bool
}

// readerTreeRow is one row of the volume/chapter tree. A header row has an
// empty chapter key and toggles its volume; a chapter row may be unavailable
// for the active translation and is then drawn disabled.
type readerTreeRow struct {
	volume   string
	chapter  string
	header   bool
	expanded bool
	missing  bool
	// available/total are shown on a volume header.
	available int
	total     int
}

// readerPageState is one manga page in the virtualised list.
type readerPageState struct {
	url     string
	img     image.Image
	chunks  []image.Image
	width   int
	height  int
	chapter int
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

// readerMangaRow is one virtualised list row. Tall scans are split into
// several chunk rows so the GPU only receives textures that are near the
// viewport, instead of uploading a whole 25000px page at once.
type readerMangaRow struct {
	page  int
	chunk int
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
	pageIndex       int
	// chapters are the keys available in the selected translation.
	chapters []string
	// translations is the ordered translation-team list with chapter counts.
	translations []readerTranslation
	// treeRows is the flattened volume/chapter tree shown in the sidebar.
	treeRows      []readerTreeRow
	treeClick     []widget.Clickable
	expanded      map[string]bool
	pendingToggle string

	// search filters the chapter tree by display label or raw key.
	lastQuery string
	search    widget.Editor
	clear     widget.Clickable

	translationClick  widget.Clickable
	translationAnchor image.Rectangle
	openTranslations  bool
	translationScrim  widget.Clickable

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
	mangaRows      []readerMangaRow
	mangaRowsDirty bool
	mangaList      layout.List
	ranobeList     layout.List
	// autoStitch turns the current chapter into an endless vertical strip:
	// approaching the bottom appends the next chapter instead of stopping.
	autoStitch    bool
	stitchClick   widget.Clickable
	streamNext    int
	streamPending bool
	streamLoaded  map[int]bool

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
		expanded:       make(map[string]bool),
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
	r.expandCurrentVolume()
	r.loadChapter()
}

func (r *readerScreen) rebuildTranslations() {
	current := r.currentChapterKey()
	r.translations = r.translations[:0]
	for _, voice := range r.title.Voiceovers {
		hasCurrent := false
		for _, key := range voice.Episodes {
			if key == current {
				hasCurrent = true
				break
			}
		}
		r.translations = append(r.translations, readerTranslation{
			name:       voice.Name,
			count:      len(voice.Episodes),
			missing:    len(voice.Missing),
			hasCurrent: hasCurrent,
		})
	}
	if r.selectedVoice >= len(r.translations) {
		r.selectedVoice = 0
	}
}

func (r *readerScreen) refreshTranslationFlags() {
	current := r.currentChapterKey()
	for index := range r.translations {
		if index >= len(r.title.Voiceovers) {
			continue
		}
		r.translations[index].hasCurrent = false
		for _, key := range r.title.Voiceovers[index].Episodes {
			if key == current {
				r.translations[index].hasCurrent = true
				break
			}
		}
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

// chapterAvailable reports whether the selected translation has the key.
func (r *readerScreen) chapterAvailable(key string) bool {
	for _, candidate := range r.chapters {
		if candidate == key {
			return true
		}
	}
	return false
}

// rebuildTree groups the complete title into volumes and marks chapters the
// active translation skipped. The global episode list is used on purpose: a
// reader must see that team A has a gap where team B has a chapter.
func (r *readerScreen) rebuildTree() {
	all := r.title.Episodes
	if len(all) == 0 {
		all = r.chapters
	}
	if r.expanded == nil {
		r.expanded = make(map[string]bool)
	}
	available := make(map[string]bool, len(r.chapters))
	for _, key := range r.chapters {
		available[key] = true
	}
	query := strings.ToLower(strings.TrimSpace(r.search.Text()))
	matches := func(key string) bool {
		return query == "" ||
			strings.Contains(strings.ToLower(key), query) ||
			strings.Contains(strings.ToLower(readingChapterLabel(key)), query)
	}

	volumeOrder := make([]string, 0, 16)
	byVolume := make(map[string][]string, 16)
	for _, key := range all {
		volume, _ := catalog.SplitReadingChapterKey(key)
		if _, ok := byVolume[volume]; !ok {
			volumeOrder = append(volumeOrder, volume)
		}
		byVolume[volume] = append(byVolume[volume], key)
	}

	r.treeRows = r.treeRows[:0]
	for _, volume := range volumeOrder {
		keys := byVolume[volume]
		shown := make([]string, 0, len(keys))
		availableCount := 0
		for _, key := range keys {
			if available[key] {
				availableCount++
			}
			if matches(key) {
				shown = append(shown, key)
			}
		}
		if len(shown) == 0 {
			continue
		}
		expanded := r.expanded[volume] || query != ""
		r.treeRows = append(r.treeRows, readerTreeRow{
			volume:    volume,
			header:    true,
			expanded:  expanded,
			available: availableCount,
			total:     len(keys),
		})
		if !expanded {
			continue
		}
		for _, key := range shown {
			r.treeRows = append(r.treeRows, readerTreeRow{
				volume:  volume,
				chapter: key,
				missing: !available[key],
			})
		}
	}
	r.treeClick = make([]widget.Clickable, len(r.treeRows))
	r.chapterList.Position = layout.Position{}
}

func (r *readerScreen) refreshFiltered() {
	r.rebuildTree()
}

func (r *readerScreen) expandCurrentVolume() {
	volume, _ := catalog.SplitReadingChapterKey(r.currentChapterKey())
	if volume == "" {
		return
	}
	if r.expanded == nil {
		r.expanded = make(map[string]bool)
	}
	if !r.expanded[volume] {
		r.expanded[volume] = true
		r.rebuildTree()
	}
}

func readerVolumeLabel(volume string) string {
	if volume == "" || volume == "0" {
		return "Без тома"
	}
	return "Том " + volume
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
	r.pageIndex = 0
	r.loadErr = ""
	r.cancelImages()
	for len(r.imageResults) > 0 {
		<-r.imageResults
	}

	voice := r.translationName()
	key := r.currentChapterKey()
	title := r.title
	r.refreshTranslationFlags()

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

// scrollPage moves the manga viewport by whole pages. Chapters still change
// with the footer buttons; arrows are for reading inside the current chapter.
func (r *readerScreen) scrollPage(delta int) {
	if len(r.pages) == 0 {
		return
	}
	next := r.pageIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(r.pages) {
		next = len(r.pages) - 1
	}
	r.pageIndex = next
	if len(r.mangaRows) == 0 {
		r.rebuildMangaRows()
	}
	r.mangaList.ScrollTo(r.rowForPage(next))
	r.requestFrame()
}

// selectChapter switches to a chapter in the current translation.
func (r *readerScreen) selectChapter(index int) {
	if index < 0 || index >= len(r.chapters) || index == r.selectedChapter {
		return
	}
	r.selectedChapter = index
	r.expandCurrentVolume()
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
		r.pages[index] = readerPageState{
			url:     page.URL,
			width:   page.Width,
			height:  page.Height,
			chapter: r.selectedChapter,
		}
	}
	r.streamNext = r.selectedChapter + 1
	r.streamPending = false
	r.streamLoaded = map[int]bool{r.selectedChapter: true}
	r.mangaRowsDirty = true
	r.mangaList.Position = layout.Position{}
	r.ranobeList.Position = layout.Position{}
}

// streamChapterKey returns the next chapter that should be stitched below the
// current strip, or "" at the end of the translation.
func (r *readerScreen) streamChapterKey() string {
	if r.streamNext < 0 || r.streamNext >= len(r.chapters) {
		return ""
	}
	return r.chapters[r.streamNext]
}

// appendStreamChapter appends a loaded chapter below the current pages. The
// sidebar still selects the chapter at the top of the strip; as the viewport
// reaches a new chapter, syncStreamPosition moves the saved position forward.
func (r *readerScreen) appendStreamChapter(index int, content catalog.ReadingContent) bool {
	if index < 0 || index >= len(r.chapters) {
		return false
	}
	if r.streamLoaded == nil {
		r.streamLoaded = make(map[int]bool)
	}
	if r.streamLoaded[index] {
		return false
	}
	if len(content.Pages) == 0 {
		// An empty chapter must not stall the strip forever: mark it seen and
		// let the next layout request the following chapter.
		r.streamLoaded[index] = true
		r.streamPending = false
		if index >= r.streamNext {
			r.streamNext = index + 1
		}
		return false
	}
	for _, page := range content.Pages {
		r.pages = append(r.pages, readerPageState{
			url:     page.URL,
			width:   page.Width,
			height:  page.Height,
			chapter: index,
		})
	}
	r.streamLoaded[index] = true
	r.streamPending = false
	if index >= r.streamNext {
		r.streamNext = index + 1
	}
	r.mangaRowsDirty = true
	r.requestFrame()
	return true
}

// extendStream requests or appends the next chapter. It is called only when
// the viewport is close to the end and never blocks the layout goroutine.
func (r *readerScreen) extendStream() {
	if !r.autoStitch || r.title.IsRanobe() || r.streamPending {
		return
	}
	index := r.streamNext
	key := r.streamChapterKey()
	if key == "" {
		return
	}
	if content, ok := r.chapterCache[readerChapterCacheKey(r.translationName(), key)]; ok {
		r.appendStreamChapter(index, content)
		return
	}
	r.streamPending = true
	r.requestChapter(r.translationName(), key, r.title)
}

// syncStreamPosition updates the saved chapter when the viewport crosses into
// a chapter appended by auto-stitch. It deliberately does not call loadChapter:
// the already appended pages stay in place.
func (r *readerScreen) syncStreamPosition(page int) {
	if !r.autoStitch || page < 0 || page >= len(r.pages) {
		return
	}
	chapter := r.pages[page].chapter
	if chapter < 0 || chapter >= len(r.chapters) || chapter == r.selectedChapter {
		return
	}
	r.selectedChapter = chapter
	r.refreshTranslationFlags()
	r.expandCurrentVolume()
	r.trackSelection(r.title, r.translationName(), r.chapters[chapter])
}

// toggleAutoStitch starts or stops the endless strip without throwing away
// pages that have already been appended.
func (r *readerScreen) toggleAutoStitch() {
	if r.title.IsRanobe() {
		return
	}
	r.autoStitch = !r.autoStitch
	if r.autoStitch {
		if r.streamLoaded == nil {
			r.streamLoaded = make(map[int]bool)
		}
		r.streamLoaded[r.selectedChapter] = true
		if r.streamNext <= r.selectedChapter {
			r.streamNext = r.selectedChapter + 1
		}
		r.extendStream()
	} else {
		r.streamPending = false
	}
	r.requestFrame()
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
	r.expandCurrentVolume()
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
				if result.key == r.streamChapterKey() {
					r.streamPending = false
					r.autoStitch = false
					logger.Warn("reader auto-stitch stopped", "slug", r.title.Slug, "translation", result.voice, "chapter", result.key, "error", result.err)
				}
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
				if r.autoStitch {
					r.extendStream()
				}
				if r.animationsRun() {
					gtx.Execute(op.InvalidateCmd{})
				}
				continue
			}
			if r.autoStitch && result.key == r.streamChapterKey() {
				r.streamPending = false
				r.appendStreamChapter(r.streamNext, result.content)
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

// handleReaderKeys maps Left/Right to manga page navigation. Ranobe chapters
// are still switched by the footer; Up/Down scroll the content list through the
// normal focus and PageUp/PageDown are left to the list.
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
			if r.title.IsRanobe() {
				r.selectChapter(r.selectedChapter - 1)
			} else {
				r.scrollPage(-1)
			}
		case key.NameRightArrow:
			if r.title.IsRanobe() {
				r.selectChapter(r.selectedChapter + 1)
			} else {
				r.scrollPage(1)
			}
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
	r.translationAnchor = translationRect.Add(rect.Min)
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
	itemHeight := gtx.Dp(unit.Dp(50))
	itemGap := gtx.Dp(unit.Dp(2))
	menuPad := gtx.Dp(unit.Dp(4))
	height := len(r.translations)*itemHeight + len(r.translations)*itemGap + 2*menuPad
	anchor := r.translationAnchor
	if anchor.Empty() {
		anchor = image.Rect(
			sidebar.Min.X+pad,
			sidebar.Min.Y+pad,
			sidebar.Max.X-pad,
			sidebar.Min.Y+pad+gtx.Dp(readerControlHeight),
		)
	}
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
	draw := func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(6))
		if index == r.selectedVoice {
			fillRounded(gtx, readerChapterActive, local, radius)
		} else if translation.hasCurrent && click.Hovered() {
			fillRounded(gtx, readerChapterHover, local, radius)
		}
		name := translation.name
		if name == "" {
			name = "Без команды"
		}
		label := fmt.Sprintf("%s · %d гл.", name, translation.count)
		padX := gtx.Dp(unit.Dp(10))
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		textColor := mainTextColor
		if !translation.hasCurrent {
			textColor = readerDisabledText
		} else if index == r.selectedVoice {
			textColor = titleColor
		}
		r.typo.draw(gtx, label, face, unit.Sp(12.5), textColor, image.Pt(padX, gtx.Dp(unit.Dp(7))))
		note := fmt.Sprintf("нет %d гл.", translation.missing)
		noteColor := readerMutedColor
		if !translation.hasCurrent {
			note = "нет этой главы"
			noteColor = mainDangerColor
		}
		if translation.missing == 0 {
			note = "все главы"
			noteColor = readerMutedColor
		}
		r.typo.draw(gtx, note, font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(10.5), noteColor, image.Pt(padX, gtx.Dp(unit.Dp(27))))
		if index == r.selectedVoice {
			check := image.Pt(local.Dx()-padX-gtx.Dp(unit.Dp(7)), local.Dy()/2)
			drawCheckIcon(gtx, check, gtx.Dp(unit.Dp(13)), mainAccentColor1)
		}
	}
	if !translation.hasCurrent {
		draw(gtx, rect)
		return false
	}
	return layoutClickableRect(gtx, rect, click, draw)
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
		border := mainInputBorder
		fill := mainInputColor
		if active {
			border = mainInputFocusBorder
			fill = mixColor(mainInputColor, mainAccentColor1, 0.12)
		}
		fillRounded(gtx, border, local, radius)
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
			drawChevronUp(gtx, center, size, gtx.Dp(unit.Dp(1.6)), mainMutedColor)
		} else {
			drawChevronDown(gtx, center, size, gtx.Dp(unit.Dp(1.6)), mainMutedColor)
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
	if len(r.treeRows) == 0 && len(r.chapters) > 0 {
		r.rebuildTree()
	}
	if len(r.treeRows) == 0 {
		r.typo.draw(gtx, "Ничего не найдено", font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12), mainMutedColor, image.Pt(0, gtx.Dp(unit.Dp(8))))
		return
	}
	if len(r.chapterClick) != len(r.chapters) {
		r.chapterClick = make([]widget.Clickable, len(r.chapters))
	}
	if len(r.treeClick) != len(r.treeRows) {
		r.treeClick = make([]widget.Clickable, len(r.treeRows))
	}
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	r.chapterList.Layout(cgtx, len(r.treeRows), func(gtx layout.Context, index int) layout.Dimensions {
		item := image.Rect(0, 0, rect.Dx(), gtx.Dp(readerChapterHeight))
		if index >= len(r.treeRows) {
			return layout.Dimensions{Size: item.Size()}
		}
		row := r.treeRows[index]
		var clicked bool
		if row.header {
			clicked = r.layoutVolumeRow(gtx, item, row, index)
			if clicked {
				r.pendingToggle = row.volume
			}
		} else {
			chapterIndex := chapterIndexForKey(r.chapters, row.chapter)
			clicked = r.layoutChapterRow(gtx, item, row, chapterIndex)
			if clicked {
				r.selectChapter(chapterIndex)
			}
		}
		return layout.Dimensions{Size: item.Size()}
	})
	if volume := r.pendingToggle; volume != "" {
		r.pendingToggle = ""
		r.expanded[volume] = !r.expanded[volume]
		r.rebuildTree()
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (r *readerScreen) layoutVolumeRow(gtx layout.Context, rect image.Rectangle, row readerTreeRow, index int) bool {
	if rect.Empty() || index < 0 || index >= len(r.treeClick) {
		return false
	}
	click := &r.treeClick[index]
	return layoutClickableRect(gtx, rect, click, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(7))
		if click.Hovered() {
			fillRounded(gtx, readerChapterHover, local.Inset(gtx.Dp(unit.Dp(2))), radius)
		}
		padX := gtx.Dp(unit.Dp(8))
		label := readerVolumeLabel(row.volume)
		face := font.Font{Typeface: fontFamily, Weight: font.Bold}
		_, ink := r.typo.measure(gtx, label, face, unit.Sp(12.5))
		r.typo.draw(gtx, label, face, unit.Sp(12.5), readerTitleColor, image.Pt(padX+gtx.Dp(unit.Dp(14)), (local.Dy()-ink.Dy())/2))
		count := fmt.Sprintf("%d/%d", row.available, row.total)
		countFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
		_, countInk := r.typo.measure(gtx, count, countFace, unit.Sp(10.5))
		r.typo.draw(gtx, count, countFace, unit.Sp(10.5), readerMutedColor, image.Pt(local.Dx()-padX-countInk.Dx(), (local.Dy()-countInk.Dy())/2))
		chevron := image.Pt(padX+gtx.Dp(unit.Dp(3)), local.Dy()/2)
		if row.expanded {
			drawChevronUp(gtx, chevron, gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(1.5)), readerMutedColor)
		} else {
			drawChevronDown(gtx, chevron, gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(1.5)), readerMutedColor)
		}
	})
}

func (r *readerScreen) layoutChapterRow(gtx layout.Context, rect image.Rectangle, row readerTreeRow, index int) bool {
	if rect.Empty() {
		return false
	}
	selected := index >= 0 && index == r.selectedChapter
	var click *widget.Clickable
	if index >= 0 && index < len(r.chapterClick) {
		click = &r.chapterClick[index]
	}
	draw := func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(6))
		background := color.NRGBA{A: 0}
		if selected {
			background = readerChapterActive
		} else if click != nil && click.Hovered() {
			background = readerChapterHover
		}
		if background.A != 0 {
			fillRounded(gtx, background, local, radius)
		}
		weight := font.Medium
		if row.missing {
			weight = font.Normal
		}
		face := font.Font{Typeface: fontFamily, Weight: weight}
		textColor := mainTextColor
		switch {
		case row.missing:
			textColor = readerDisabledText
		case selected:
			textColor = titleColor
		}
		label := readingChapterLabel(row.chapter)
		padX := gtx.Dp(unit.Dp(12))
		_, ink := r.typo.measure(gtx, label, face, unit.Sp(12.5))
		r.typo.draw(gtx, label, face, unit.Sp(12.5), textColor, image.Pt(padX, (local.Dy()-ink.Dy())/2))
		if row.missing {
			note := "нет"
			noteFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
			_, noteInk := r.typo.measure(gtx, note, noteFace, unit.Sp(10.5))
			r.typo.draw(gtx, note, noteFace, unit.Sp(10.5), readerDisabledText, image.Pt(local.Dx()-padX-noteInk.Dx(), (local.Dy()-noteInk.Dy())/2))
		}
	}
	if row.missing || click == nil {
		draw(gtx, image.Rect(0, 0, rect.Dx(), rect.Dy()))
		return false
	}
	return layoutClickableRect(gtx, rect, click, draw)
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
	controlTop := (headerH - gtx.Dp(unit.Dp(28))) / 2
	right := local.Dx() - padX
	if !r.title.IsRanobe() && len(r.pages) > 0 {
		counter := fmt.Sprintf("%d / %d", min(r.pageIndex+1, len(r.pages)), len(r.pages))
		counterFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
		_, counterInk := r.typo.measure(gtx, counter, counterFace, unit.Sp(11))
		counterX := right - gtx.Dp(unit.Dp(96)) - gtx.Dp(unit.Dp(12)) - counterInk.Dx()
		r.typo.draw(gtx, counter, counterFace, unit.Sp(11), mainMutedColor, image.Pt(counterX, gtx.Dp(unit.Dp(13))))
	}
	if !r.title.IsRanobe() {
		toggleRect := image.Rect(right-gtx.Dp(unit.Dp(96)), controlTop, right, controlTop+gtx.Dp(unit.Dp(28)))
		if r.layoutStitchToggle(gtx, toggleRect) {
			r.toggleAutoStitch()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
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

// layoutStitchToggle is the reader's infinite-strip switch. It changes only
// reading behaviour, so it sits next to the chapter title rather than in the
// title settings page.
func (r *readerScreen) layoutStitchToggle(gtx layout.Context, rect image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	if len(r.pages) == 0 {
		return false
	}
	clicked := layoutClickableRect(gtx, rect, &r.stitchClick, func(gtx layout.Context, local image.Rectangle) {
		radius := gtx.Dp(unit.Dp(14))
		background := mainInputColor
		border := mainInputBorder
		text := mainMutedColor
		if r.autoStitch {
			background = mixColor(mainInputColor, mainAccentColor1, 0.22)
			border = mainAccentColor1
			text = titleColor
		} else if r.stitchClick.Hovered() {
			border = mainInputFocusBorder
			text = mainTextColor
		}
		fillRounded(gtx, border, local, radius)
		fillRounded(gtx, background, local.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
		label := "Лента"
		face := font.Font{Typeface: fontFamily, Weight: font.Medium}
		_, ink := r.typo.measure(gtx, label, face, unit.Sp(11.5))
		r.typo.draw(gtx, label, face, unit.Sp(11.5), text, image.Pt(gtx.Dp(unit.Dp(12)), (local.Dy()-ink.Dy())/2))
		dot := image.Pt(local.Dx()-gtx.Dp(unit.Dp(14)), local.Dy()/2)
		dotColor := readerMutedColor
		if r.autoStitch {
			dotColor = mainAccentColor1
		}
		radiusDot := gtx.Dp(unit.Dp(3))
		fillRounded(gtx, dotColor, image.Rect(dot.X-radiusDot, dot.Y-radiusDot, dot.X+radiusDot, dot.Y+radiusDot), radiusDot)
	})
	return clicked
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

// rowForPage maps a source page to the first row that displays it.
func (r *readerScreen) rowForPage(page int) int {
	for index, row := range r.mangaRows {
		if row.page == page {
			return index
		}
	}
	return 0
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
	pageWidth := min(rect.Dx(), gtx.Dp(readerPageMaxWidth))
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
		width := pageWidth
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
		item := image.Rect(0, 0, width, height)
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
	first := r.mangaList.Position.First
	if first >= 0 && first < len(r.mangaRows) {
		page := r.mangaRows[first].page
		r.pageIndex = page
		r.syncStreamPosition(page)
		if r.autoStitch && first+8 >= len(r.mangaRows) {
			r.extendStream()
		}
	}
	r.evictPagesOutside(visibleFirst, visibleLast)
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
