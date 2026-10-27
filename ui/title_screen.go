package ui

import (
	"image"
	"image/color"
	"sort"
	"strconv"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/arfush/yo/catalog"
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
	titlePageQualityMinWidth = unit.Dp(56)
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

// titleStandardQualities are the presets the quality block has always offered.
// A Kinopoisk title keeps them after its ladder is known so a missing step is
// visible as a blocked button, not as a silently absent one.
var titleStandardQualities = []int{720, 1080, 1440, 2160}

// qualityOption is one button of the quality block.
type qualityOption struct {
	value   int
	label   string
	enabled bool
}

// titleQualityOptions builds the buttons for one voice. A voice without a known
// ladder (AnimeLIB, or a Kinopoisk title before the background probe returns)
// keeps the standard four. A known ladder keeps the standard rows and disables
// the missing ones; a voice that offers none of them (only 360/480) switches to
// its real steps instead of showing four dead buttons.
func titleQualityOptions(available []int, preferred int) ([]qualityOption, int) {
	present := make(map[int]bool, len(available))
	for _, value := range available {
		if value > 0 {
			present[value] = true
		}
	}
	hasStandard := false
	for _, value := range titleStandardQualities {
		if present[value] {
			hasStandard = true
			break
		}
	}

	var options []qualityOption
	switch {
	case len(present) == 0:
		for _, value := range titleStandardQualities {
			options = append(options, qualityOption{value: value, label: titleQualityLabel(value), enabled: true})
		}
	case hasStandard:
		for _, value := range titleStandardQualities {
			options = append(options, qualityOption{value: value, label: titleQualityLabel(value), enabled: present[value]})
		}
	default:
		values := make([]int, 0, len(present))
		for value := range present {
			values = append(values, value)
		}
		sort.Ints(values)
		for _, value := range values {
			options = append(options, qualityOption{value: value, label: titleQualityLabel(value), enabled: true})
		}
	}
	return options, titleQualitySelection(options, preferred)
}

// titleQualitySelection picks the enabled button nearest to the preferred
// quality, preferring the lower step exactly like playback does.
func titleQualitySelection(options []qualityOption, preferred int) int {
	if len(options) == 0 {
		return 0
	}
	if preferred <= 0 {
		preferred = 1080
	}
	lower := -1
	for index, option := range options {
		if !option.enabled {
			continue
		}
		if option.value == preferred {
			return index
		}
		if option.value < preferred {
			lower = index
		}
	}
	if lower >= 0 {
		return lower
	}
	for index, option := range options {
		if option.enabled {
			return index
		}
	}
	return 0
}

func titleQualityLabel(value int) string {
	switch value {
	case 360:
		return "360 SD"
	case 480:
		return "480 SD"
	case 720:
		return "720 HD"
	case 1080:
		return "1080 FHD"
	case 1440:
		return "1440 2K"
	case 2160:
		return "2160 4K"
	}
	return strconv.Itoa(value)
}

// qualitySectionLabel names the block. The word "preferred" was dropped for
// everyone: the strip is just the quality of the current voice.
func (w *titleScreen) qualitySectionLabel() string {
	return "КАЧЕСТВО"
}

type titleVoiceRow struct {
	click widget.Clickable
	help  widget.Clickable
	kodik widget.Clickable

	hover      hoverAnim
	helpHover  hoverAnim
	kodikHover hoverAnim
}

type titleScreen struct {
	typo *typography
	item historyTitle

	voiceList   layout.List
	episodeList layout.List
	seasonList  layout.List
	qualityList layout.List
	voices      []titleVoiceRow
	episodes    []widget.Clickable
	seasons     []widget.Clickable
	qualities   []widget.Clickable

	// qualityOptions is what the quality block draws; it follows the selected
	// voice's ladder and the probe result.
	qualityOptions []qualityOption

	voicesPlaced   bool
	episodesPlaced bool
	seasonsPlaced  bool
	episodeColumns int

	selectedVoice   int
	selectedEpisode int
	// selectedSeason is the season whose episodes the grid shows. It always
	// follows selectedEpisode: switching a season selects that season's first
	// available episode.
	selectedSeason  int
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
	onQuality    func(value int) error
	onWatch      func(voice, episode string, quality int) error
	onUserStatus func(historyTitle, catalog.UserStatus) error
	onVoice      func(index int)
	onError      func(error)
}

// setUserStatus updates the local pill without touching the database. The app
// calls it after persistence or after the user changes an unsaved title.
func (w *titleScreen) setUserStatus(status catalog.UserStatus) {
	w.item.userStatus = catalog.NormalizeUserStatus(status)
}

// dismissPopup closes the status menu when the window loses focus.
func (w *titleScreen) dismissPopup() {
	w.statusMenu.close()
}

func (w *titleScreen) canChangeUserStatus() bool {
	return w.item.status != statusAnnounced
}

// showsUserStatus reports whether the title already has a history row. A
// link-opened title must not advertise the default "В планах" pill before the
// user saves it; playback tracking also assigns an id, after which the pill is
// meaningful again.
func (w *titleScreen) showsUserStatus() bool {
	return w.item.id != 0
}

// hasEpisodeColumns reports whether the voiceover and episode panel should be
// laid out. A title without a loaded episode list has nothing to choose from,
// so the columns and the preferred-quality block are omitted entirely.
func (w *titleScreen) hasEpisodeColumns() bool {
	return len(w.item.episodes) > 0
}

func (w *titleScreen) layoutOverlay(gtx layout.Context, origin image.Point) {
	if !w.statusMenu.open {
		return
	}
	id := w.item.id
	w.statusMenu.layout(gtx, w.typo, origin, func(status catalog.UserStatus) {
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

func newTitleScreen(typo *typography, item historyTitle, preferredQuality int) *titleScreen {
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
	w := &titleScreen{
		typo:            typo,
		item:            item,
		selectedVoice:   selectedVoice,
		selectedEpisode: selectedEpisode,
		selectedSeason:  item.seasonIndexForEpisode(selectedEpisode),
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
	if len(item.seasons) > 1 {
		w.seasons = make([]widget.Clickable, len(item.seasons))
	}
	w.qualityOptions, w.selectedQuality = titleQualityOptions(w.selectedVoiceQualities(), preferredQuality)
	w.qualities = make([]widget.Clickable, len(w.qualityOptions))
	return w
}

// selectedVoiceQualities returns the ladder of the current voice, or nil while
// it is unknown (AnimeLIB, or a Kinopoisk title waiting for the probe).
func (w *titleScreen) selectedVoiceQualities() []int {
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		return nil
	}
	return w.item.voices[w.selectedVoice].qualities
}

func (w *titleScreen) selectedQualityValue() int {
	if w.selectedQuality < 0 || w.selectedQuality >= len(w.qualityOptions) {
		return 0
	}
	return w.qualityOptions[w.selectedQuality].value
}

// refreshQualityOptions rebuilds the buttons after a voice switch or a probe
// result. The current quality is kept when the new ladder offers it; otherwise
// the nearest lower enabled step wins, matching playback.
func (w *titleScreen) refreshQualityOptions(preferred int) {
	current := w.selectedQualityValue()
	if current <= 0 {
		current = preferred
	}
	w.qualityOptions, w.selectedQuality = titleQualityOptions(w.selectedVoiceQualities(), current)
	w.qualities = make([]widget.Clickable, len(w.qualityOptions))
}

// applyQualities stores a probed ladder for one voice. The probe runs in the
// background, so the result may belong to a voice the user has already left.
func (w *titleScreen) applyQualities(voice int, values []int) {
	if voice < 0 || voice >= len(w.item.voices) || len(values) == 0 {
		return
	}
	w.item.voices[voice].qualities = append([]int(nil), values...)
	if voice == w.selectedVoice {
		w.refreshQualityOptions(0)
	}
}

func (w *titleScreen) Close() {}

// animationsRun reports whether this screen may keep scheduling its own frames.
func (w *titleScreen) animationsRun() bool {
	return w.hidden == nil || !w.hidden()
}

// applySaved adopts the local id and poster assigned after the title was
// persisted, so pressing Save again updates the same history entry. The
// current voice and episode selection are left untouched.
func (w *titleScreen) applySaved(item historyTitle) {
	w.item.id = item.id
	w.item.slug = item.slug
	w.item.coverPath = item.coverPath
	w.item.coverNeedsRetry = item.coverNeedsRetry
}

// applyProgress adopts the saved position after playback ends.
func (w *titleScreen) applyProgress(episode string, resumeAt, duration float64) {
	w.item.resumeAt = resumeAt
	w.item.duration = duration
	if index := episodeIndex(w.item, episode); index >= 0 {
		w.selectedEpisode = index
		w.selectedSeason = w.item.seasonIndexForEpisode(index)
		w.episodesPlaced = false
		w.progressEpisode = index
	}
}

// layoutHeaderProgress paints the watched-episode bar under the status row.
// rect ends at the columns below the header, so a short window simply skips it.
// A film has no series counter, so only its progress bar is drawn.
func (w *titleScreen) setWatching(value bool) {
	w.watching = value
	if value {
		w.watchError = ""
	}
}

func (w *titleScreen) setWatchError(message string) {
	w.watching = false
	w.watchError = message
}

func (w *titleScreen) handleKeys(gtx layout.Context) {
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
func (w *titleScreen) stepEpisode(gtx layout.Context, delta int) {
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
			w.selectedSeason = w.item.seasonIndexForEpisode(index)
			w.episodesPlaced = false
			w.ensureEpisodeVisible()
			gtx.Execute(op.InvalidateCmd{})
			return
		}
		index += delta
	}
}

func (w *titleScreen) ensureEpisodeVisible() {
	if w.selectedEpisode < 0 {
		return
	}
	row := w.selectedVisibleRow()
	position := w.episodeList.Position
	if position.Count <= 0 || row < position.First || row >= position.First+position.Count {
		w.episodeList.ScrollTo(row)
	}
}

func (w *titleScreen) ensureVoiceVisible() {
	position := w.voiceList.Position
	if position.Count <= 0 || w.selectedVoice < position.First || w.selectedVoice >= position.First+position.Count {
		w.voiceList.ScrollTo(w.selectedVoice)
	}
}

func (w *titleScreen) stepVoice(gtx layout.Context, delta int) {
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
	w.ensureVoiceVisible()
	if w.selectedEpisode != previousEpisode {
		w.ensureEpisodeVisible()
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (w *titleScreen) layout(gtx layout.Context) {
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

func (w *titleScreen) selectVoice(index int) {
	if index < 0 || index >= len(w.item.voices) {
		return
	}
	changed := index != w.selectedVoice
	w.selectedVoice = index
	if w.episodeMissing(w.selectedEpisodeLabel()) {
		if next := nextAvailableEpisode(w.item, index, w.selectedEpisode); next >= 0 {
			w.selectedEpisode = next
		}
	}
	if changed {
		w.refreshQualityOptions(0)
		if w.onVoice != nil {
			w.onVoice(index)
		}
	}
}

func (w *titleScreen) selectedEpisodeLabel() string {
	if w.selectedEpisode < 0 || w.selectedEpisode >= len(w.item.episodes) {
		return ""
	}
	return w.item.episodes[w.selectedEpisode]
}

// visibleEpisodeIndices lists the global episode indices the grid shows. A
// series with several seasons shows only the selected season; everything else
// shows the whole list so AnimeLIB titles keep their single grid.
func (w *titleScreen) visibleEpisodeIndices() []int {
	if len(w.item.seasons) <= 1 {
		indices := make([]int, len(w.item.episodes))
		for index := range indices {
			indices[index] = index
		}
		return indices
	}
	season := w.selectedSeason
	if season < 0 || season >= len(w.item.seasons) {
		season = 0
	}
	wanted := make(map[string]bool, len(w.item.seasons[season].episodes))
	for _, episode := range w.item.seasons[season].episodes {
		wanted[episode] = true
	}
	indices := make([]int, 0, len(wanted))
	for index, episode := range w.item.episodes {
		if wanted[episode] {
			indices = append(indices, index)
		}
	}
	return indices
}

// selectSeason switches the visible season and moves the selection to its first
// available episode, so pressing Watch after browsing a season plays what the
// grid highlights.
func (w *titleScreen) selectSeason(gtx layout.Context, index int) {
	if index < 0 || index >= len(w.item.seasons) || index == w.selectedSeason {
		return
	}
	w.selectedSeason = index
	if episode := w.firstAvailableInSeason(index); episode >= 0 {
		w.selectedEpisode = episode
	}
	w.episodesPlaced = false
	w.ensureEpisodeVisible()
	gtx.Execute(op.InvalidateCmd{})
}

// firstAvailableInSeason returns the first episode of a season the current voice
// carries, or the first episode at all when the voice misses the opening.
func (w *titleScreen) firstAvailableInSeason(index int) int {
	if index < 0 || index >= len(w.item.seasons) {
		return -1
	}
	first := -1
	for _, key := range w.item.seasons[index].episodes {
		episode := episodeIndex(w.item, key)
		if episode < 0 {
			continue
		}
		if first < 0 {
			first = episode
		}
		if !w.episodeMissing(key) {
			return episode
		}
	}
	return first
}

// selectedVisibleRow is the grid row of the selected episode inside the visible
// season, used to keep it on screen.
func (w *titleScreen) selectedVisibleRow() int {
	if w.episodeColumns <= 0 {
		return 0
	}
	for position, index := range w.visibleEpisodeIndices() {
		if index == w.selectedEpisode {
			return position / w.episodeColumns
		}
	}
	return 0
}

func (w *titleScreen) selectedVoiceName() string {
	if w.selectedVoice < 0 || w.selectedVoice >= len(w.item.voices) {
		return ""
	}
	return w.item.voices[w.selectedVoice].name
}

func (w *titleScreen) episodeMissing(episode string) bool {
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

func (w *titleScreen) layoutTitlePanel(gtx layout.Context, rect image.Rectangle) {
	if rect.Empty() {
		return
	}
	panelStyle{fill: titlePanelFill, border: titlePanelBorder, radius: unit.Dp(10)}.layout(gtx, rect)

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
	seasonsLabel := w.item.seasonsSectionLabel()
	episodesLabel := w.item.episodesSectionLabel(w.selectedSeason)
	if seasonsLabel == "" {
		w.typo.draw(gtx, episodesLabel, labelFace, titlePageSectionSize, mainMutedColor, image.Pt(innerX, labelY))
	} else {
		w.typo.draw(gtx, seasonsLabel, labelFace, titlePageSectionSize, mainMutedColor, image.Pt(innerX, labelY))
	}
	contentTop := labelY + labelInk.Dy() + gtx.Dp(unit.Dp(13))
	episodeTop := contentTop
	qualityLabel := w.qualitySectionLabel()
	_, qualityLabelInk := w.typo.measure(gtx, qualityLabel, labelFace, titlePageSectionSize)
	qualityHeight := gtx.Dp(titlePageQualityHeight)

	// Anchor the quality block to the panel's lower edge. The episode grid
	// then gets all remaining height and clips its last visible row at the
	// block boundary instead of leaving a painted strip above the footer.
	var qualityLabelTop, qualityTop, gridGap int
	if compact {
		qualityHeight = gtx.Dp(unit.Dp(30))
		bottomPad := gtx.Dp(unit.Dp(8))
		labelGap := gtx.Dp(unit.Dp(4))
		gridGap = gtx.Dp(unit.Dp(8))
		qualityLabelTop = rect.Max.Y - bottomPad - qualityHeight - labelGap - qualityLabelInk.Dy()
		qualityTop = qualityLabelTop + qualityLabelInk.Dy() + labelGap
	} else {
		labelGap := gtx.Dp(unit.Dp(18))
		gridGap = gtx.Dp(unit.Dp(18))
		bottomPad := gtx.Dp(unit.Dp(14))
		qualityTop = rect.Max.Y - bottomPad - qualityHeight
		qualityLabelTop = qualityTop - labelGap - qualityLabelInk.Dy()
	}
	gridBottom := qualityLabelTop - gridGap

	if seasonsLabel != "" {
		used := w.layoutSeasons(gtx, image.Rect(innerX, contentTop, innerRight, gridBottom))
		if used > 0 {
			episodeLabelY := contentTop + used + gtx.Dp(unit.Dp(14))
			episodeTop = episodeLabelY + labelInk.Dy() + gtx.Dp(unit.Dp(10))
			if episodeLabelY+labelInk.Dy() < qualityLabelTop {
				w.typo.draw(
					gtx,
					episodesLabel,
					labelFace,
					titlePageSectionSize,
					mainMutedColor,
					image.Pt(innerX, episodeLabelY),
				)
			}
		}
	}
	if gridBottom > episodeTop {
		w.layoutEpisodes(gtx, image.Rect(innerX, episodeTop, innerRight, gridBottom))
	}
	if qualityLabelTop <= episodeTop || qualityLabelTop+qualityLabelInk.Dy() > rect.Max.Y {
		return
	}
	w.typo.draw(
		gtx,
		qualityLabel,
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
}

// layoutMoviePanel leaves only the quality choice for a film. A movie has no
// episode list to choose from, so the block sits at the top of the panel
// instead of being anchored to the bottom above the episode grid.
func (w *titleScreen) layoutMoviePanel(gtx layout.Context, rect image.Rectangle, face font.Font, labelInk image.Rectangle, labelY int, compact bool) {
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
		w.qualitySectionLabel(),
		face,
		titlePageSectionSize,
		mainMutedColor,
		image.Pt(innerX, labelY),
	)
	w.layoutQualities(gtx, image.Rect(innerX, qualityTop, innerRight, qualityTop+qualityHeight))
}

func (w *titleScreen) layoutFooter(gtx layout.Context) {
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
func (w *titleScreen) save(gtx layout.Context) {
	w.saveError = ""
	w.savedUntil = time.Time{}
	if w.onSave != nil {
		if err := w.onSave(w.selectedVoiceName(), w.selectedEpisodeLabel()); err != nil {
			w.saveError = friendlyError(err)
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
func (w *titleScreen) watch(gtx layout.Context) {
	if w.watching || w.item.status == statusAnnounced || w.onWatch == nil {
		return
	}
	w.watchError = ""
	if err := w.onWatch(w.selectedVoiceName(), w.selectedEpisodeLabel(), w.selectedQualityValue()); err != nil {
		w.watchError = friendlyError(err)
		if w.onError != nil {
			w.onError(err)
		}
	} else {
		w.watching = true
	}
	gtx.Execute(op.InvalidateCmd{})
}

func (w *titleScreen) shortcutInfo() shortcutInfo {
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

func (w *titleScreen) layoutHitArea(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, pointerCursor bool, draw func(layout.Context)) {
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

func (w *titleScreen) layoutClickableDraw(gtx layout.Context, rect image.Rectangle, click *widget.Clickable, draw func(layout.Context)) {
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
