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

// animeLinkPlaceholders are the anime-flavoured prompts that all ask for an
// AnimeLIB link. Rotation needs at least two entries.
var animeLinkPlaceholders = []string{
	"Скинь ссылку на AnimeLIB, бака!",
	"Накама, дай линк на AnimeLIB!",
	"Оре требует ссылку на AnimeLIB!",
	"Сэмпай, кинь ссылку на AnimeLIB!",
	"Ня, дай ссылку на AnimeLIB!",
	"Даттебайо! Ссылку на AnimeLIB!",
	"Не то чтобы хочу… Но сейчас бы ссылку AnimeLIB…",
	"Банкай: «Ссылка на AnimeLIB»!",
	"Ссылка на AnimeLIB или фанфик!",
	"Без ссылки AnimeLIB — как исекай без грузовик-сана!",
	"Богиня сказала: дай ссылку AnimeLIB.",
	"Эрен, дай ссылку на AnimeLIB!",
	"Лайт, запиши: нужна ссылка AnimeLIB.",
	"Гоку, загадай ссылку на AnimeLIB!",
	"Твою ссылку, бака! AnimeLIB!",
	"Не цундэрэ я! Дай ссылку AnimeLIB.",
	"Бака! Где ссылка на AnimeLIB?",
	"Ссылку AnimeLIB или заблокирую!",
	"Сэмпай, не заставляй меня ждать — вставь в это поле ссылку на тайтл с AnimeLIB, и посмотрим серию.",
	"Оре-татимаэ!.. Ладно, просто вставь ссылку на тайтл с AnimeLIB в поле, и я прощу тебя.",
	"Даже Гоку не найдёт нужный тайтл, пока ты не вставишь ссылку с AnimeLIB в это поле.",
	"Богиня интернета требует: вставь в поле ссылку на тайтл с AnimeLIB, иначе уйдёт спать.",
	"Даттебайо! Пока ты думаешь, серия уже вышла — тащи в поле ссылку на тайтл с AnimeLIB.",
	"Банкай: «Ссылка на тайтл с AnimeLIB в поле»! Активирую всю свою лень ради одного Ctrl+V.",
	"Не то чтобы я так уж хотела… но без ссылки на тайтл с AnimeLIB в этом поле вечер пропал.",
	"Грузовик-сан уже проехал, а в поле всё нет ссылки на тайтл с AnimeLIB — исекай отменяется.",
	"Сакура опадает, а поле пустое: вставь в него ссылку на тайтл с AnimeLIB, пока не поздно.",
	"Эрен, ты решил уничтожить всё, или просто забыл вставить в поле ссылку на тайтл с AnimeLIB?",
	"Лайт, запиши в тетрадь: «Сегодня я вставил в поле ссылку на тайтл с AnimeLIB».",
	"Ссылка на тайтл с AnimeLIB в этом поле или тысяча лет одиночества в пустом списке серий.",
	"Кавайность строки растёт, когда в неё вставляют ссылку на тайтл с AnimeLIB, а не что попало.",
	"Цундэрэ-строке тоже нужно внимание: вставь в неё ссылку на тайтл с AnimeLIB, и она оттает.",
	"Сэмпай, я верю: один Ctrl+V — и в поле уже красуется ссылка на тайтл с AnimeLIB.",
	"Накама, приказ капитана: вставь в поле ссылку на тайтл с AnimeLIB, и никаких исключений!",
	"Твой стенд слаб против пустого поля — вставь ссылку на тайтл с AnimeLIB и покажи силу.",
	"Если в поле не появится ссылка на тайтл с AnimeLIB, я буду дуться целую арку.",
	"Итадакимас! Но сначала подай в поле ссылку на тайтл с AnimeLIB, а потом уже серии.",
	"Онии-тян не любит пустые поля: накорми строку ссылкой на тайтл с AnimeLIB.",
	"В этом мире магии побеждает не заклинание, а вставленная в поле ссылка на тайтл с AnimeLIB.",
	"Сэмпай, пока ты ищешь мотивацию, я ищу тайтл — вставь в поле ссылку на тайтл с AnimeLIB.",
	"Суки-суки! Ссылка на тайтл с AnimeLIB в этом поле нравится мне больше всего интернета.",
	"Ганбатте! Достань из закладок ссылку на тайтл с AnimeLIB и вставь её в это поле.",
	"Пустое поле — не арт-хаус, а повод наконец вставить в него ссылку на тайтл с AnimeLIB.",
	"Аниме-боги смотрят свысока и ждут, когда ты вставишь в поле ссылку на тайтл с AnimeLIB.",
	"Ты можешь открыть портал в другой мир или вставить в поле ссылку на тайтл с AnimeLIB — второе проще.",
	"Ксо! Ещё немного, и я сама вставлю в это поле ссылку на тайтл с AnimeLIB за тебя.",
	"Сэмпай, не заставляй её краснеть — вставь в поле ссылку на тайтл с AnimeLIB прямо сейчас.",
	"Умэ-умэ! Никакой магии не нужно: просто вставь в поле ссылку на тайтл с AnimeLIB.",
	"Даже самый длинный филлер кончится раньше, чем ты вставишь в поле ссылку на тайтл с AnimeLIB.",
	"Хидой! Пустое поле снова напоминает: вставь ссылку на тайтл с AnimeLIB, а не страдай.",
	"Пять героев не справятся с пустым полем, а ссылка на тайтл с AnimeLIB в нём — легко.",
	"Оре-но тикара! Вся моя сила уходит на просьбу вставить в поле ссылку на тайтл с AnimeLIB.",
	"Без ссылки на тайтл с AnimeLIB в поле я как меч без рукояти — вроде есть, а толку ноль.",
	"Сэмпай, ты же не хочешь, чтобы поле осталось пустым вместо ссылки на тайтл с AnimeLIB?",
	"Загадай желание не звёздам, а этому полю — пусть в нём появится ссылка на тайтл с AnimeLIB.",
	"Финал арки ближе, чем кажется, а ссылки на тайтл с AnimeLIB в поле всё ещё нет.",
	"Тиби-версия этой строки плачет: вставь в неё ссылку на тайтл с AnimeLIB, и она улыбнётся.",
	"Накама, отряд собран, припасы готовы — не хватает только ссылки на тайтл с AnimeLIB в поле.",
	"Ты не поверишь, но сильнейшее заклинание здесь — вставленная в поле ссылка на тайтл с AnimeLIB.",
	"Оре-но тайтл! Вставь в поле ссылку на тайтл с AnimeLIB, и я поделюсь вечностью серий.",
	"Эта строка создана не для пустоты, а для ссылки на тайтл с AnimeLIB, вставленной в неё.",
	"Даже синоби не подкрадётся к тайтлу, пока ты не вставишь в поле ссылку с AnimeLIB.",
	"Оре-но сэкай! Сначала вставь в поле ссылку на тайтл с AnimeLIB, потом уже спасение мира.",
	"Сэмпай, вставь в поле ссылку на тайтл с AnimeLIB, и я разрешу смотреть опенинг без пропуска.",
	"Аниме-марафон не начнётся, пока в это поле не попадёт ссылка на тайтл с AnimeLIB.",
	"Твой внутренний герой просыпается с мыслью: вставь в поле ссылку на тайтл с AnimeLIB.",
	"Каждый раз, когда поле пустует, где-то плачет тиби — дай ему ссылку на тайтл с AnimeLIB.",
	"Сэмпай, я верю: где-то в буфере обмена уже ждёт ссылка на тайтл с AnimeLIB для этого поля.",
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
