package shell

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/ui/appearance"
)

// Home geometry, in logical pixels relative to the content rectangle.
const (
	homePadX            = 15
	homeFieldTop        = 38
	homeFieldHeight     = 48
	homeHistoryCenter   = 115
	homeChipsTop        = 102
	homeChipHeight      = 29
	homeChipRadius      = 6
	homeListTop         = 147
	homeCardHeight      = 73
	homeCardGap         = 5
	homeCardRadius      = 8
	homePosterX         = 37
	homePosterY         = 10
	homePosterW         = 40
	homePosterH         = 54
	homeTitleX          = 90
	homeActionSize      = 31
	homeActionGap       = 7
	homeActionInset     = 13
	homePillHeight      = 28
	homeFooterButtonW   = 28
	homeFooterButtonH   = 27
	homeFooterButtonGap = 8
	homeFooterInset     = 24
)

var homeFilters = []string{"Все", "Смотрю", "В планах", "Любимые", "Просмотрено", "Брошено"}

var filterStatuses = []titleStatus{
	statusWatching,
	statusPlanned,
	statusFavorite,
	statusCompleted,
	statusDropped,
}

type titleStatus uint8

const (
	statusPlanned titleStatus = iota
	statusWatching
	statusFavorite
	statusCompleted
	statusDropped
)

type titleItem struct {
	title    string
	status   titleStatus
	poster   int
	progress int
	total    int
	note     string
	active   bool
}

type homeTargetKind uint8

const (
	homeNone homeTargetKind = iota
	homeChip
	homeOpen
	homeClear
	homeCard
	homeHandle
	homePlay
	homeTrash
	homeFooter
)

type homeTarget struct {
	kind  homeTargetKind
	index int
}

// Home is the home screen: the link field, history filters and the scrollable
// title list with drag reordering. A dragged card floats over the content area
// and an accent block marks the slot it will land in.
type Home struct {
	fonts  *fonts
	poster *qt.QImage

	items  []titleItem
	filter int

	hover homeTarget
	press homeTarget

	dragging bool
	dragFrom int
	dragTo   int
	dragGrab image.Point
	dragPos  image.Point

	scroll    int
	maxScroll int

	content image.Rectangle
	footer  image.Rectangle
}

// NewHome builds the home screen with the test library.
func NewHome() *Home {
	return &Home{
		fonts:  loadFonts(),
		poster: loadPoster(),
		items: []titleItem{
			{title: "Восхождение в тени: Потерянное Эхо", status: statusPlanned, poster: 0, note: "24 серии"},
			{title: "Дюрарара!!", status: statusWatching, poster: 1, progress: 8, total: 24, active: true},
			{title: "Забота об одарённой девушке…", status: statusFavorite, poster: 2, progress: 12, total: 12},
			{title: "Монстрик Карамелька", status: statusCompleted, poster: 3, progress: 12, total: 12},
			{title: "Атака титанов", status: statusDropped, poster: 4, progress: 5, total: 25},
			{title: "Магическая битва", status: statusWatching, poster: 5, progress: 3, total: 24},
			{title: "Ван-Пис", status: statusPlanned, poster: 6, note: "1085 серий"},
		},
		dragFrom: -1,
		dragTo:   -1,
	}
}

// homeLayout holds every measurement of one frame.
type homeLayout struct {
	content      image.Rectangle
	footer       image.Rectangle
	sectionY     int
	field        image.Rectangle
	linkIcon     image.Rectangle
	placeholderX int
	hint         image.Rectangle
	open         image.Rectangle
	historyY     int
	countX       int
	clear        image.Rectangle
	chips        []image.Rectangle
	list         image.Rectangle
}

// Paint draws the home content.
func (h *Home) Paint(p *qt.QPainter, content image.Rectangle, palette appearance.Palette) {
	h.content = content
	l := h.computeLayout()
	if l.content.Empty() {
		return
	}
	h.paintField(p, l, palette)
	h.paintHistoryHeader(p, l, palette)
	h.paintChips(p, l, palette)
	h.paintList(p, l, palette)
	h.paintDragged(p, l, palette)
}

// PaintFooter draws the three footer buttons.
func (h *Home) PaintFooter(p *qt.QPainter, footer image.Rectangle, palette appearance.Palette) {
	h.footer = footer
	l := h.computeLayout()
	for i, r := range h.footerButtons(l) {
		body, border := palette.SurfaceAlt, palette.Border
		switch {
		case h.press == (homeTarget{homeFooter, i}):
			body = palette.Panel
		case h.hover == (homeTarget{homeFooter, i}):
			body = palette.Button
		}
		fillRound(p, r, 6, border)
		fillRound(p, r.Inset(1), 5, body)
		icon := image.Rect(r.Min.X+(r.Dx()-14)/2, r.Min.Y+(r.Dy()-14)/2,
			r.Min.X+(r.Dx()-14)/2+14, r.Min.Y+(r.Dy()-14)/2+14)
		drawSVGIcon(p, footerIcon(i), icon, palette.ControlIcon)
	}
}

// Pointer handles pointer events forwarded by the shell.
func (h *Home) Pointer(event PointerEvent) bool {
	switch event.Action {
	case PointerMove:
		if h.dragging {
			h.dragPos = event.Pos
			h.updateDrag(event.Pos)
			return true
		}
		target := h.hitAt(event.Pos)
		if target != h.hover {
			h.hover = target
			return true
		}
		return false
	case PointerDown:
		target := h.hitAt(event.Pos)
		h.press = target
		h.hover = target
		if target.kind == homeHandle {
			l := h.computeLayout()
			h.dragging = true
			h.dragFrom = target.index
			h.dragTo = target.index
			h.dragGrab = event.Pos.Sub(h.cardRect(l, target.index).Min)
			h.dragPos = event.Pos
		}
		return true
	case PointerUp:
		if h.dragging {
			h.commitDrag()
			h.dragging = false
			h.press = homeTarget{}
			h.hover = h.hitAt(event.Pos)
			return true
		}
		target := h.hitAt(event.Pos)
		changed := h.press != (homeTarget{}) || h.hover != target
		if h.press.kind == homeChip && target == h.press {
			h.selectFilter(target.index)
		}
		h.press = homeTarget{}
		h.hover = target
		return changed
	case PointerLeave:
		changed := h.hover != (homeTarget{}) || h.press != (homeTarget{})
		h.hover = homeTarget{}
		h.press = homeTarget{}
		return changed
	}
	return false
}

// Wheel scrolls the history list.
func (h *Home) Wheel(event WheelEvent) bool {
	l := h.computeLayout()
	if !event.Pos.In(l.list) {
		return false
	}
	h.clampScroll(l)
	if h.maxScroll == 0 {
		return false
	}
	h.scroll -= event.Delta * 40
	h.clampScroll(l)
	return true
}

// Cursor returns the cursor shape over the home screen.
func (h *Home) Cursor(pos image.Point) qt.CursorShape {
	if h.dragging {
		return qt.ClosedHandCursor
	}
	switch h.hitAt(pos).kind {
	case homeHandle:
		return qt.OpenHandCursor
	case homeChip, homeOpen, homeClear, homePlay, homeTrash, homeFooter:
		return qt.PointingHandCursor
	}
	return qt.ArrowCursor
}

// computeLayout measures one frame.
func (h *Home) computeLayout() homeLayout {
	c := h.content
	l := homeLayout{content: c, footer: h.footer}
	if c.Empty() {
		return l
	}
	if l.footer.Empty() {
		l.footer = image.Rect(c.Min.X, c.Max.Y+1, c.Max.X, c.Max.Y+1+footerHeight)
	}
	left := c.Min.X + homePadX
	right := c.Max.X - homePadX

	l.sectionY = c.Min.Y + 22
	l.field = image.Rect(left, c.Min.Y+homeFieldTop, right, c.Min.Y+homeFieldTop+homeFieldHeight)
	l.linkIcon = image.Rect(l.field.Min.X+14, l.field.Min.Y+16, l.field.Min.X+30, l.field.Min.Y+32)
	l.placeholderX = l.field.Min.X + 43

	openW := 106
	if l.field.Dx() < 280 {
		openW = max(72, l.field.Dx()/2)
	}
	l.open = image.Rect(l.field.Max.X-6-openW, l.field.Min.Y+6, l.field.Max.X-6, l.field.Min.Y+6+36)
	hintW := measureText(h.fonts.at(10, qt.QFont__Normal), "Ctrl + K")
	hintX := l.open.Min.X - 12 - hintW
	l.hint = image.Rect(hintX, l.field.Min.Y+18, hintX+hintW, l.field.Min.Y+30)

	l.historyY = c.Min.Y + homeHistoryCenter
	l.countX = left + measureText(h.fonts.at(16, qt.QFont__DemiBold), "История") + 10
	clearW := measureText(h.fonts.at(11, qt.QFont__Normal), "Очистить всё")
	l.clear = image.Rect(right-clearW, l.historyY-10, right, l.historyY+10)
	l.chips = h.chipLayout(c, l)
	l.list = image.Rect(left, c.Min.Y+homeListTop, right, c.Max.Y-8)
	return l
}

// chipLayout centers the filter row between the history count and the clear
// action, compressing padding and gaps when the window is narrow.
func (h *Home) chipLayout(c image.Rectangle, l homeLayout) []image.Rectangle {
	font := h.fonts.at(11, qt.QFont__Medium)
	widths := make([]int, len(homeFilters))
	for i, label := range homeFilters {
		widths[i] = measureText(font, label)
	}
	count := measureText(h.fonts.at(11.8, qt.QFont__Normal), strconv.Itoa(len(h.visible())))
	left := l.countX + count + 20
	right := l.clear.Min.X - 16
	if right < left {
		right = left
	}

	padding, gap := 12, 7
	total := func() int {
		sum := gap * (len(widths) - 1)
		for _, w := range widths {
			sum += w + 2*padding
		}
		return sum
	}
	for total() > right-left && padding > 6 {
		padding--
		if gap > 3 {
			gap--
		}
	}

	start := left + (right-left-total())/2
	if start < left {
		start = left
	}
	rects := make([]image.Rectangle, len(widths))
	x := start
	for i, w := range widths {
		chipW := w + 2*padding
		rects[i] = image.Rect(x, c.Min.Y+homeChipsTop, x+chipW, c.Min.Y+homeChipsTop+homeChipHeight)
		x += chipW + gap
	}
	return rects
}

// visible returns the item indices that pass the active filter.
func (h *Home) visible() []int {
	if h.filter <= 0 || h.filter > len(filterStatuses) {
		indices := make([]int, len(h.items))
		for i := range h.items {
			indices[i] = i
		}
		return indices
	}
	want := filterStatuses[h.filter-1]
	indices := make([]int, 0, len(h.items))
	for i, item := range h.items {
		if item.status == want {
			indices = append(indices, i)
		}
	}
	return indices
}

// selectFilter changes the active chip and resets the scroll.
func (h *Home) selectFilter(index int) {
	if index < 0 || index >= len(homeFilters) || index == h.filter {
		return
	}
	h.filter = index
	h.scroll = 0
	h.hover = homeTarget{}
	h.press = homeTarget{}
	h.dragging = false
}

// clampScroll keeps the list offset inside its range and records the maximum.
func (h *Home) clampScroll(l homeLayout) {
	count := len(h.visible())
	contentH := count*homeCardHeight + max(0, count-1)*homeCardGap
	h.maxScroll = max(0, contentH-l.list.Dy())
	if h.scroll < 0 {
		h.scroll = 0
	}
	if h.scroll > h.maxScroll {
		h.scroll = h.maxScroll
	}
}

// updateDrag recomputes the drop slot and auto-scrolls near the list edges.
func (h *Home) updateDrag(pos image.Point) {
	l := h.computeLayout()
	if l.list.Empty() {
		return
	}
	const edge = 24
	switch {
	case pos.Y < l.list.Min.Y+edge:
		h.scroll -= 6
	case pos.Y > l.list.Max.Y-edge:
		h.scroll += 6
	}
	h.clampScroll(l)

	count := len(h.visible())
	if count == 0 {
		return
	}
	card := h.draggedCardRect(l)
	center := card.Min.Y + homeCardHeight/2
	index := int(math.Round(float64(center-l.list.Min.Y+h.scroll) / float64(homeCardHeight+homeCardGap)))
	if index < 0 {
		index = 0
	}
	if index >= count {
		index = count - 1
	}
	h.dragTo = index
}

// draggedCardRect returns the free-floating card position inside the content
// area. The card follows the pointer but never leaves the content rectangle.
func (h *Home) draggedCardRect(l homeLayout) image.Rectangle {
	if l.content.Empty() || l.list.Empty() {
		return image.Rectangle{}
	}
	width := l.list.Dx()
	x := h.dragPos.X - h.dragGrab.X
	y := h.dragPos.Y - h.dragGrab.Y
	x = max(l.content.Min.X, min(x, l.content.Max.X-width))
	y = max(l.content.Min.Y, min(y, l.content.Max.Y-homeCardHeight))
	return image.Rect(x, y, x+width, y+homeCardHeight)
}

// commitDrag moves the dragged item to the drop slot.
func (h *Home) commitDrag() {
	vis := h.visible()
	if h.dragFrom < 0 || h.dragFrom >= len(vis) || h.dragTo < 0 || h.dragTo >= len(vis) || h.dragFrom == h.dragTo {
		return
	}
	h.moveItem(vis[h.dragFrom], vis[h.dragTo])
}

// moveItem moves one item in the full order.
func (h *Home) moveItem(from, to int) {
	if from == to || from < 0 || from >= len(h.items) || to < 0 || to >= len(h.items) {
		return
	}
	item := h.items[from]
	h.items = append(h.items[:from], h.items[from+1:]...)
	h.items = append(h.items[:to], append([]titleItem{item}, h.items[to:]...)...)
}

// hitAt resolves the interactive target under a point.
func (h *Home) hitAt(pos image.Point) homeTarget {
	l := h.computeLayout()
	if l.content.Empty() {
		return homeTarget{}
	}
	if pos.In(l.open) {
		return homeTarget{homeOpen, 0}
	}
	if pos.In(l.clear) {
		return homeTarget{homeClear, 0}
	}
	for i, chip := range l.chips {
		if pos.In(chip) {
			return homeTarget{homeChip, i}
		}
	}
	for i, button := range h.footerButtons(l) {
		if pos.In(button) {
			return homeTarget{homeFooter, i}
		}
	}
	if pos.In(l.list) {
		for i := range h.visible() {
			card := h.cardRect(l, i)
			if !pos.In(card) {
				continue
			}
			if pos.In(h.handleRect(card)) {
				return homeTarget{homeHandle, i}
			}
			play, trash := h.actionRects(card)
			switch {
			case pos.In(play):
				return homeTarget{homePlay, i}
			case pos.In(trash):
				return homeTarget{homeTrash, i}
			}
			return homeTarget{homeCard, i}
		}
	}
	return homeTarget{}
}

// cardRect returns the visible rectangle of one card.
func (h *Home) cardRect(l homeLayout, index int) image.Rectangle {
	top := l.list.Min.Y + index*(homeCardHeight+homeCardGap) - h.scroll
	return image.Rect(l.list.Min.X, top, l.list.Max.X, top+homeCardHeight)
}

// handleRect is the drag handle hitbox.
func (h *Home) handleRect(card image.Rectangle) image.Rectangle {
	return image.Rect(card.Min.X+10, card.Min.Y+22, card.Min.X+28, card.Min.Y+52)
}

// actionRects returns the play and trash hitboxes of one card.
func (h *Home) actionRects(card image.Rectangle) (play, trash image.Rectangle) {
	trash = image.Rect(card.Max.X-homeActionInset-homeActionSize, card.Min.Y+21,
		card.Max.X-homeActionInset, card.Min.Y+21+homeActionSize)
	play = trash.Add(image.Pt(-(homeActionSize + homeActionGap), 0))
	return play, trash
}

// footerButtons returns the three footer buttons, left to right.
func (h *Home) footerButtons(l homeLayout) [3]image.Rectangle {
	var buttons [3]image.Rectangle
	if l.footer.Empty() {
		return buttons
	}
	right := l.footer.Max.X - homeFooterInset
	top := l.footer.Min.Y + 10
	for i := 0; i < 3; i++ {
		x := right - (3-i)*homeFooterButtonW - (2-i)*homeFooterButtonGap
		buttons[i] = image.Rect(x, top, x+homeFooterButtonW, top+homeFooterButtonH)
	}
	return buttons
}

// footerIcon names the footer glyphs left to right.
func footerIcon(index int) string {
	switch index {
	case 0:
		return "refresh"
	case 1:
		return "user"
	default:
		return "filters"
	}
}

// paintField draws the section heading and the link field.
func (h *Home) paintField(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	drawTextAt(p, h.fonts.at(12, qt.QFont__DemiBold), palette.Text, l.field.Min.X, l.sectionY, "Открыть тайтл по ссылке")

	fillRound(p, l.field, 8, palette.Border)
	fillRound(p, l.field.Inset(1), 7, palette.Panel)
	drawSVGIcon(p, "link", l.linkIcon, palette.TextMuted)

	placeholderFont := h.fonts.at(12.9, qt.QFont__Normal)
	maxWidth := l.open.Min.X - l.placeholderX - 12
	placeholder := elide(placeholderFont, "Вставьте ссылку на AnimeLIB", float64(maxWidth))
	drawTextAt(p, placeholderFont, palette.TextMuted, l.placeholderX, l.field.Min.Y+l.field.Dy()/2, placeholder)

	hintFont := h.fonts.at(10, qt.QFont__Normal)
	if l.hint.Min.X > l.placeholderX+120 {
		drawTextAt(p, hintFont, palette.TextFaint, l.hint.Min.X, l.field.Min.Y+l.field.Dy()/2, "Ctrl + K")
	}

	body := palette.Accent
	switch {
	case h.press == (homeTarget{homeOpen, 0}):
		body = mixColor(palette.Accent, palette.Canvas, 0.18)
	case h.hover == (homeTarget{homeOpen, 0}):
		body = mixColor(palette.Accent, palette.White, 0.10)
	}
	fillRound(p, l.open, 8, body)
	labelFont := h.fonts.at(12.6, qt.QFont__Medium)
	label := "Открыть"
	labelW := measureText(labelFont, label)
	contentW := labelW + 6 + 11
	x := l.open.Min.X + (l.open.Dx()-contentW)/2
	centerY := l.open.Min.Y + l.open.Dy()/2
	drawTextAt(p, labelFont, palette.OnAccent, x, centerY, label)
	arrow := image.Rect(x+labelW+6, centerY-5, x+labelW+6+11, centerY+6)
	drawSVGIcon(p, "arrow", arrow, palette.OnAccent)
}

// paintHistoryHeader draws the heading, count and clear action.
func (h *Home) paintHistoryHeader(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	drawTextAt(p, h.fonts.at(16, qt.QFont__DemiBold), palette.Text, l.field.Min.X, l.historyY, "История")
	count := strconv.Itoa(len(h.visible()))
	drawTextAt(p, h.fonts.at(11.8, qt.QFont__Normal), palette.TextFaint, l.countX, l.historyY, count)

	clearColor := palette.TextFaint
	switch {
	case h.press == (homeTarget{homeClear, 0}):
		clearColor = palette.Text
	case h.hover == (homeTarget{homeClear, 0}):
		clearColor = palette.TextMuted
	}
	drawTextAt(p, h.fonts.at(11, qt.QFont__Normal), clearColor, l.clear.Min.X, l.historyY, "Очистить всё")
}

// paintChips draws the status filter row.
func (h *Home) paintChips(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	font := h.fonts.at(11, qt.QFont__Medium)
	for i, chip := range l.chips {
		body, border, text := palette.Panel, palette.Border, palette.TextMuted
		switch {
		case i == h.filter:
			body, border, text = palette.Badge, palette.Ink, palette.Ink
		case h.press == (homeTarget{homeChip, i}):
			body, border, text = mixColor(palette.Panel, palette.Canvas, 0.12), palette.BorderStrong, palette.Text
		case h.hover == (homeTarget{homeChip, i}):
			body, border, text = mixColor(palette.Panel, palette.Text, 0.06), palette.BorderStrong, palette.Text
		}
		fillRound(p, chip, homeChipRadius, border)
		fillRound(p, chip.Inset(1), homeChipRadius-1, body)
		label := homeFilters[i]
		labelW := measureText(font, label)
		drawTextAt(p, font, text, chip.Min.X+(chip.Dx()-labelW)/2, chip.Min.Y+chip.Dy()/2, label)
	}
}

// paintList draws the scrollable history list.
func (h *Home) paintList(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	if l.list.Empty() {
		return
	}
	h.clampScroll(l)
	vis := h.visible()
	if len(vis) == 0 {
		font := h.fonts.at(12, qt.QFont__Normal)
		label := "Ничего не найдено"
		width := measureText(font, label)
		drawTextAt(p, font, palette.TextMuted, l.list.Min.X+(l.list.Dx()-width)/2, l.list.Min.Y+l.list.Dy()/2, label)
		return
	}

	p.Save()
	p.SetClipRect2(l.list.Min.X, l.list.Min.Y, l.list.Dx(), l.list.Dy())
	if h.dragging {
		h.paintDropBlock(p, l, palette)
	}
	for i := range vis {
		if h.dragging && (i == h.dragFrom || i == h.dragTo) {
			continue
		}
		card := h.cardRect(l, i)
		if card.Max.Y < l.list.Min.Y || card.Min.Y > l.list.Max.Y {
			continue
		}
		h.paintCard(p, card, h.items[vis[i]], i, palette)
	}
	p.Restore()
}

// paintDragged draws the lifted card above the list. It follows the pointer
// anywhere inside the content area.
func (h *Home) paintDragged(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	if !h.dragging {
		return
	}
	vis := h.visible()
	if h.dragFrom < 0 || h.dragFrom >= len(vis) {
		return
	}
	card := h.draggedCardRect(l)
	if card.Empty() {
		return
	}
	h.paintCard(p, card, h.items[vis[h.dragFrom]], h.dragFrom, palette)
}

// paintDropBlock marks the slot the dragged card will land in with an accent
// block the size of a card.
func (h *Home) paintDropBlock(p *qt.QPainter, l homeLayout, palette appearance.Palette) {
	card := h.cardRect(l, h.dragTo)
	if card.Empty() {
		return
	}
	body := mixColor(palette.Panel, palette.Accent, 0.22)
	fillRound(p, card, homeCardRadius, palette.Accent)
	fillRound(p, card.Inset(2), homeCardRadius-1, body)
}

// paintCard draws one title row.
func (h *Home) paintCard(p *qt.QPainter, card image.Rectangle, item titleItem, index int, palette appearance.Palette) {
	lifted := h.dragging && index == h.dragFrom
	hovered := h.hover == (homeTarget{homeCard, index})
	body, border := palette.Panel, palette.Border
	switch {
	case lifted:
		body, border = palette.Selection, palette.Ink
	case item.active:
		body, border = palette.Selection, palette.Edge
	case hovered:
		border = palette.BorderStrong
	}
	fillRound(p, card, homeCardRadius, border)
	fillRound(p, card.Inset(1), homeCardRadius-1, body)

	poster := image.Rect(card.Min.X+homePosterX, card.Min.Y+homePosterY,
		card.Min.X+homePosterX+homePosterW, card.Min.Y+homePosterY+homePosterH)
	if h.poster != nil {
		clip := roundedPath(poster, 4)
		p.Save()
		p.SetClipPath(clip)
		src := qt.NewQRect4(item.poster*homePosterW, 0, homePosterW, homePosterH)
		dst := qt.NewQRect4(poster.Min.X, poster.Min.Y, poster.Dx(), poster.Dy())
		p.DrawImage2(dst, h.poster, src)
		src.Delete()
		dst.Delete()
		p.Restore()
		clip.Delete()
	} else {
		fillRound(p, poster, 4, palette.Tile)
	}

	handleColor := palette.TextFaint
	if h.hover == (homeTarget{homeHandle, index}) || lifted {
		handleColor = palette.Ink
	}
	for _, dx := range []float64{17.5, 20.5} {
		for _, dy := range []float64{34.5, 37, 39.5} {
			fillEllipse(p, float64(card.Min.X)+dx, float64(card.Min.Y)+dy, 0.9, handleColor)
		}
	}

	play, trash := h.actionRects(card)
	statusRight := play.Min.X - 21
	titleX := card.Min.X + homeTitleX
	titleMax := statusRight - titleX - 12
	if titleMax < 40 {
		titleMax = 40
	}
	titleFont := h.fonts.at(14, qt.QFont__DemiBold)
	drawTextAt(p, titleFont, palette.Text, titleX, card.Min.Y+27,
		elide(titleFont, item.title, float64(titleMax)))

	subtitle := item.note
	if item.total > 0 {
		subtitle = fmt.Sprintf("%d из %d серий", item.progress, item.total)
	}
	drawTextAt(p, h.fonts.at(11, qt.QFont__Normal), palette.TextMuted, titleX, card.Min.Y+44, subtitle)

	if item.total > 0 {
		barW := min(230, statusRight-titleX-80)
		if barW > 60 {
			bar := image.Rect(titleX, card.Min.Y+57, titleX+barW, card.Min.Y+60)
			fillRound(p, bar, 1.5, palette.SurfaceAlt)
			filled := int(float64(barW) * float64(item.progress) / float64(item.total))
			if filled > 0 {
				fillRound(p, image.Rect(bar.Min.X, bar.Min.Y, bar.Min.X+filled, bar.Max.Y), 1.5, palette.Accent)
			}
			progress := fmt.Sprintf("%d / %d", item.progress, item.total)
			drawTextAt(p, h.fonts.at(10, qt.QFont__Normal), palette.TextFaint,
				bar.Max.X+13, card.Min.Y+58, progress)
		}
	}

	status := statusColors(item.status, palette)
	label := statusLabel(item.status)
	pillW := measureText(h.fonts.at(11, qt.QFont__Medium), label) + 24
	pill := image.Rect(statusRight-pillW, card.Min.Y+23, statusRight, card.Min.Y+23+homePillHeight)
	fillRound(p, pill, 6, status.Border)
	fillRound(p, pill.Inset(1), 5, status.Background)
	drawTextAt(p, h.fonts.at(11, qt.QFont__Medium), status.Text, pill.Min.X+12, pill.Min.Y+pill.Dy()/2, label)

	h.paintAction(p, play, "play", palette, homeTarget{homePlay, index}, palette.ActionIcon)
	h.paintAction(p, trash, "trash", palette, homeTarget{homeTrash, index}, palette.DeleteIcon)
}

// paintAction draws one play or trash button.
func (h *Home) paintAction(p *qt.QPainter, r image.Rectangle, name string, palette appearance.Palette, target homeTarget, icon color.NRGBA) {
	body, border := palette.Button, palette.ButtonBorder
	switch {
	case h.press == target:
		body, border = palette.Panel, palette.BorderStrong
	case h.hover == target:
		body = mixColor(palette.Button, palette.Text, 0.08)
	}
	fillRound(p, r, 6, border)
	fillRound(p, r.Inset(1), 5, body)
	glyph := image.Rect(r.Min.X+8, r.Min.Y+8, r.Max.X-8, r.Max.Y-8)
	drawSVGIcon(p, name, glyph, icon)
}

// statusLabel returns the chip label of a status.
func statusLabel(status titleStatus) string {
	switch status {
	case statusWatching:
		return "Смотрю"
	case statusFavorite:
		return "Любимые"
	case statusCompleted:
		return "Просмотрено"
	case statusDropped:
		return "Брошено"
	default:
		return "В планах"
	}
}

// statusColors returns the pill colors of a status.
func statusColors(status titleStatus, palette appearance.Palette) appearance.StatusColors {
	switch status {
	case statusWatching:
		return palette.Watching
	case statusFavorite:
		return palette.Favorite
	case statusDropped:
		return palette.Dropped
	case statusCompleted:
		return appearance.StatusColors{
			Text:       palette.Success,
			Background: mixColor(palette.Success, palette.Canvas, 0.82),
			Border:     mixColor(palette.Success, palette.Canvas, 0.55),
		}
	default:
		return palette.Planned
	}
}

// mixColor blends two palette colors.
func mixColor(a, b color.NRGBA, t float32) color.NRGBA {
	blend := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return color.NRGBA{R: blend(a.R, b.R), G: blend(a.G, b.G), B: blend(a.B, b.B), A: blend(a.A, b.A)}
}

// elide shortens a line to width with an ellipsis.
func elide(font *qt.QFont, text string, width float64) string {
	if width <= 0 {
		return ""
	}
	fm := qt.NewQFontMetricsF(font)
	elided := fm.ElidedText(text, qt.ElideRight, width)
	fm.Delete()
	return elided
}

// fillEllipse fills one circle.
func fillEllipse(p *qt.QPainter, cx, cy, radius float64, col color.NRGBA) {
	brush := qt.NewQBrush3(qcolor(col))
	p.SetPenWithStyle(qt.NoPen)
	p.SetBrush(brush)
	center := qt.NewQPointF3(cx, cy)
	p.DrawEllipse3(center, radius, radius)
	center.Delete()
	brush.Delete()
}
