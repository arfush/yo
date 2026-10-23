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

// animeLinkPlaceholders are original anime-flavoured prompts that all ask for
// an AnimeLIB link. Keep this list at 50 entries.
var animeLinkPlaceholders = []string{
	"Ссылку AnimeLIB, и этот мир будет спасён!",
	"Даже герой из исекая просит ссылку AnimeLIB.",
	"Без ссылки AnimeLIB мой сюжет зашёл в тупик.",
	"Кинжал в спину? Лучше ссылку AnimeLIB.",
	"Ссылка AnimeLIB — мой самый сильный артефакт.",
	"Покажи ссылку AnimeLIB, и я поверю в магию.",
	"Сёнэн без ссылки AnimeLIB — не сёнэн.",
	"Ссылку AnimeLIB, или я запущу филлеры!",
	"Мой внутренний демон требует ссылку AnimeLIB.",
	"Ссылка AnimeLIB? Это же уровень босса!",
	"Даже Соник не успеет так быстро, как я жду ссылку AnimeLIB.",
	"Вставь ссылку AnimeLIB, и начнётся опенинг.",
	"Ссылку AnimeLIB — и никаких клиффхэнгеров.",
	"Мой меч жаждет ссылку AnimeLIB.",
	"Без ссылки AnimeLIB я как филлерный эпизод.",
	"Ссылку AnimeLIB, иначе сюжет уйдёт в филлер.",
	"Собери ссылку AnimeLIB, как древний артефакт.",
	"Ссылка AnimeLIB — это мой путь ниндзя.",
	"Вставь ссылку AnimeLIB, и я активирую банкай.",
	"Даже титан ждёт твою ссылку AnimeLIB.",
	"Ссылку AnimeLIB, и я покажу тебе истинную форму.",
	"Без ссылки AnimeLIB мой кунай не долетит.",
	"Ссылка AnimeLIB — билет в другой мир.",
	"Вставь ссылку AnimeLIB, и мы сбежим в аниме.",
	"Мой хвост чувствует приближение ссылки AnimeLIB.",
	"Ссылку AnimeLIB, или я съем твой онигири.",
	"Ня! Дай ссылку AnimeLIB, пожалуйста.",
	"Ссылка AnimeLIB — залог спокойной жизни.",
	"Вставь ссылку AnimeLIB, и я перестану мяукать.",
	"Даже Гоку просит ссылку AnimeLIB для тренировки.",
	"Ссылку AnimeLIB, и мой уровень силы вырастет.",
	"Без ссылки AnimeLIB я не выйду из комнаты.",
	"Ссылка AnimeLIB — мой сценарий на сегодня.",
	"Принеси ссылку AnimeLIB, как герой приносит победу.",
	"Ссылку AnimeLIB, или начнётся арка филлеров.",
	"Мои глаза видят только ссылку AnimeLIB.",
	"Вставь ссылку AnimeLIB, и мы пропустим опенинг.",
	"Ссылка AnimeLIB — ключ от двери в Акацуки.",
	"Без ссылки AnimeLIB даже магия бессильна.",
	"Ссылку AnimeLIB, и я надену свою школьную форму.",
	"Мой сэмпай ждёт ссылку AnimeLIB.",
	"Вставь ссылку AnimeLIB, и я пойду на свидание.",
	"Ссылка AnimeLIB — это не магия, это судьба.",
	"Даже дракон из финала просит ссылку AnimeLIB.",
	"Ссылку AnimeLIB, иначе я стану злодеем.",
	"Без ссылки AnimeLIB мой опенинг не запустится.",
	"Ссылка AnimeLIB — мой последний козырь.",
	"Вставь ссылку AnimeLIB, и мы спасём этот сезон.",
	"Ссылку AnimeLIB, и я исполню твоё желание.",
	"Моя катана ищет ссылку AnimeLIB.",
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
