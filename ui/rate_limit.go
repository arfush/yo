package ui

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/arfush/yoanime/internal/animelib"
)

// rateLimitTextSize and rateLimitTextColor style the temporary message that
// replaces the empty middle of the header while the API is rate limited.
const rateLimitTextSize = unit.Sp(11.5)

var rateLimitTextColor = color.NRGBA{R: 0xf0, G: 0xb4, B: 0x5e, A: 0xff}

// rateLimitPhrases is shown in the header while requests are paused. One is
// picked at random each time a new cool-down starts.
var rateLimitPhrases = []string{
	"Семпай, притормози — сервер медитирует",
	"Исекай-таймаут: подожди пару мгновений",
	"Аниме на паузе, а сервер — на подзарядке",
	"Не спойлерь запросами, дай серверу серию досмотреть",
	"Магия перезаряжается, подожди чуток",
	"Сервер ушёл на опенинг, скоро вернётся",
	"Тише, семпай, тайтлы качаются",
	"Дай серверу передохнуть между сериями",
	"Гигантский молот на перезарядке",
	"Сервер копит ману, подожди немного",
	"Не трогай пульт — идёт реклама",
	"Сейчас будет «продолжение следует»",
	"Сервер в режиме цундере: подожди, потом ответит",
	"Терпение, кохай, это тренировка",
	"Запросы ушли на филлер, скоро вернёмся к сюжету",
	"Сервер смотрит опенинг, не перематывай",
	"Погоди секунду, коты-помощники устали",
	"Сейчас не время для расспросов, дай отдышаться",
	"Данж на перезагрузке, отдохни у входа",
	"Сервер применяет «Отдых» — подожди ход",
}

// rateLimitBanner keeps the phrase and visibility state for the header notice.
type rateLimitBanner struct {
	active bool
	phrase string
}

// update refreshes the banner from the API client's shared notice state. The
// notice is visible from the 429 until the pause runs out or a request gets a
// successful answer. Letting the countdown expire hides the text even when no
// retry was attempted, so it cannot linger after the timer reaches zero.
func (b *rateLimitBanner) update() {
	pending := animelib.RateLimitNoticePending()
	if pending && animelib.RateLimitRemaining() > 0 {
		if !b.active {
			b.active = true
			b.phrase = rateLimitPhrases[rand.Intn(len(rateLimitPhrases))]
		}
		return
	}
	if pending {
		// The cool-down is over but no request has been made since; drop the
		// stale notice now that the countdown has nothing left to show.
		animelib.DismissRateLimitNotice()
	}
	b.active = false
	b.phrase = ""
}

// rateLimitWaitLabel renders the remaining pause as whole seconds for the
// header notice. Rounding up avoids showing "0 с" while requests are still
// parked for a fraction of a second.
func rateLimitWaitLabel(remaining time.Duration) string {
	seconds := int(math.Ceil(remaining.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("%d с", seconds)
}

func (b *rateLimitBanner) notice(gtx layout.Context, g geometry) (headerNotice, bool) {
	b.update()
	if !b.active {
		return headerNotice{}, false
	}
	// Repaint while the notice is visible so it disappears promptly once the
	// request gets through, even on screens that do not otherwise animate.
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(200 * time.Millisecond)})

	text := b.phrase
	if remaining := animelib.RateLimitRemaining(); remaining > 0 {
		text += " · " + rateLimitWaitLabel(remaining)
	}
	return headerNotice{text: text, tone: headerNoticeRate}, true
}
