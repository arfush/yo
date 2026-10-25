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
	"Сэмпай, не заставляй меня ждать — просто вставь ссылку на AnimeLIB в это поле.",
	"Оре-татимаэ!.. Ладно, просто скинь ссылку на AnimeLIB, и я прощу тебя.",
	"Даже Гоку со всей своей силой не сможет найти тайтл без ссылки на AnimeLIB.",
	"Богиня интернета требует подношение в виде ссылки на AnimeLIB, иначе уйдёт спать.",
	"Даттебайо! Пока ты думаешь, целая серия уже вышла — тащи ссылку на AnimeLIB.",
	"Банкай: «Ссылка на AnimeLIB»! Активирую всю свою лень ради одного клика.",
	"Не то чтобы я так уж хотела… но без ссылки на AnimeLIB этот вечер пропал впустую.",
	"Грузовик-сан уже проехал, а ты всё ещё не дал ссылку на AnimeLIB — исекай отменяется.",
	"Сакура опадает, а поле пустое: без ссылки на AnimeLIB тайтл останется мечтой.",
	"Эрен, ты решил уничтожить всё, или просто забыл вставить ссылку на AnimeLIB?",
	"Лайт, запиши в тетрадь: «Сегодня я наконец дал ссылку на AnimeLIB».",
	"Ссылка на AnimeLIB или тысяча лет одиночества в пустом списке серий.",
	"Кавайность этой строки напрямую зависит от того, вставишь ли ты сюда ссылку на AnimeLIB.",
	"Цундэрэ-строке тоже нужно внимание: не оставляй её без ссылки на AnimeLIB.",
	"Сэмпай, я верю в тебя: один Ctrl+V — и ссылка на AnimeLIB уже здесь.",
	"Накама, капитан приказал: ссылка на AnimeLIB в это поле, и никаких исключений!",
	"Твой стенд слаб против этого поля — вставь ссылку на AnimeLIB и покажи силу.",
	"Если ссылка на AnimeLIB не появится, я буду дуться целую арку и ни секундой меньше.",
	"Итадакимас! Но сначала подай на стол ссылку на AnimeLIB, а потом уже серии.",
	"Онии-тян не любит пустые поля: накорми строку ссылкой на AnimeLIB.",
	"В этом мире магии побеждает не заклинание, а правильно вставленная ссылка на AnimeLIB.",
	"Сэмпай, пока ты ищешь мотивацию, я ищу тайтл — но без ссылки на AnimeLIB никак.",
	"Суки-суки! Твоя ссылка на AnimeLIB мне нравится больше, чем весь остальной интернет.",
	"Ганбатте! Достань из закладок ту самую ссылку на AnimeLIB и вставь её сюда.",
	"Пустое поле — это не арт-хаус, а просто повод наконец вставить ссылку на AnimeLIB.",
	"Аниме-боги смотрят на тебя свысока и ждут ссылку на AnimeLIB в этом поле.",
	"Ты можешь открыть портал в другой мир или просто вставить ссылку на AnimeLIB — второе проще.",
	"Ксо! Ещё немного, и я сама побегу искать ссылку на AnimeLIB по всему интернету.",
	"Сэмпай, не заставляй её краснеть от неловкости — вставь ссылку на AnimeLIB прямо сейчас.",
	"Умэ-умэ! Никакой магии не нужно, если у тебя есть ссылка на AnimeLIB под рукой.",
	"Даже самый длинный филлер закончится раньше, чем ты решишь вставить ссылку на AnimeLIB.",
	"Хидой! Пустое поле снова напоминает, что ты забыл про ссылку на AnimeLIB.",
	"Команда из пяти героев не справится с пустым полем, а ссылка на AnimeLIB справится легко.",
	"Оре-но тикара! Вся моя сила уходит на то, чтобы попросить у тебя ссылку на AnimeLIB.",
	"Без ссылки на AnimeLIB я как меч без рукояти — вроде есть, а толку ноль.",
	"Сэмпай, ты же не хочешь, чтобы я грустила в углу с пустым полем вместо ссылки на AnimeLIB?",
	"Загадай желание, но не звёздам, а этому полю — пусть в нём появится ссылка на AnimeLIB.",
	"Финал арки ближе, чем кажется, а ссылки на AnimeLIB в поле всё ещё нет.",
	"Тиби-версия этой строки плачет по ссылке на AnimeLIB, и я тоже почти.",
	"Накама, наш отряд собран, припасы готовы, не хватает только ссылки на AnimeLIB.",
	"Ты не поверишь, но самое сильное заклинание здесь — вставленная ссылка на AnimeLIB.",
	"Оре-но тайтл! Дай мне ссылку на AnimeLIB, и я поделюсь с тобой целой вечностью серий.",
	"Эта строка создана не для пустоты, а для ссылки на AnimeLIB, которую ты так долго хранил.",
	"Даже синоби из скрытой деревни не подкрадётся к тайтлу без ссылки на AnimeLIB.",
	"Оре-но сэкай! Сначала ссылка на AnimeLIB, потом уже спасение мира, так эффективнее.",
	"Сэмпай, если вставишь ссылку на AnimeLIB, я разрешу тебе смотреть опенинг без пропуска.",
	"Аниме-марафон не начнётся, пока в этом поле не появится хоть одна ссылка на AnimeLIB.",
	"Твой внутренний герой просыпается тоже с одной мыслью: где взять ссылку на AnimeLIB?",
	"Каждый раз, когда поле остаётся пустым, где-то умирает одна слеза — дай ему ссылку на AnimeLIB.",
	"Сэмпай, я верю, что где-то в буфере обмена тебя уже ждёт ссылка на AnimeLIB.",
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
