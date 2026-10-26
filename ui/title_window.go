package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yonime/internal/library"
)

const (
	titlePagePadX            = unit.Dp(28)
	titlePagePadTop          = unit.Dp(32)
	titlePagePosterWidth     = unit.Dp(86)
	titlePagePosterHeight    = unit.Dp(120)
	titlePagePosterGap       = unit.Dp(18)
	titlePageColumnGap       = unit.Dp(22)
	titlePageColumnsGapTop   = unit.Dp(6)
	titlePagePanelBottomGap  = unit.Dp(14)
	titlePageVoiceHeight     = unit.Dp(36)
	titlePageVoiceGap        = unit.Dp(6)
	titlePageVoiceTailSpace  = unit.Dp(14)
	titlePageEpisodeMinWidth = unit.Dp(44)
	titlePageEpisodeHeight   = unit.Dp(42)
	titlePageEpisodeGap      = unit.Dp(6)
	titlePageQualityHeight   = unit.Dp(34)
	titlePageButtonHeight    = unit.Dp(34)
	titlePageFooterButtonGap = unit.Dp(12)
)

const (
	titlePageTitleSize    = unit.Sp(21)
	titlePageOriginalSize = unit.Sp(12.5)
	titlePageSectionSize  = unit.Sp(10.5)
	titlePageVoiceSize    = unit.Sp(12)
	titlePageEpisodeSize  = unit.Sp(12)
	titlePageQualitySize  = unit.Sp(11.5)
	titlePageNoteSize     = unit.Sp(10.5)
	titlePageButtonSize   = unit.Sp(12)
	titlePageTooltipSize  = unit.Sp(10.5)
)

var (
	titlePanelFill   = color.NRGBA{R: 0x0f, G: 0x0f, B: 0x17, A: 0xff}
	titlePanelBorder = color.NRGBA{R: 0x20, G: 0x1f, B: 0x2b, A: 0xff}

	titleVoiceFill   = color.NRGBA{R: 0x14, G: 0x13, B: 0x1d, A: 0xff}
	titleVoiceBorder = color.NRGBA{R: 0x26, G: 0x23, B: 0x31, A: 0xff}
	titleVoiceText   = color.NRGBA{R: 0xc9, G: 0xc5, B: 0xd4, A: 0xff}

	titleVoiceSelectedFill   = color.NRGBA{R: 0x1b, G: 0x12, B: 0x30, A: 0xff}
	titleVoiceSelectedBorder = color.NRGBA{R: 0x8b, G: 0x5c, B: 0xf6, A: 0xff}

	titleVoiceIncompleteText = color.NRGBA{R: 0xf0, G: 0xc6, B: 0x74, A: 0xff}

	titleVoiceHoverFill   = color.NRGBA{R: 0x1b, G: 0x18, B: 0x26, A: 0xff}
	titleVoiceHoverBorder = color.NRGBA{R: 0x3e, G: 0x35, B: 0x52, A: 0xff}

	titleHelpFill        = color.NRGBA{R: 0x2a, G: 0x21, B: 0x14, A: 0xff}
	titleHelpBorder      = color.NRGBA{R: 0x9c, G: 0x70, B: 0x2f, A: 0xff}
	titleHelpHoverFill   = color.NRGBA{R: 0x3a, G: 0x2b, B: 0x14, A: 0xff}
	titleHelpHoverBorder = color.NRGBA{R: 0xc9, G: 0x91, B: 0x3f, A: 0xff}

	titleTooltipFill   = color.NRGBA{R: 0x22, G: 0x1b, B: 0x12, A: 0xff}
	titleTooltipBorder = color.NRGBA{R: 0x9c, G: 0x70, B: 0x2f, A: 0xff}
	titleTooltipText   = color.NRGBA{R: 0xf0, G: 0xc6, B: 0x74, A: 0xff}

	titleEpisodeFill   = color.NRGBA{R: 0x17, G: 0x16, B: 0x20, A: 0xff}
	titleEpisodeBorder = color.NRGBA{R: 0x27, G: 0x24, B: 0x32, A: 0xff}
	titleEpisodeText   = color.NRGBA{R: 0xcf, G: 0xcb, B: 0xd8, A: 0xff}
	titleEpisodeHover  = color.NRGBA{R: 0x21, G: 0x1d, B: 0x2e, A: 0xff}
	titleEpisodeMuted  = color.NRGBA{R: 0x55, G: 0x50, B: 0x62, A: 0xff}

	titleQualityFill   = color.NRGBA{R: 0x17, G: 0x16, B: 0x20, A: 0xff}
	titleQualityBorder = color.NRGBA{R: 0x27, G: 0x24, B: 0x32, A: 0xff}
	titleQualityText   = color.NRGBA{R: 0xcf, G: 0xcb, B: 0xd8, A: 0xff}
	titleQualityHover  = color.NRGBA{R: 0x21, G: 0x1d, B: 0x2e, A: 0xff}

	titleButtonFill        = color.NRGBA{R: 0x1b, G: 0x19, B: 0x24, A: 0xff}
	titleButtonBorder      = color.NRGBA{R: 0x2b, G: 0x27, B: 0x38, A: 0xff}
	titleButtonHover       = color.NRGBA{R: 0x22, G: 0x1c, B: 0x34, A: 0xff}
	titleButtonHoverBorder = color.NRGBA{R: 0x4a, G: 0x3c, B: 0x69, A: 0xff}
	titleButtonText        = color.NRGBA{R: 0xe4, G: 0xe0, B: 0xec, A: 0xff}

	titleSavedFill        = color.NRGBA{R: 0x12, G: 0x24, B: 0x1b, A: 0xff}
	titleSavedHover       = color.NRGBA{R: 0x18, G: 0x31, B: 0x24, A: 0xff}
	titleSavedBorder      = color.NRGBA{R: 0x2b, G: 0x52, B: 0x3d, A: 0xff}
	titleSavedHoverBorder = color.NRGBA{R: 0x3a, G: 0x6f, B: 0x53, A: 0xff}
	titleSavedText        = color.NRGBA{R: 0x7f, G: 0xcf, B: 0x9d, A: 0xff}
)

var titleQualityLabels = []string{"720 HD", "1080 FHD", "1440 2K", "2160 4K"}
var titleQualityValues = []int{720, 1080, 1440, 2160}

func titleQualityIndex(value int) int {
	if value <= 0 {
		return 1
	}
	best := 1
	bestDistance := int(^uint(0) >> 1)
	for index, quality := range titleQualityValues {
		distance := quality - value
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best = index
			bestDistance = distance
		}
	}
	return best
}

type titleVoiceRow struct {
	click widget.Clickable
	help  widget.Clickable
	kodik widget.Clickable

	hover      hoverAnim
	helpHover  hoverAnim
	kodikHover hoverAnim
}

type titleWindow struct {
	typo *typography
	item historyTitle

	voiceList   layout.List
	episodeList layout.List
	voices      []titleVoiceRow
	episodes    []widget.Clickable
	qualities   []widget.Clickable

	selectedVoice   int
	selectedEpisode int
	selectedQuality int
	focusCleared    bool
	// progressEpisode is the saved episode whose playback position fills the
	// header progress bar. Selecting another episode clears the fill until that
	// episode is watched.
	progressEpisode int

	back        footerNamedButton
	saveButton  footerNamedButton
	watchButton footerNamedButton

	userStatusClick widget.Clickable

	statusMenu userStatusMenu

	tooltipIndex  int
	tooltipKodik  bool
	tooltipAnchor image.Point

	savedUntil time.Time
	saveError  string

	watching   bool
	watchError string

	// hidden reports that the window is parked in the notification area, so the
	// transient "Сохранено" ticks stop scheduling frames.
	hidden func() bool

	onBack       func()
	onSave       func(voice, episode string) error
	onQuality    func(index int) error
	onWatch      func(voice, episode string, quality int) error
	onUserStatus func(historyTitle, library.UserStatus) error
	onError      func(error)
}

// setUserStatus updates the local pill without touching the database. The app
// calls it after persistence or after the user changes an unsaved title.
func (w *titleWindow) setUserStatus(status library.UserStatus) {
	w.item.userStatus = library.NormalizeUserStatus(status)
}

// dismissPopup closes the status menu when the window loses focus.
func (w *titleWindow) dismissPopup() {
	w.statusMenu.close()
}

func (w *titleWindow) canChangeUserStatus() bool {
	return w.item.status != statusAnnounced
}

// showsUserStatus reports whether the title already has a history row. A
// link-opened title must not advertise the default "В планах" pill before the
// user saves it; playback tracking also assigns an id, after which the pill is
// meaningful again.
func (w *titleWindow) showsUserStatus() bool {
	return w.item.id != 0
}

// hasEpisodeColumns reports whether the voiceover and episode panel should be
// laid out. A title without a loaded episode list has nothing to choose from,
// so the columns and the preferred-quality block are omitted entirely.
func (w *titleWindow) hasEpisodeColumns() bool {
	return len(w.item.episodes) > 0
}

func (w *titleWindow) layoutOverlay(gtx layout.Context, origin image.Point) {
	if !w.statusMenu.open {
		return
	}
	id := w.item.id
	w.statusMenu.layout(gtx, w.typo, origin, func(status library.UserStatus) {
		if !w.canChangeUserStatus() {
			return
		}
		previous := w.item.userStatus
		w.setUserStatus(status)
		if w.onUserStatus != nil {
			if err := w.onUserStatus(historyTitle{id: id}, status); err != nil {
				w.setUserStatus(previous)
			}
		}
	})
}

func newTitleWindow(typo *typography, item historyTitle, preferredQuality int) *titleWindow {
	selectedVoice := item.selectedVoice
	if selectedVoice < 0 || selectedVoice >= len(item.voices) {
		selectedVoice = 0
	}
	selectedEpisode := item.selectedEpisode
	if selectedEpisode < 0 || selectedEpisode >= len(item.episodes) {
		selectedEpisode = 0
	}
	if len(item.episodes) == 0 {
		selectedEpisode = -1
	}
	// The bar belongs to the episode whose position was saved, not to the
	// fallback selection used when that episode is missing from the voiceover.
	progressEpisode := selectedEpisode
	if index := episodeIndex(item, item.episodeLabel); index >= 0 {
		progressEpisode = index
	}
	w := &titleWindow{
		typo:            typo,
		item:            item,
		selectedVoice:   selectedVoice,
		selectedEpisode: selectedEpisode,
		selectedQuality: titleQualityIndex(preferredQuality),
		progressEpisode: progressEpisode,
		tooltipIndex:    -1,
		back:            newBackFooterButton(),
		saveButton: footerNamedButton{
			label: "Сохранить",
			style: footerSecondaryStyle,
			size:  titlePageButtonSize,
		},
		watchButton: newWatchFooterButton(),
	}
	w.voiceList.Axis = layout.Vertical
	w.voiceList.Gap = 0
	w.episodeList.Axis = layout.Vertical
	w.episodeList.Gap = 0
	w.voices = make([]titleVoiceRow, len(item.voices))
	if len(item.episodes) > 0 {
		w.episodes = make([]widget.Clickable, len(item.episodes))
	}
	w.qualities = make([]widget.Clickable, len(titleQualityLabels))
	return w
}

func (w *titleWindow) Close() {}

// animationsRun reports whether this screen may keep scheduling its own frames.
func (w *titleWindow) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

// applySaved adopts the local id and poster assigned after the title was
// persisted, so pressing Save again updates the same history entry. The
// current voice and episode selection are left untouched.
func (w *titleWindow) applySaved(item historyTitle) {
	w.item.id = item.id
	w.item.slug = item.slug
	w.item.coverPath = item.coverPath
	w.item.coverNeedsRetry = item.coverNeedsRetry
}

// applyProgress adopts the saved position after playback ends.
func (w *titleWindow) applyProgress(episode string, resumeAt, duration float64) {
	w.item.resumeAt = resumeAt
	w.item.duration = duration
	if index := episodeIndex(w.item, episode); index >= 0 {
		w.selectedEpisode = index
		w.progressEpisode = index
	}
}

// layoutHeaderProgress paints the watched-episode bar under the status row.
// rect ends at the columns below the header, so a short window simply skips it.
// A film has no series counter, so only its progress bar is drawn.
func (w *titleWindow) layoutHeaderProgress(gtx layout.Context, rect image.Rectangle) {
	label := w.item.progressLabel()
	barHeight := gtx.Dp(episodeProgressHeight)
	if rect.Empty() || (!w.item.movie && label == "") {
		return
	}
	labelFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	var labelInk image.Rectangle
	if label != "" {
		_, labelInk = w.typo.measure(gtx, label, labelFace, episodeProgressLabelSz)
	}
	requiredHeight := barHeight
	if labelInk.Dy() > requiredHeight {
		requiredHeight = labelInk.Dy()
	}
	if rect.Dy() < requiredHeight {
		return
	}
	gap := gtx.Dp(episodeProgressGap)
	barWidth := rect.Dx() * 28 / 100
	if barWidth < gtx.Dp(unit.Dp(120)) {
		barWidth = gtx.Dp(unit.Dp(120))
	}
	maxBar := rect.Dx()
	if labelInk.Dx() > 0 {
		maxBar -= gap + labelInk.Dx()
	}
	if barWidth > maxBar {
		barWidth = maxBar
	}
	if barWidth < gtx.Dp(unit.Dp(56)) {
		return
	}
	bar := image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+barWidth, rect.Min.Y+barHeight)
	progress := float32(0)
	if w.selectedEpisode == w.progressEpisode {
		progress = episodeProgressValue(w.item.resumeAt, w.item.duration)
	}
	drawEpisodeProgressBar(gtx, bar, progress)
	if label == "" {
		return
	}
	w.typo.draw(
		gtx,
		label,
		labelFace,
		episodeProgressLabelSz,
		episodeProgressLabelCol,
		image.Pt(bar.Max.X+gap, bar.Min.Y+(barHeight-labelInk.Dy())/2),
	)
}

func (w *titleWindow) setWatching(value bool) {
	w.watching = value
	if value {
		w.watchError = ""
	}
}

func (w *titleWindow) setWatchError(message string) {
	w.watching = false
	w.watchError = message
}

func (w *titleWindow) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameLeftArrow},
			key.Filter{Name: key.NameRightArrow},
			key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameDownArrow},
			key.Filter{Name: key.NameReturn},
			key.Filter{Name: key.NameEnter},
			key.Filter{Name: "S"},
			key.Filter{Name: "s"},
			key.Filter{Name: "Ы"},
			key.Filter{Name: "ы"},
		)
		if !ok {
			break
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			if w.statusMenu.open {
				w.statusMenu.close()
				gtx.Execute(op.InvalidateCmd{})
				continue
			}
			if w.onBack != nil {
				w.onBack()
			}
			continue
		}
		if w.statusMenu.open {
			continue
		}
		switch {
		case e.Name == key.NameLeftArrow:
			w.stepEpisode(gtx, -1)
		case e.Name == key.NameRightArrow:
			w.stepEpisode(gtx, 1)
		case e.Name == key.NameUpArrow:
			w.stepVoice(gtx, -1)
		case e.Name == key.NameDownArrow:
			w.stepVoice(gtx, 1)
		case isSaveKey(e.Name):
			w.save(gtx)
		case e.Name == key.NameReturn, e.Name == key.NameEnter:
			w.watch(gtx)
		}
	}
}

// stepEpisode moves the episode cursor by delta and skips episodes missing
// from the selected voiceover. It does not wrap: the ends are hard stops.
func (w *titleWindow) stepEpisode(gtx layout.Context, delta int) {
	if len(w.item.episodes) == 0 {
		return
	}
	index := w.selectedEpisode
	if index < 0 || index >= len(w.item.episodes) {
		index = 0
	} else {
		index += delta
	}
	for index >= 0 && index < len(w.item.episodes) {
		if !w.episodeMissing(w.item.episodes[index]) {
			w.selectedEpisode = index
			w.episodeList.ScrollTo(index)
			gtx.Execute(op.InvalidateCmd{})
			return
		}
		index += delta
	}
}

func (w *titleWindow) stepVoice(gtx layout.Context, delta int) {
	count := len(w.item.voices)
	if count == 0 {
		return
	}
	index := w.selectedVoice
	if index < 0 || index >= count {
		index = 0
	} else {
		index += delta
	}
	if index < 0 || index >= count {
		return
	}
	previousEpisode := w.selectedEpisode
	w.selectVoice(index)
	w.voiceList.ScrollTo(index)
	if w.selectedEpisode != previousEpisode {
		w.episodeList.ScrollTo(w.selectedEpisode)
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (w *titleWindow) layout(gtx layout.Context) {
	// The URL editor from the main screen can still own keyboard focus when
	// the title page opens. Clear it once so arrow keys reach the episode
	// navigation below instead of being consumed by the hidden editor.
	if !w.focusCleared {
		w.focusCleared = true
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	w.handleKeys(gtx)
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
	if poster.Empty() {
		return
	}
	drawPosterArt(gtx, w.typo, poster, w.item, unit.Sp(32))

	headerX := poster.Max.X + gtx.Dp(titlePagePosterGap)
	statusWidth := statusPillWidth(gtx, w.typo)
	statusHeight := gtx.Dp(mainStatusHeight)
	titleFace := font.Font{Typeface: fontFamily, Weight: font.Bold}
	_, titleInk := w.typo.measure(gtx, w.item.title, titleFace, titlePageTitleSize)
	titleTop := poster.Min.Y + gtx.Dp(19)
	titleMax := size.X - padX - headerX
	drawTruncatedText(
		gtx,
		w.typo,
		w.item.title,
		titleFace,
		titlePageTitleSize,
		mainTextColor,
		image.Pt(headerX, titleTop),
		titleMax,
	)

	// The English title used to sit on this line. It is replaced by the two
	// status tags in the same order as history: release first, user second.
	statusLineTop := titleTop + max(titleInk.Dy(), statusHeight) + gtx.Dp(10)
	releaseRect := image.Rect(
		headerX,
		statusLineTop,
		headerX+statusWidth,
		statusLineTop+statusHeight,
	)
	drawStatusPill(gtx, w.typo, releaseRect, w.item.status)

	if w.showsUserStatus() {
		statusGap := gtx.Dp(mainStatusGap)
		userRect := image.Rect(
			releaseRect.Max.X+statusGap,
			statusLineTop,
			releaseRect.Max.X+statusGap+statusWidth,
			statusLineTop+statusHeight,
		)
		userClicked := w.userStatusClick.Clicked(gtx)
		statusStack := op.Offset(userRect.Min).Push(gtx.Ops)
		statusGtx := gtx
		statusGtx.Constraints = layout.Exact(userRect.Size())
		w.userStatusClick.Layout(statusGtx, func(gtx layout.Context) layout.Dimensions {
			if w.canChangeUserStatus() {
				pointer.CursorPointer.Add(gtx.Ops)
			}
			drawUserStatusPill(gtx, w.typo, image.Rect(0, 0, userRect.Dx(), userRect.Dy()), w.item.userStatus, true)
			return layout.Dimensions{Size: userRect.Size()}
		})
		statusStack.Pop()
		if userClicked && w.canChangeUserStatus() {
			w.statusMenu.show(userRect, w.item.userStatus, w.item.id, userRect.Dx(), userRect.Dy())
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	// Keep the bar below the status row, with enough breathing room that it
	// reads as a separate block rather than part of the pill line.
	headerProgressTop := statusLineTop + statusHeight + gtx.Dp(22)

	columnsTop := poster.Max.Y + gtx.Dp(titlePageColumnsGapTop)
	w.layoutHeaderProgress(gtx, image.Rect(headerX, headerProgressTop, size.X-padX, columnsTop))
	if w.item.status == statusAnnounced {
		w.drawAnnouncement(gtx, image.Rect(padX, columnsTop, size.X-padX, size.Y))
		return
	}
	if !w.hasEpisodeColumns() {
		// Nothing is loaded to choose from: omit both columns and the quality
		// preference instead of painting an empty panel.
		return
	}
	// The voice column reaches the content bottom, so its rows disappear under
	// the footer. The series panel keeps a small gap above the footer instead
	// of looking glued to it.
	bottom := size.Y
	if bottom <= columnsTop+gtx.Dp(80) {
		return
	}
	panelBottom := bottom - gtx.Dp(titlePagePanelBottomGap)
	if panelBottom <= columnsTop+gtx.Dp(80) {
		panelBottom = bottom
	}

	columnGap := gtx.Dp(titlePageColumnGap)
	leftWidth := int(float32(available) * 0.40)
	if maxLeft := gtx.Dp(unit.Dp(305)); leftWidth > maxLeft {
		leftWidth = maxLeft
	}
	if minRight := gtx.Dp(unit.Dp(270)); available-leftWidth-columnGap < minRight {
		leftWidth = available - columnGap - minRight
	}
	if leftWidth < gtx.Dp(unit.Dp(190)) {
		leftWidth = gtx.Dp(unit.Dp(190))
	}
	left := image.Rect(padX, columnsTop, padX+leftWidth, bottom)
	right := image.Rect(left.Max.X+columnGap, columnsTop, size.X-padX, panelBottom)
	if left.Empty() || right.Empty() {
		return
	}

	rowsTop := w.layoutVoiceLabel(gtx, left)
	w.layoutVoices(gtx, image.Rect(left.Min.X, rowsTop, left.Max.X, left.Max.Y))
	w.layoutTitlePanel(gtx, right)
	w.layoutTooltip(gtx, size)
}

func (w *titleWindow) drawAnnouncement(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	drawTruncatedText(
		gtx,
		w.typo,
		"Тайтл ещё не вышел. Озвучки и серии появятся после премьеры.",
		font.Font{Typeface: fontFamily, Weight: font.Normal},
		titlePageOriginalSize,
		mainMutedColor,
		image.Pt(rect.Min.X, rect.Min.Y+gtx.Dp(unit.Dp(4))),
		rect.Dx(),
	)
}

func (w *titleWindow) layoutVoiceLabel(gtx layout.Context, rect image.Rectangle) int {
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, "ОЗВУЧКА", face, titlePageSectionSize)
	top := rect.Min.Y + gtx.Dp(unit.Dp(5))
	w.typo.draw(gtx, "ОЗВУЧКА", face, titlePageSectionSize, mainMutedColor, image.Pt(rect.Min.X, top))
	return top + ink.Dy() + gtx.Dp(unit.Dp(7))
}

func (w *titleWindow) layoutVoices(gtx layout.Context, rect image.Rectangle) {
	w.tooltipIndex = -1
	w.tooltipKodik = false
	if rect.Empty() || len(w.item.voices) == 0 {
		return
	}
	rowHeight := gtx.Dp(titlePageVoiceHeight)
	gap := gtx.Dp(titlePageVoiceGap)
	w.voiceList.Gap = gap

	voiceCount := len(w.item.voices)
	tailHeight := gtx.Dp(titlePageVoiceTailSpace)
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	// The synthetic tail row matches the history list: the last voice can
	// scroll up slightly so it is not pressed against the footer.
	w.voiceList.Layout(lgtx, voiceCount+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == voiceCount {
			return layout.Dimensions{Size: image.Pt(rect.Dx(), tailHeight)}
		}
		w.layoutVoiceRow(gtx, index, rect.Dx(), rowHeight)
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	stack.Pop()

	if w.tooltipIndex >= 0 && w.tooltipIndex < len(w.item.voices) {
		stride := rowHeight + gap
		if stride > 0 {
			rowTop := rect.Min.Y + (w.tooltipIndex-w.voiceList.Position.First)*stride - w.voiceList.Position.Offset
			w.tooltipAnchor = image.Pt(rect.Max.X+gtx.Dp(unit.Dp(8)), rowTop+rowHeight/2)
		}
	}
}

func (w *titleWindow) layoutVoiceRow(gtx layout.Context, index, width, height int) {
	if index < 0 || index >= len(w.item.voices) {
		return
	}
	row := &w.voices[index]
	voice := w.item.voices[index]
	selected := index == w.selectedVoice
	incomplete := !voice.complete()
	hasKodik := voice.hasKodik

	_, clicked := row.click.Update(gtx)
	if clicked {
		w.selectVoice(index)
		gtx.Execute(op.InvalidateCmd{})
	}
	if incomplete {
		_, _ = row.help.Update(gtx)
	}
	if hasKodik {
		_, _ = row.kodik.Update(gtx)
	}
	hovered := row.click.Hovered() || (incomplete && row.help.Hovered()) || (hasKodik && row.kodik.Hovered())
	hover := row.hover.update(gtx, hovered)
	helpHover := float32(0)
	if incomplete {
		helpHover = row.helpHover.update(gtx, row.help.Hovered())
		if row.help.Hovered() {
			w.tooltipIndex = index
			w.tooltipKodik = false
		}
	}
	kodikHover := float32(0)
	if hasKodik {
		kodikHover = row.kodikHover.update(gtx, row.kodik.Hovered())
		if row.kodik.Hovered() {
			w.tooltipIndex = index
			w.tooltipKodik = true
		}
	}

	rowRect := image.Rect(0, 0, width, height)
	fill := titleVoiceFill
	border := titleVoiceBorder
	text := titleVoiceText
	if selected {
		fill = titleVoiceSelectedFill
		border = titleVoiceSelectedBorder
		text = mainTextColor
	}
	if hover > 0 && !selected {
		fill = mixColor(fill, titleVoiceHoverFill, hover)
		border = mixColor(border, titleVoiceHoverBorder, hover)
	}

	radius := gtx.Dp(unit.Dp(8))
	fillRounded(gtx, border, rowRect, radius)
	fillRounded(gtx, fill, rowRect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	dotColor := mainDragDotColor
	if selected {
		dotColor = mainAccentColor1
	}
	fillCircle(gtx, image.Pt(gtx.Dp(unit.Dp(15)), height/2), gtx.Dp(unit.Dp(4)), dotColor)

	textX := gtx.Dp(unit.Dp(29))
	badgeWidth := gtx.Dp(unit.Dp(26))
	badgeHeight := gtx.Dp(unit.Dp(20))
	badgeGap := gtx.Dp(unit.Dp(6))
	badgeTop := (height - badgeHeight) / 2
	badgeRight := width - gtx.Dp(unit.Dp(10))
	// The Kodik badge keeps the right edge; the missing-episodes badge sits to
	// its left so both signals stay visible on the same row.
	kodikRect := image.Rect(badgeRight-badgeWidth, badgeTop, badgeRight, badgeTop+badgeHeight)
	if hasKodik {
		badgeRight = kodikRect.Min.X - badgeGap
	}
	helpRect := image.Rect(badgeRight-badgeWidth, badgeTop, badgeRight, badgeTop+badgeHeight)
	textRight := rowRect.Max.X - gtx.Dp(unit.Dp(10))
	if incomplete {
		textRight = helpRect.Min.X - gtx.Dp(unit.Dp(7))
	} else if hasKodik {
		textRight = kodikRect.Min.X - gtx.Dp(unit.Dp(7))
	}
	_, textInk := w.typo.measure(gtx, voice.name, font.Font{Typeface: fontFamily, Weight: font.Medium}, titlePageVoiceSize)
	textY := (height - textInk.Dy()) / 2
	drawTruncatedText(
		gtx,
		w.typo,
		voice.name,
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageVoiceSize,
		text,
		image.Pt(textX, textY),
		textRight-textX,
	)

	if incomplete {
		w.drawVoiceBadge(gtx, helpRect, "!?", helpHover)
	}
	if hasKodik {
		w.drawVoiceBadge(gtx, kodikRect, "K", kodikHover)
	}

	rowRight := width - gtx.Dp(unit.Dp(42))
	if incomplete {
		rowRight = helpRect.Min.X - badgeGap
	} else if hasKodik {
		rowRight = kodikRect.Min.X - badgeGap
	}
	rowHit := image.Rect(0, 0, rowRight, height)
	w.layoutHitArea(gtx, rowHit, &row.click, true, nil)
	if incomplete {
		w.layoutHitArea(gtx, helpRect, &row.help, true, nil)
	}
	if hasKodik {
		w.layoutHitArea(gtx, kodikRect, &row.kodik, true, nil)
	}
}

// drawVoiceBadge paints one of the small square markers at the right of a
// voice row. hover is the badge's own hover progress.
func (w *titleWindow) drawVoiceBadge(gtx layout.Context, rect image.Rectangle, label string, hover float32) {
	fill := mixColor(titleHelpFill, titleHelpHoverFill, hover)
	border := mixColor(titleHelpBorder, titleHelpHoverBorder, hover)
	radius := gtx.Dp(unit.Dp(5))
	fillRounded(gtx, fill, rect, radius)
	strokeRoundedRect(gtx, rect.Inset(gtx.Dp(unit.Dp(1))), float32(radius), float32(gtx.Dp(unit.Dp(1))), border)
	w.typo.drawCentered(
		gtx,
		label,
		font.Font{Typeface: fontFamily, Weight: font.Bold},
		unit.Sp(9.5),
		titleVoiceIncompleteText,
		centerOf(rect),
	)
}

func (w *titleWindow) selectVoice(index int) {
	if index < 0 || index >= len(w.item.voices) {
		return
	}
	w.selectedVoice = index
	if w.episodeMissing(w.selectedEpisodeLabel()) {
		if next := nextAvailableEpisode(w.item, index, w.selectedEpisode); next >= 0 {
			w.selectedEpisode = next
		}
	}
}

func (w *titleWindow) selectedEpisodeLabel() string {
	if w.selectedEpisode < 0 || w.selectedEpisode >= len(w.item.episodes) {
		return ""
	}
	return w.item.episodes[w.selectedEpisode]
}

func (w *titleWindow) selectedVoiceName() string {
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		return ""
	}
	return w.item.voices[w.selectedVoice].name
}

func (w *titleWindow) episodeMissing(episode string) bool {
	if episode == "" || w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		return false
	}
	for _, missing := range w.item.voices[w.selectedVoice].missing {
		if missing == episode {
			return true
		}
	}
	return false
}

func (w *titleWindow) layoutTitlePanel(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	radius := gtx.Dp(unit.Dp(10))
	fillRounded(gtx, titlePanelBorder, rect, radius)
	fillRounded(gtx, titlePanelFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))

	innerX := rect.Min.X + gtx.Dp(unit.Dp(16))
	innerRight := rect.Max.X - gtx.Dp(unit.Dp(16))
	compact := rect.Dy() < gtx.Dp(unit.Dp(230))

	labelFace := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, labelInk := w.typo.measure(gtx, "СЕРИИ", labelFace, titlePageSectionSize)
	labelY := rect.Min.Y + gtx.Dp(unit.Dp(13))
	if w.item.movie {
		w.layoutMoviePanel(gtx, rect, labelFace, labelInk, labelY, compact)
		return
	}
	episodesLabel := episodeRangeLabel(w.item.episodes)
	w.typo.draw(gtx, episodesLabel, labelFace, titlePageSectionSize, mainMutedColor, image.Pt(innerX, labelY))

	episodeTop := labelY + labelInk.Dy() + gtx.Dp(unit.Dp(13))
	_, qualityLabelInk := w.typo.measure(gtx, "ПРЕДПОЧТИТЕЛЬНОЕ КАЧЕСТВО", labelFace, titlePageSectionSize)
	qualityHeight := gtx.Dp(titlePageQualityHeight)

	// Anchor the quality block to the panel's lower edge. The episode grid
	// then gets all remaining height and clips its last visible row at the
	// block boundary instead of leaving a painted strip above the footer.
	var qualityLabelTop, qualityTop, noteTop, gridGap int
	if compact {
		qualityHeight = gtx.Dp(unit.Dp(30))
		bottomPad := gtx.Dp(unit.Dp(8))
		labelGap := gtx.Dp(unit.Dp(4))
		gridGap = gtx.Dp(unit.Dp(8))
		qualityLabelTop = rect.Max.Y - bottomPad - qualityHeight - labelGap - qualityLabelInk.Dy()
		qualityTop = qualityLabelTop + qualityLabelInk.Dy() + labelGap
		noteTop = 0
	} else {
		noteHeight := gtx.Dp(unit.Dp(30))
		noteGap := gtx.Dp(unit.Dp(14))
		labelGap := gtx.Dp(unit.Dp(18))
		gridGap = gtx.Dp(unit.Dp(18))
		bottomPad := gtx.Dp(unit.Dp(14))
		noteTop = rect.Max.Y - bottomPad - noteHeight
		qualityTop = noteTop - noteGap - qualityHeight
		qualityLabelTop = qualityTop - labelGap - qualityLabelInk.Dy()
	}
	gridBottom := qualityLabelTop - gridGap
	if gridBottom > episodeTop {
		w.layoutEpisodes(gtx, image.Rect(innerX, episodeTop, innerRight, gridBottom))
	}
	if qualityLabelTop <= episodeTop || qualityLabelTop+qualityLabelInk.Dy() > rect.Max.Y {
		return
	}
	w.typo.draw(
		gtx,
		"ПРЕДПОЧТИТЕЛЬНОЕ КАЧЕСТВО",
		labelFace,
		titlePageSectionSize,
		mainMutedColor,
		image.Pt(innerX, qualityLabelTop),
	)

	if qualityTop+qualityHeight > rect.Max.Y {
		return
	}
	qualityRect := image.Rect(innerX, qualityTop, innerRight, qualityTop+qualityHeight)
	w.layoutQualities(gtx, qualityRect)

	if !compact {
		noteFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
		w.typo.draw(
			gtx,
			"Если выбранное качество недоступно для серии,",
			noteFace,
			titlePageNoteSize,
			mainFaintColor,
			image.Pt(innerX, noteTop),
		)
		w.typo.draw(
			gtx,
			"будет автоматически включено ближайшее меньшее.",
			noteFace,
			titlePageNoteSize,
			mainFaintColor,
			image.Pt(innerX, noteTop+gtx.Dp(unit.Dp(14))),
		)
	}
}

// layoutMoviePanel leaves only the quality preference for a film. A movie has
// no episode list to choose from, so the block sits at the top of the panel
// instead of being anchored to the bottom above the episode grid.
func (w *titleWindow) layoutMoviePanel(gtx layout.Context, rect image.Rectangle, face font.Font, labelInk image.Rectangle, labelY int, compact bool) {
	innerX := rect.Min.X + gtx.Dp(unit.Dp(16))
	innerRight := rect.Max.X - gtx.Dp(unit.Dp(16))
	qualityHeight := gtx.Dp(titlePageQualityHeight)
	if compact {
		qualityHeight = gtx.Dp(unit.Dp(30))
	}
	qualityTop := labelY + labelInk.Dy() + gtx.Dp(unit.Dp(13))
	if qualityTop+qualityHeight > rect.Max.Y {
		return
	}
	w.typo.draw(
		gtx,
		"ПРЕДПОЧТИТЕЛЬНОЕ КАЧЕСТВО",
		face,
		titlePageSectionSize,
		mainMutedColor,
		image.Pt(innerX, labelY),
	)
	w.layoutQualities(gtx, image.Rect(innerX, qualityTop, innerRight, qualityTop+qualityHeight))
	if compact {
		return
	}

	noteTop := qualityTop + qualityHeight + gtx.Dp(unit.Dp(14))
	noteFace := font.Font{Typeface: fontFamily, Weight: font.Normal}
	w.typo.draw(
		gtx,
		"Если выбранное качество недоступно,",
		noteFace,
		titlePageNoteSize,
		mainFaintColor,
		image.Pt(innerX, noteTop),
	)
	w.typo.draw(
		gtx,
		"будет автоматически включено ближайшее меньшее.",
		noteFace,
		titlePageNoteSize,
		mainFaintColor,
		image.Pt(innerX, noteTop+gtx.Dp(unit.Dp(14))),
	)
}

func (w *titleWindow) layoutEpisodes(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() || len(w.episodes) == 0 {
		return
	}
	gap := gtx.Dp(titlePageEpisodeGap)
	minWidth := gtx.Dp(titlePageEpisodeMinWidth)
	columns := 7
	for columns > 1 {
		width := (rect.Dx() - gap*(columns-1)) / columns
		if width >= minWidth {
			break
		}
		columns--
	}
	buttonWidth := (rect.Dx() - gap*(columns-1)) / columns
	if buttonWidth <= 0 {
		return
	}
	rowHeight := gtx.Dp(titlePageEpisodeHeight)
	rows := (len(w.episodes) + columns - 1) / columns
	w.episodeList.Axis = layout.Vertical
	w.episodeList.Gap = gap

	stack := op.Offset(rect.Min).Push(gtx.Ops)
	lgtx := gtx
	lgtx.Constraints = layout.Exact(rect.Size())
	w.episodeList.Layout(lgtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
		for column := 0; column < columns; column++ {
			index := row*columns + column
			if index >= len(w.episodes) {
				break
			}
			left := column * (buttonWidth + gap)
			cell := image.Rect(left, 0, left+buttonWidth, rowHeight)
			w.layoutEpisodeButton(gtx, index, cell)
		}
		return layout.Dimensions{Size: image.Pt(rect.Dx(), rowHeight)}
	})
	stack.Pop()
}

func (w *titleWindow) layoutEpisodeButton(gtx layout.Context, index int, rect image.Rectangle) {
	if rect.Empty() || index < 0 || index >= len(w.episodes) {
		return
	}
	click := &w.episodes[index]
	_, clicked := click.Update(gtx)
	missing := w.episodeMissing(w.item.episodes[index])
	if clicked && !missing {
		w.selectedEpisode = index
		gtx.Execute(op.InvalidateCmd{})
	}
	hovered := click.Hovered() && !missing

	if missing {
		stack := op.Offset(rect.Min).Push(gtx.Ops)
		w.drawEpisodeFace(gtx, index, image.Rectangle{Max: rect.Size()}, true, false)
		stack.Pop()
		return
	}
	w.layoutClickableDraw(gtx, rect, click, func(gtx layout.Context) {
		w.drawEpisodeFace(gtx, index, image.Rectangle{Max: rect.Size()}, false, hovered)
	})
}

func (w *titleWindow) drawEpisodeFace(gtx layout.Context, index int, local image.Rectangle, missing, hovered bool) {
	if index < 0 || index >= len(w.episodes) || local.Empty() {
		return
	}
	if index == w.selectedEpisode && !missing {
		fillRoundedGradient(gtx, local, local, gtx.Dp(unit.Dp(8)), mainAccentColor1, mainAccentColor2)
	} else {
		fill := titleEpisodeFill
		border := titleEpisodeBorder
		if hovered {
			fill = titleEpisodeHover
			border = titleVoiceHoverBorder
		}
		if missing {
			fill = titlePanelFill
			border = titlePanelBorder
		}
		fillRounded(gtx, border, local, gtx.Dp(unit.Dp(8)))
		fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(7)))
	}
	textColor := titleEpisodeText
	if missing {
		textColor = titleEpisodeMuted
	} else if index == w.selectedEpisode {
		textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	w.typo.drawCentered(
		gtx,
		w.item.episodes[index],
		font.Font{Typeface: fontFamily, Weight: font.Medium},
		titlePageEpisodeSize,
		textColor,
		centerOf(local),
	)
}

func (w *titleWindow) layoutQualities(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	gap := gtx.Dp(unit.Dp(8))
	n := len(w.qualities)
	if n == 0 {
		return
	}
	width := (rect.Dx() - gap*(n-1)) / n
	for i := range w.qualities {
		left := rect.Min.X + i*(width+gap)
		cell := image.Rect(left, rect.Min.Y, left+width, rect.Max.Y)
		if cell.Empty() {
			continue
		}
		click := &w.qualities[i]
		_, clicked := click.Update(gtx)
		if clicked {
			w.selectedQuality = i
			if w.onQuality != nil {
				if err := w.onQuality(i); err != nil {
					w.watchError = err.Error()
					if w.onError != nil {
						w.onError(err)
					}
				}
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		hovered := click.Hovered()
		w.layoutClickableDraw(gtx, cell, click, func(gtx layout.Context) {
			local := image.Rect(0, 0, cell.Dx(), cell.Dy())
			if i == w.selectedQuality {
				fillRoundedGradient(gtx, local, local, gtx.Dp(unit.Dp(8)), mainAccentColor1, mainAccentColor2)
			} else {
				fill := titleQualityFill
				border := titleQualityBorder
				if hovered {
					fill = titleQualityHover
					border = titleVoiceHoverBorder
				}
				fillRounded(gtx, border, local, gtx.Dp(unit.Dp(8)))
				fillRounded(gtx, fill, local.Inset(gtx.Dp(unit.Dp(1))), gtx.Dp(unit.Dp(7)))
			}
			textColor := titleQualityText
			if i == w.selectedQuality {
				textColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
			}
			w.typo.drawCentered(
				gtx,
				titleQualityLabels[i],
				font.Font{Typeface: fontFamily, Weight: font.Medium},
				titlePageQualitySize,
				textColor,
				centerOf(local),
			)
		})
	}
}

func (w *titleWindow) layoutTooltip(gtx layout.Context, size image.Point) {
	if w.tooltipIndex < 0 || w.tooltipIndex >= len(w.item.voices) {
		return
	}
	label := ""
	if w.tooltipKodik {
		label = "Будут серии с Kodik"
	} else {
		label = voiceMissingLabel(w.item.voices[w.tooltipIndex])
	}
	if label == "" {
		return
	}
	face := font.Font{Typeface: fontFamily, Weight: font.Medium}
	_, ink := w.typo.measure(gtx, label, face, titlePageTooltipSize)
	if ink.Empty() {
		return
	}
	padX := gtx.Dp(unit.Dp(10))
	padY := gtx.Dp(unit.Dp(7))
	tipWidth := ink.Dx() + 2*padX
	tipHeight := ink.Dy() + 2*padY
	x := w.tooltipAnchor.X
	y := w.tooltipAnchor.Y - tipHeight/2
	if x+tipWidth > size.X-gtx.Dp(unit.Dp(8)) {
		x = size.X - gtx.Dp(unit.Dp(8)) - tipWidth
	}
	if x < gtx.Dp(unit.Dp(8)) {
		x = gtx.Dp(unit.Dp(8))
	}
	if y < gtx.Dp(unit.Dp(8)) {
		y = gtx.Dp(unit.Dp(8))
	}
	if y+tipHeight > size.Y-gtx.Dp(unit.Dp(8)) {
		y = size.Y - gtx.Dp(unit.Dp(8)) - tipHeight
	}
	rect := image.Rect(x, y, x+tipWidth, y+tipHeight)
	radius := gtx.Dp(unit.Dp(6))
	fillRounded(gtx, titleTooltipBorder, rect, radius)
	fillRounded(gtx, titleTooltipFill, rect.Inset(gtx.Dp(unit.Dp(1))), radius-gtx.Dp(unit.Dp(1)))
	w.typo.draw(gtx, label, face, titlePageTooltipSize, titleTooltipText, image.Pt(rect.Min.X+padX, rect.Min.Y+padY))
}

func (w *titleWindow) layoutFooter(gtx layout.Context) {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	padX := gtx.Dp(unit.Dp(28))
	height := gtx.Dp(titlePageButtonHeight)
	top := (size.Y - height) / 2
	backWidth := gtx.Dp(unit.Dp(120))
	backRect := image.Rect(padX, top, padX+backWidth, top+height)

	saveWidth := gtx.Dp(unit.Dp(112))
	watchWidth := gtx.Dp(unit.Dp(122))
	right := size.X - padX
	var watchRect image.Rectangle
	if w.item.status != statusAnnounced {
		watchRect = image.Rect(right-watchWidth, top, right, top+height)
		right = watchRect.Min.X - gtx.Dp(titlePageFooterButtonGap)
	}
	saveRect := image.Rect(right-saveWidth, top, right, top+height)

	if w.back.layout(gtx, w.typo, backRect) {
		if w.onBack != nil {
			w.onBack()
		}
		return
	}

	saveLabel := "Сохранить"
	w.saveButton.style = footerSecondaryStyle
	if !w.savedUntil.IsZero() && gtx.Now.Before(w.savedUntil) {
		saveLabel = "Сохранено"
		w.saveButton.style = footerSavedStyle
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: w.savedUntil})
		}
	}
	w.saveButton.label = saveLabel
	if w.saveButton.layout(gtx, w.typo, saveRect) {
		w.save(gtx)
	}

	if !watchRect.Empty() {
		w.watchButton.label = "Смотреть"
		if w.watching {
			w.watchButton.label = "Запуск..."
		}
		if w.watchButton.layout(gtx, w.typo, watchRect) {
			w.watch(gtx)
		}
	}
}

// save is shared by the footer button and the S shortcut.
func (w *titleWindow) save(gtx layout.Context) {
	w.saveError = ""
	w.savedUntil = time.Time{}
	if w.onSave != nil {
		if err := w.onSave(w.selectedVoiceName(), w.selectedEpisodeLabel()); err != nil {
			w.saveError = err.Error()
			if w.onError != nil {
				w.onError(err)
			}
		}
	}
	if w.saveError == "" {
		w.savedUntil = gtx.Now.Add(1500 * time.Millisecond)
		if w.animationsRun() {
			gtx.Execute(op.InvalidateCmd{At: w.savedUntil})
		}
	}
	gtx.Execute(op.InvalidateCmd{})
}

// watch is shared by the footer button and Enter. The watch control is absent
// for announced titles, so the shortcut must enforce the same rule.
func (w *titleWindow) watch(gtx layout.Context) {
	if w.watching || w.item.status == statusAnnounced || w.onWatch == nil {
		return
	}
	w.watchError = ""
	if err := w.onWatch(w.selectedVoiceName(), w.selectedEpisodeLabel(), titleQualityValues[w.selectedQuality]); err != nil {
		w.watchError = err.Error()
		if w.onError != nil {
			w.onError(err)
		}
	} else {
		w.watching = true
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (w *titleWindow) shortcutInfo() shortcutInfo {
	items := []shortcutItem{
		{keys: "F1", label: "Открыть или закрыть сводку"},
	}
	if len(w.item.episodes) > 1 && !w.item.movie {
		items = append(items, shortcutItem{keys: "← / →", label: "Предыдущая или следующая серия"})
	}
	if len(w.item.voices) > 1 {
		items = append(items, shortcutItem{keys: "↑ / ↓", label: "Предыдущая или следующая озвучка"})
	}
	items = append(items, shortcutItem{keys: "S", label: "Сохранить изменения"})
	if w.item.status != statusAnnounced {
		items = append(items, shortcutItem{keys: "Enter", label: "Запустить просмотр"})
	}
	items = append(items,
		shortcutItem{keys: "Esc", label: "Вернуться или закрыть меню"},
	)
	return shortcutInfo{
		page:  "Страница тайтла",
		items: items,
	}
}

func (w *titleWindow) layoutHitArea(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, pointerCursor bool, draw func(layout.Context)) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		if pointerCursor {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		if draw != nil {
			draw(gtx)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
}

func (w *titleWindow) layoutClickableDraw(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context)) {
	stack := op.Offset(rect.Min).Push(gtx.Ops)
	defer stack.Pop()
	cgtx := gtx
	cgtx.Constraints = layout.Exact(rect.Size())
	click.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		if draw != nil {
			draw(gtx)
		}
		return layout.Dimensions{Size: rect.Size()}
	})
}

func drawChevronLeft(gtx layout.Context, center image.Point, size, width int, col color.NRGBA) {
	if size <= 0 {
		return
	}
	half := float32(size) / 2
	strokeLine(gtx,
		f32.Pt(float32(center.X)+half*0.35, float32(center.Y)-half*0.8),
		f32.Pt(float32(center.X)-half*0.55, float32(center.Y)),
		float32(width),
		col,
	)
	strokeLine(gtx,
		f32.Pt(float32(center.X)-half*0.55, float32(center.Y)),
		f32.Pt(float32(center.X)+half*0.35, float32(center.Y)+half*0.8),
		float32(width),
		col,
	)
}
