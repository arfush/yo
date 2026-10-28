package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
)

// readerMutedColor and friends colour the reading title tree. They moved here
// together with readerTreeRow when the reader itself became sidebar-free; the
// values are section-themed by applySectionTheme like the rest of the chrome.
var (
	readerMutedColor    = color.NRGBA{R: 0x8d, G: 0x87, B: 0x9b, A: 0xff}
	readerChapterHover  = color.NRGBA{R: 0x22, G: 0x1d, B: 0x33, A: 0xff}
	readerChapterActive = color.NRGBA{R: 0x2b, G: 0x22, B: 0x45, A: 0xff}
	readerDisabledText  = color.NRGBA{R: 0x55, G: 0x4f, B: 0x66, A: 0xff}
)

// readerTreeRow is one row of the volume/chapter tree on the reading title
// screen. A header row has an empty chapter key and toggles its volume; a
// chapter row may be unavailable for the active translation and is then drawn
// disabled.
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

// readerVolumeLabel names a volume header on the reading title screen.
func readerVolumeLabel(volume string) string {
	if volume == "" || volume == "0" {
		return "Без тома"
	}
	return "Том " + volume
}

// readingTitleLoadResult carries a refreshed manga/ranobe title back to the UI
// goroutine. Chapter lists are never stored in SQLite, so a saved history row
// has to fetch them again before it can show what to read.
type readingTitleLoadResult struct {
	generation int
	title      catalog.Title
	err        error
}

// readingTitleScreen is the landing page for manga and ranobe titles. Once its
// chapter list arrives, it delegates all drawing to the shared titleScreen so
// the layout is pixel-identical to anime; only the section labels and the
// forward action change.
type readingTitleScreen struct {
	typo *typography

	item   historyTitle
	title  catalog.Title
	loaded bool
	// inner is the shared anime title screen. Reading titles use exactly the
	// same layout and controls, with only the section labels renamed.
	inner *titleScreen

	loading        bool
	loadErr        string
	loadGeneration int
	loadResults    chan readingTitleLoadResult
	loadCtx        context.Context
	loadCancel     context.CancelFunc
	onUserStatus   func(historyTitle, catalog.UserStatus) error

	selectedVoice   int
	selectedChapter int
	chapters        []string

	treeRows      []readerTreeRow
	treeClick     []widget.Clickable
	chapterClick  []widget.Clickable
	expanded      map[string]bool
	pendingToggle string

	voiceClick   []widget.Clickable
	voiceList    layout.List
	chapterList  layout.List
	focusCleared bool

	back       footerNamedButton
	saveButton footerNamedButton
	readButton footerNamedButton
	savedUntil time.Time
	saveError  string

	invalidate func()
	hidden     func() bool

	load     func(context.Context, string) (catalog.Title, error)
	saveFunc func(catalog.Title, string, string) (catalog.Title, error)
	onRead   func(historyTitle, catalog.Title, string, string)
	onBack   func()
	onError  func(error)
	onSaved  func(catalog.Title)
}

func newReadingTitleScreen(typo *typography, item historyTitle) *readingTitleScreen {
	screen := &readingTitleScreen{
		typo:            typo,
		item:            item,
		loadResults:     make(chan readingTitleLoadResult, 1),
		expanded:        make(map[string]bool),
		selectedVoice:   item.selectedVoice,
		selectedChapter: item.selectedEpisode,
		back:            newBackFooterButton(),
		saveButton: footerNamedButton{
			label: "Сохранить",
			style: footerSecondaryStyle,
			size:  titlePageButtonSize,
		},
		readButton: newWatchFooterButton(),
	}
	screen.readButton.label = "Читать"
	screen.readButton.reserve = "Читать"
	screen.voiceList.Axis = layout.Vertical
	screen.chapterList.Axis = layout.Vertical
	if len(item.chapters) > 0 && len(item.voices) > 0 {
		screen.applyLoaded(libraryTitleFromHistory(item))
	} else {
		screen.rebuildFromItem()
	}
	return screen
}

func (w *readingTitleScreen) startLoad() {
	if w.load == nil || w.loaded || w.loading {
		return
	}
	input := w.item.loadInput()
	if strings.TrimSpace(input) == "" {
		w.loadErr = "Не удалось определить ссылку тайтла. Вернитесь и откройте его заново."
		return
	}
	w.loading = true
	w.loadErr = ""
	w.loadGeneration++
	generation := w.loadGeneration
	ctx, cancel := context.WithCancel(context.Background())
	w.loadCtx = ctx
	w.loadCancel = cancel
	go func() {
		title, err := w.load(ctx, input)
		if err != nil {
			err = errLoadTitle.wrap(err)
		}
		w.loadResults <- readingTitleLoadResult{generation: generation, title: title, err: err}
		if w.invalidate != nil {
			w.invalidate()
		}
	}()
}

func (w *readingTitleScreen) updateLoad(gtx layout.Context) {
	for {
		select {
		case result := <-w.loadResults:
			if result.generation != w.loadGeneration {
				continue
			}
			w.loading = false
			if result.err != nil {
				w.loadErr = userErrorText(result.err)
				if w.onError != nil {
					w.onError(result.err)
				}
				continue
			}
			w.applyLoaded(result.title)
			if w.invalidate != nil {
				w.invalidate()
			}
		default:
			return
		}
	}
}

func (w *readingTitleScreen) applyLoaded(title catalog.Title) {
	w.title = title
	w.item = titleFromLibrary(title)
	w.loaded = true
	w.loading = false
	w.loadErr = ""
	w.selectedVoice = w.item.selectedVoice
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		w.selectedVoice = 0
	}
	w.selectedChapter = w.item.selectedEpisode
	w.rebuildChapters()
	w.buildInner()
}

// buildInner hands rendering to the shared title screen. Only the reading
// callbacks differ: Save persists chapters and Watch becomes Read.
func (w *readingTitleScreen) buildInner() {
	if w.typo == nil || len(w.item.episodes) == 0 {
		w.inner = nil
		return
	}
	inner := newTitleScreen(w.typo, w.item, 0)
	inner.hidden = w.hidden
	inner.onBack = func() {
		if w.onBack != nil {
			w.onBack()
		}
	}
	inner.onError = func(err error) {
		if w.onError != nil {
			w.onError(err)
		}
	}
	inner.onUserStatus = func(item historyTitle, status catalog.UserStatus) error {
		if w.onUserStatus == nil {
			return nil
		}
		return w.onUserStatus(item, status)
	}
	inner.onSave = func(voice, chapter string) error {
		if w.saveFunc == nil {
			return nil
		}
		saved, err := w.saveFunc(libraryTitleFromHistory(w.item), voice, chapter)
		if err != nil {
			return err
		}
		w.item.id = saved.ID
		w.item.slug = saved.Slug
		w.item.coverPath = saved.CoverPath
		w.item.coverNeedsRetry = saved.CoverNeedsRetry
		inner.applySaved(w.item)
		if w.onSaved != nil {
			w.onSaved(saved)
		}
		return nil
	}
	inner.onWatch = func(voice, chapter string, _ int) error {
		if w.onRead != nil {
			w.onRead(w.item, w.title, voice, chapter)
		}
		return nil
	}
	w.inner = inner
}

func (w *readingTitleScreen) rebuildFromItem() {
	w.title = libraryTitleFromHistory(w.item)
	w.selectedVoice = w.item.selectedVoice
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		w.selectedVoice = 0
	}
	w.rebuildChapters()
}

func (w *readingTitleScreen) rebuildChapters() {
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		w.chapters = nil
	} else {
		w.chapters = append([]string(nil), w.item.voices[w.selectedVoice].episodes...)
	}
	if w.selectedChapter < 0 || w.selectedChapter >= len(w.chapters) {
		w.selectedChapter = firstReadingChapter(w.chapters)
	}
	w.chapterClick = make([]widget.Clickable, len(w.chapters))
	w.rebuildTree()
}

func firstReadingChapter(chapters []string) int {
	if len(chapters) == 0 {
		return -1
	}
	return 0
}

func (w *readingTitleScreen) chapterAvailable(key string) bool {
	for _, chapter := range w.chapters {
		if chapter == key {
			return true
		}
	}
	return false
}

// rebuildTree groups the complete chapter list by volume. Chapters the active
// translation skipped stay visible but disabled, so the title page explains
// gaps before the reader is opened.
func (w *readingTitleScreen) rebuildTree() {
	all := w.item.episodes
	if len(all) == 0 {
		for _, chapter := range w.item.chapters {
			all = append(all, chapter.Key)
		}
	}
	if w.expanded == nil {
		w.expanded = make(map[string]bool)
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
	if current := w.currentChapter(); current != "" {
		volume, _ := catalog.SplitReadingChapterKey(current)
		if volume != "" {
			w.expanded[volume] = true
		}
	}
	w.treeRows = w.treeRows[:0]
	for _, volume := range volumeOrder {
		keys := byVolume[volume]
		available := 0
		for _, key := range keys {
			if w.chapterAvailable(key) {
				available++
			}
		}
		w.treeRows = append(w.treeRows, readerTreeRow{
			volume:    volume,
			header:    true,
			expanded:  w.expanded[volume],
			available: available,
			total:     len(keys),
		})
		if !w.expanded[volume] {
			continue
		}
		for _, key := range keys {
			w.treeRows = append(w.treeRows, readerTreeRow{
				volume:  volume,
				chapter: key,
				missing: !w.chapterAvailable(key),
			})
		}
	}
	w.treeClick = make([]widget.Clickable, len(w.treeRows))
	w.chapterList.Position = layout.Position{}
}

func (w *readingTitleScreen) currentChapter() string {
	if w.selectedChapter >= 0 && w.selectedChapter < len(w.chapters) {
		return w.chapters[w.selectedChapter]
	}
	return ""
}

func (w *readingTitleScreen) selectVoice(index int) {
	if index < 0 || index >= len(w.item.voices) || index == w.selectedVoice {
		return
	}
	saved := w.currentChapter()
	w.selectedVoice = index
	w.rebuildChapters()
	w.selectedChapter = chapterIndexForKey(w.chapters, saved)
	if w.selectedChapter < 0 {
		w.selectedChapter = firstReadingChapter(w.chapters)
	}
	volume, _ := catalog.SplitReadingChapterKey(w.currentChapter())
	if volume != "" {
		w.expanded[volume] = true
	}
	w.rebuildTree()
}

func (w *readingTitleScreen) selectChapter(index int) {
	if index < 0 || index >= len(w.chapters) || index == w.selectedChapter {
		return
	}
	w.selectedChapter = index
}

func (w *readingTitleScreen) read() {
	if w.selectedChapter < 0 || w.selectedChapter >= len(w.chapters) {
		return
	}
	if w.onRead != nil {
		w.onRead(w.item, w.title, w.translationName(), w.chapters[w.selectedChapter])
	}
}

func (w *readingTitleScreen) translationName() string {
	if w.selectedVoice >= 0 && w.selectedVoice < len(w.item.voices) {
		return w.item.voices[w.selectedVoice].name
	}
	return ""
}

func (w *readingTitleScreen) save(gtx layout.Context) {
	if w.saveFunc == nil || !w.loaded {
		return
	}
	if w.selectedChapter < 0 || w.selectedChapter >= len(w.chapters) {
		return
	}
	title, err := w.saveFunc(w.title, w.translationName(), w.chapters[w.selectedChapter])
	if err != nil {
		w.saveError = userErrorText(err)
		if w.onError != nil {
			w.onError(err)
		}
		return
	}
	w.item = titleFromLibrary(title)
	w.title = title
	w.savedUntil = gtx.Now.Add(2 * time.Second)
	w.saveError = ""
	if w.onSaved != nil {
		w.onSaved(title)
	}
	if w.invalidate != nil {
		w.invalidate()
	}
}

func (w *readingTitleScreen) layout(gtx layout.Context) {
	w.updateLoad(gtx)
	if !w.loaded && !w.loading && w.loadErr == "" {
		w.startLoad()
	}
	if w.inner != nil {
		w.inner.layout(gtx)
		return
	}
	if !w.focusCleared {
		w.focusCleared = true
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(titlePagePadX)
	padTop := gtx.Dp(titlePagePadTop)
	available := size.X - 2*padX
	if available <= 0 {
		return
	}
	posterHeight := gtx.Dp(titlePagePosterHeight)
	posterWidth := gtx.Dp(titlePagePosterWidth)
	if size.Y < gtx.Dp(390) {
		posterHeight = gtx.Dp(92)
		posterWidth = posterHeight * 86 / 120
	}
	poster := image.Rect(padX, padTop, padX+posterWidth, padTop+posterHeight)
	drawPosterArt(gtx, w.typo, poster, w.item, unit.Sp(32))
	headerX := poster.Max.X + gtx.Dp(titlePagePosterGap)
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	titleTop := poster.Min.Y + gtx.Dp(19)
	drawTruncatedText(gtx, w.typo, w.item.title, titleFace, titlePageTitleSize, mainTextColor, image.Pt(headerX, titleTop), size.X-padX-headerX)
	_, titleInk := w.typo.measure(gtx, w.item.title, titleFace, titlePageTitleSize)
	statusTop := titleTop + max(titleInk.Dy(), gtx.Dp(mainStatusHeight)) + gtx.Dp(10)
	statusWidth := statusPillWidth(gtx, w.typo)
	statusRect := image.Rect(headerX, statusTop, headerX+statusWidth, statusTop+gtx.Dp(mainStatusHeight))
	drawStatusPill(gtx, w.typo, statusRect, w.item.status)
	kind := "МАНГА"
	if w.item.ranobe() {
		kind = "РАНОБЭ"
	}
	w.typo.draw(gtx, kind, font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(10.5), mainMutedColor, image.Pt(statusRect.Max.X+gtx.Dp(unit.Dp(10)), statusTop+gtx.Dp(unit.Dp(5))))
	if w.item.original != "" {
		originalFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
		drawTruncatedText(gtx, w.typo, w.item.original, originalFace, unit.Sp(12), mainMutedColor, image.Pt(headerX, statusRect.Max.Y+gtx.Dp(12)), size.X-padX-headerX)
	}

	columnsTop := poster.Max.Y + gtx.Dp(titlePageColumnsGapTop)
	bottom := size.Y - gtx.Dp(titlePagePanelBottomGap)
	if bottom <= columnsTop+gtx.Dp(80) {
		bottom = size.Y
	}
	columnGap := gtx.Dp(titlePageColumnGap)
	leftWidth := int(float32(available) * 0.36)
	if maxLeft := gtx.Dp(unit.Dp(310)); leftWidth > maxLeft {
		leftWidth = maxLeft
	}
	if minRight := gtx.Dp(unit.Dp(300)); available-leftWidth-columnGap < minRight {
		leftWidth = available - columnGap - minRight
	}
	if leftWidth < gtx.Dp(unit.Dp(210)) {
		leftWidth = gtx.Dp(unit.Dp(210))
	}
	left := image.Rect(padX, columnsTop, padX+leftWidth, bottom)
	right := image.Rect(left.Max.X+columnGap, columnsTop, size.X-padX, bottom)
	if left.Empty() || right.Empty() {
		return
	}
	w.layoutTranslations(gtx, left)
	w.layoutChapters(gtx, right)
	if w.loading {
		w.layoutLoading(gtx, right)
	} else if w.loadErr != "" {
		w.layoutError(gtx, right)
	}
}

func (w *readingTitleScreen) layoutTranslations(gtx layout.Context, rect image.Rectangle) {
	panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: unit.Dp(10)}.layout(gtx, rect)
	innerX := rect.Min.X + gtx.Dp(unit.Dp(16))
	innerRight := rect.Max.X - gtx.Dp(unit.Dp(16))
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	label := "ПЕРЕВОДЫ"
	if len(w.item.voices) == 0 {
		label = "ПЕРЕВОД"
	}
	w.typo.draw(gtx, label, labelFace, titlePageSectionSize, mainMutedColor, image.Pt(innerX, rect.Min.Y+gtx.Dp(unit.Dp(13))))
	top := rect.Min.Y + gtx.Dp(unit.Dp(42))
	if top >= rect.Max.Y {
		return
	}
	if len(w.voiceClick) != len(w.item.voices) {
		w.voiceClick = make([]widget.Clickable, len(w.item.voices))
	}
	rowHeight := gtx.Dp(unit.Dp(46))
	stack := op.Offset(image.Pt(innerX, top)).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(image.Pt(innerRight-innerX, rect.Max.Y-top))
	w.voiceList.Layout(lgtx, len(w.item.voices), func(gtx layout.Context, index int) layout.Dimensions {
		if index >= len(w.item.voices) || index >= len(w.voiceClick) {
			return layout.Dimensions{Size: image.Pt(innerRight-innerX, rowHeight)}
		}
		voice := w.item.voices[index]
		click := &w.voiceClick[index]
		selected := index == w.selectedVoice
		clicked := w.layoutTitleClickable(gtx, image.Rect(0, 0, innerRight-innerX, rowHeight-2), click, func(gtx layout.Context, local image.Rectangle) {
			fill := titleVoiceFill
			border := titleVoiceBorder
			text := titleVoiceText
			if selected {
				fill = titleVoiceSelectedFill
				border = titleVoiceSelectedBorder
				text = mainTextColor
			} else if click.Hovered() {
				fill = titleVoiceHoverFill
				border = titleVoiceHoverBorder
			}
			fillRounded(gtx, border, local, gtx.Dp(unit.Dp(7)))
			fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(6)))
			name := voiceDisplayName(voice.name, voice.id)
			w.typo.draw(gtx, name, font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12.5), text, image.Pt(gtx.Dp(unit.Dp(10)), gtx.Dp(unit.Dp(7))))
			note := fmt.Sprintf("%d гл.", len(voice.episodes))
			w.typo.draw(gtx, note, font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(10.5), readerMutedColor, image.Pt(gtx.Dp(unit.Dp(10)), gtx.Dp(unit.Dp(24))))
		})
		if clicked {
			w.selectVoice(index)
			gtx.Execute(op.InvalidateCmd{})
		}
		return layout.Dimensions{Size: image.Pt(innerRight-innerX, rowHeight)}
	})
	stack.Pop()
}

func (w *readingTitleScreen) layoutChapters(gtx layout.Context, rect image.Rectangle) {
	panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: unit.Dp(10)}.layout(gtx, rect)
	innerX := rect.Min.X + gtx.Dp(unit.Dp(16))
	innerRight := rect.Max.X - gtx.Dp(unit.Dp(16))
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	w.typo.draw(gtx, "ГЛАВЫ", labelFace, titlePageSectionSize, mainMutedColor, image.Pt(innerX, rect.Min.Y+gtx.Dp(unit.Dp(13))))
	top := rect.Min.Y + gtx.Dp(unit.Dp(42))
	if top >= rect.Max.Y {
		return
	}
	if len(w.chapterClick) != len(w.chapters) {
		w.chapterClick = make([]widget.Clickable, len(w.chapters))
	}
	if len(w.treeClick) != len(w.treeRows) {
		w.treeClick = make([]widget.Clickable, len(w.treeRows))
	}
	rowHeight := gtx.Dp(unit.Dp(36))
	stack := op.Offset(image.Pt(innerX, top)).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(image.Pt(innerRight-innerX, rect.Max.Y-top))
	w.chapterList.Layout(lgtx, len(w.treeRows), func(gtx layout.Context, index int) layout.Dimensions {
		item := image.Rect(0, 0, innerRight-innerX, rowHeight)
		if index >= len(w.treeRows) {
			return layout.Dimensions{Size: item.Size()}
		}
		row := w.treeRows[index]
		if row.header {
			if w.layoutReadingVolume(gtx, item, row, index) {
				w.pendingToggle = row.volume
			}
		} else {
			chapterIndex := chapterIndexForKey(w.chapters, row.chapter)
			if w.layoutReadingChapter(gtx, item, row, chapterIndex) {
				w.selectChapter(chapterIndex)
			}
		}
		return layout.Dimensions{Size: item.Size()}
	})
	stack.Pop()
	if volume := w.pendingToggle; volume != "" {
		w.pendingToggle = ""
		w.expanded[volume] = !w.expanded[volume]
		w.rebuildTree()
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (w *readingTitleScreen) layoutReadingVolume(gtx layout.Context, rect image.Rectangle, row readerTreeRow, index int) bool {
	if index < 0 || index >= len(w.treeClick) {
		return false
	}
	click := &w.treeClick[index]
	return w.layoutTitleClickable(gtx, rect, click, func(gtx layout.Context, local image.Rectangle) {
		if click.Hovered() {
			fillRounded(gtx, readerChapterHover, local, gtx.Dp(unit.Dp(7)))
		}
		label := readerVolumeLabel(row.volume)
		w.typo.draw(gtx, label, font.Font{Typeface: fontFamily, Weight: font.Bold}, unit.Sp(12.5), mainTextColor, image.Pt(gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(10))))
		note := fmt.Sprintf("%d/%d", row.available, row.total)
		_, noteInk := w.typo.measure(gtx, note, font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(10.5))
		w.typo.draw(gtx, note, font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(10.5), mainMutedColor, image.Pt(local.Dx()-gtx.Dp(unit.Dp(8))-noteInk.Dx(), gtx.Dp(unit.Dp(10))))
	})
}

func (w *readingTitleScreen) layoutReadingChapter(gtx layout.Context, rect image.Rectangle, row readerTreeRow, index int) bool {
	selected := index >= 0 && index == w.selectedChapter
	var click *widget.Clickable
	if index >= 0 && index < len(w.chapterClick) {
		click = &w.chapterClick[index]
	}
	draw := func(gtx layout.Context, local image.Rectangle) {
		if selected {
			fillRounded(gtx, readerChapterActive, local, gtx.Dp(unit.Dp(6)))
		} else if click != nil && click.Hovered() {
			fillRounded(gtx, readerChapterHover, local, gtx.Dp(unit.Dp(6)))
		}
		text := mainTextColor
		if row.missing {
			text = readerDisabledText
		} else if selected {
			text = titleColor
		}
		label := readingChapterLabel(row.chapter)
		w.typo.draw(gtx, label, font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12.5), text, image.Pt(gtx.Dp(unit.Dp(12)), gtx.Dp(unit.Dp(9))))
		if row.missing {
			w.typo.draw(gtx, "нет", font.Font{Typeface: fontFamily, Weight: font.Normal}, unit.Sp(10.5), readerDisabledText, image.Pt(local.Dx()-gtx.Dp(unit.Dp(28)), gtx.Dp(unit.Dp(10))))
		}
	}
	if row.missing || click == nil {
		draw(gtx, image.Rect(0, 0, rect.Dx(), rect.Dy()))
		return false
	}
	return w.layoutTitleClickable(gtx, rect, click, draw)
}

func (w *readingTitleScreen) layoutTitleClickable(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context, image.Rectangle)) bool {
	if rect.Empty() || click == nil {
		return false
	}
	return layoutClickableRect(gtx, rect, click, draw)
}

func (w *readingTitleScreen) layoutLoading(gtx layout.Context, rect image.Rectangle) {
	center := image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 3)})
	phase := float64(time.Now().UnixNano()) / 1e9
	phase = phase*2.2 - float64(int(phase*2.2))
	drawSpinner(gtx, center, gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(3)), phase, mainAccentColor1)
}

func (w *readingTitleScreen) layoutError(gtx layout.Context, rect image.Rectangle) {
	center := image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
	w.typo.drawCentered(gtx, w.loadErr, font.Font{Typeface: fontFamily, Weight: font.Medium}, unit.Sp(12.5), mainDangerColor, center)
}

func (w *readingTitleScreen) layoutFooter(gtx layout.Context) {
	if w.inner != nil {
		w.inner.layoutFooter(gtx)
		return
	}
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(titlePageButtonHeight)
	top := (size.Y - height) / 2
	backRect := image.Rect(padX, top, padX+gtx.Dp(unit.Dp(128)), top+height)
	if w.back.layout(gtx, w.typo, backRect) {
		if w.onBack != nil {
			w.onBack()
		}
		return
	}
	right := size.X - padX
	readWidth := gtx.Dp(unit.Dp(122))
	readRect := image.Rect(right-readWidth, top, right, top+height)
	right = readRect.Min.X - gtx.Dp(titlePageFooterButtonGap)
	saveWidth := gtx.Dp(unit.Dp(112))
	saveRect := image.Rect(right-saveWidth, top, right, top+height)
	label := "Сохранить"
	style := footerSecondaryStyle
	if !w.savedUntil.IsZero() && gtx.Now.Before(w.savedUntil) {
		label = "Сохранено"
		style = footerSavedStyle
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: w.savedUntil})
		}
	}
	w.saveButton.label = label
	w.saveButton.style = style
	if w.saveButton.layout(gtx, w.typo, saveRect) {
		w.save(gtx)
	}
	w.readButton.disabled = w.selectedChapter < 0 || w.selectedChapter >= len(w.chapters)
	w.readButton.label = "Читать"
	if w.readButton.layout(gtx, w.typo, readRect) {
		w.read()
	}
}

func (w *readingTitleScreen) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

func (w *readingTitleScreen) Close() {
	if w.loadCancel != nil {
		w.loadCancel()
		w.loadCancel = nil
	}
}
