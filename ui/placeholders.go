package ui

import (
	"math/rand"
	"time"
	"unicode/utf8"

	"gioui.org/layout"
)

type placeholderPhase uint8

const (
	placeholderIdle placeholderPhase = iota
	placeholderErasing
	placeholderTyping
)

const (
	placeholderIdleDelay  = 5 * time.Second
	placeholderEraseDelay = 22 * time.Millisecond
	placeholderTypeDelay  = 30 * time.Millisecond
	placeholderPause      = 160 * time.Millisecond
)

// placeholderState drives the animated AnimeLIB prompt in the URL field. It
// waits while the editor has text or focus, and only rewrites the prompt while
// the empty input is idle.
type placeholderState struct {
	text          string
	visible       int
	phase         placeholderPhase
	next          time.Time
	idleAt        time.Time
	lastEditorLen int
}

func newPlaceholderState() placeholderState {
	text := randomLinkPlaceholder("")
	return placeholderState{
		text:    text,
		visible: utf8.RuneCountInString(text),
	}
}

// reset returns to a full, immediately visible prompt with a fresh phrase.
func (p *placeholderState) reset() {
	p.text = randomLinkPlaceholder(p.text)
	p.visible = utf8.RuneCountInString(p.text)
	p.phase = placeholderIdle
	p.next = time.Time{}
	p.idleAt = time.Time{}
}

// showFull immediately reveals the current phrase and restarts the idle
// countdown. It is used after interactions that should not look like typing.
func (p *placeholderState) showFull(now time.Time) {
	p.visible = utf8.RuneCountInString(p.text)
	p.phase = placeholderIdle
	p.next = time.Time{}
	p.idleAt = now
}

func (p *placeholderState) visibleText() string {
	if p.visible <= 0 {
		return ""
	}
	runes := []rune(p.text)
	if p.visible >= len(runes) {
		return p.text
	}
	return string(runes[:p.visible])
}

// update advances the typewriter. editorLen is the current editor content
// length, and focused reports whether the URL field owns keyboard focus.
func (p *placeholderState) update(gtx layout.Context, editorLen int, focused bool) {
	if editorLen > 0 {
		p.lastEditorLen = editorLen
		return
	}

	if p.lastEditorLen > 0 {
		// Clearing an entered link returns the full prompt immediately.
		p.lastEditorLen = 0
		p.text = randomLinkPlaceholder(p.text)
		p.visible = utf8.RuneCountInString(p.text)
		p.phase = placeholderIdle
		p.next = time.Time{}
		p.idleAt = time.Time{}
	}

	if p.text == "" {
		p.text = randomLinkPlaceholder("")
		p.visible = utf8.RuneCountInString(p.text)
	}

	if focused {
		// A focused empty field is not idle: keep the prompt readable and stop
		// the animation until focus leaves.
		if p.phase != placeholderIdle || p.visible != utf8.RuneCountInString(p.text) {
			p.showFull(gtx.Now)
		}
		p.idleAt = time.Time{}
		return
	}

	if p.idleAt.IsZero() {
		p.idleAt = gtx.Now
	}

	switch p.phase {
	case placeholderErasing:
		if gtx.Now.Before(p.next) {
			return
		}
		if p.visible > 0 {
			p.visible--
		}
		if p.visible <= 0 {
			p.text = randomLinkPlaceholder(p.text)
			p.phase = placeholderTyping
			p.next = gtx.Now.Add(placeholderPause)
			return
		}
		p.next = gtx.Now.Add(placeholderEraseDelay)

	case placeholderTyping:
		if gtx.Now.Before(p.next) {
			return
		}
		total := utf8.RuneCountInString(p.text)
		if p.visible < total {
			p.visible++
		}
		if p.visible >= total {
			p.phase = placeholderIdle
			p.idleAt = gtx.Now
			p.next = time.Time{}
			return
		}
		p.next = gtx.Now.Add(placeholderTypeDelay)

	default:
		if p.visible != utf8.RuneCountInString(p.text) {
			p.showFull(gtx.Now)
			return
		}
		if gtx.Now.Sub(p.idleAt) >= placeholderIdleDelay {
			p.phase = placeholderErasing
			p.next = gtx.Now
		}
	}
}

// maxPlaceholderRunes keeps every prompt short enough for the URL field: at
// 14sp the editor fits about 78 runes, so 55 still leaves room for flavour
// without the hint being truncated.
const maxPlaceholderRunes = 55

// animeLinkPlaceholders are the anime-flavoured prompts that all ask for an
// AnimeLIB link. Rotation needs at least two entries.
var animeLinkPlaceholders = []string{
	"Скинь ссылку на AnimeLIB, бака!",
	"Накама, дай линк на AnimeLIB!",
	"Оре требует ссылку на AnimeLIB!",
	"Сэмпай, кинь ссылку на AnimeLIB!",
	"Ня, дай ссылку на AnimeLIB!",
	"Даттебайо! Ссылку на AnimeLIB!",
	"Не то чтобы хочу… Но ссылку на AnimeLIB дай!",
	"Банкай: «Ссылка на AnimeLIB»!",
	"Ссылка на AnimeLIB или фанфик!",
	"Без ссылки AnimeLIB — исекай без грузовика!",
	"Богиня сказала: дай ссылку AnimeLIB.",
	"Эрен, дай ссылку на AnimeLIB!",
	"Лайт, запиши: нужна ссылка AnimeLIB.",
	"Гоку, загадай ссылку на AnimeLIB!",
	"Твою ссылку, бака! AnimeLIB!",
	"Не цундэрэ я! Дай ссылку AnimeLIB.",
	"Бака! Где ссылка на AnimeLIB?",
	"Ссылку AnimeLIB или заблокирую!",
	"Сэмпай, скинь ссылку на AnimeLIB, очень прошу!",
	"Оре хочет ссылку на AnimeLIB, и точка.",
	"Даттебайо! Дай ссылку на AnimeLIB, живо!",
	"Накама, кинь линк на AnimeLIB, не тяни!",
	"Банкай: дай ссылку на AnimeLIB, активирую!",
	"Богиня просит ссылку на AnimeLIB, не зли её.",
	"Грузовик-сан, дай ссылку на AnimeLIB, умоляю!",
	"Сакура ждёт ссылку на AnimeLIB, не заставляй.",
	"Эрен, скинь ссылку на AnimeLIB, и без титанов!",
	"Лайт, запиши: ссылка на AnimeLIB нужна сейчас.",
	"Ссылку на AnimeLIB, бака, и побыстрее!",
	"Кавай — это когда есть ссылка на AnimeLIB.",
	"Цундэрэ: дай ссылку на AnimeLIB, ну пожалуйста!",
	"Ганбатте! Ссылку на AnimeLIB, ты сможешь!",
	"Накама, ссылку на AnimeLIB, отряд ждёт!",
	"Стенд: ссылка на AnimeLIB, и враг падёт!",
	"Вставь ссылку на AnimeLIB, бака, не тормози!",
	"Итадакимас! Ссылку на AnimeLIB, приятного!",
	"Онии-тян: дай ссылку на AnimeLIB, будь милым!",
	"Магия — это ссылка на AnimeLIB, вот и весь секрет.",
	"Сэмпай, дай ссылку на AnimeLIB, я верю в тебя.",
	"Суки-суки! Ссылку на AnimeLIB, обожаю!",
	"Ганбатте! Дай ссылку на AnimeLIB, не сдавайся!",
	"Пустота просит ссылку на AnimeLIB, не игнорируй.",
	"Аниме-боги: дайте ссылку на AnimeLIB, умоляю!",
	"Портал: дай ссылку на AnimeLIB, и в другой мир!",
	"Ксо! Ссылку на AnimeLIB, живо, не испытывай!",
	"Не красней: дай ссылку на AnimeLIB, здесь все свои.",
	"Умэ-умэ! Ссылку на AnimeLIB, вкусно же!",
	"Филлер? Дай ссылку на AnimeLIB, а не это!",
	"Хидой! Дай ссылку на AnimeLIB, это же хидой!",
	"Пятеро: дайте ссылку на AnimeLIB, все вместе!",
	"Тикара! Дай ссылку на AnimeLIB, вся сила в ней!",
	"Ссылку на AnimeLIB — и меч цел, и враг бежит!",
	"Не грусти: дай ссылку на AnimeLIB, и всё начнётся.",
	"Загадай: ссылка на AnimeLIB, и желание сбудется.",
	"Финал: дай ссылку на AnimeLIB, арка уже близко.",
	"Тиби: дай ссылку на AnimeLIB, я жду с мороженым!",
	"Отряд: дайте ссылку на AnimeLIB, стройся!",
	"Спелл: ссылка на AnimeLIB, и магия сработает!",
	"Оре-но тайтл: дай ссылку на AnimeLIB, прошу!",
	"Ссылка на AnimeLIB — вот и всё, что нужно.",
	"Синоби, дай ссылку на AnimeLIB, тихо и быстро!",
	"Сэкай: дай ссылку на AnimeLIB, спаси мир!",
	"Опенинг: дай ссылку на AnimeLIB, без пропусков!",
	"Марафон: дай ссылку на AnimeLIB, весь сезон!",
	"Проснись: дай ссылку на AnimeLIB, соня!",
	"Тиби: ссылку на AnimeLIB, ну, не капризничай!",
	"Ссылку на AnimeLIB из буфера, и побыстрее!",
	"Буфер обмена: ссылка на AnimeLIB уже ждёт!",
}

func randomLinkPlaceholder(exclude string) string {
	if len(animeLinkPlaceholders) == 0 {
		return ""
	}
	index := rand.Intn(len(animeLinkPlaceholders))
	if len(animeLinkPlaceholders) > 1 && animeLinkPlaceholders[index] == exclude {
		index = (index + 1) % len(animeLinkPlaceholders)
	}
	return animeLinkPlaceholders[index]
}
