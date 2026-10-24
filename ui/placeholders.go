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
