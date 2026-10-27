// Package discord publishes the current playback as Discord Rich Presence.
//
// Discord accepts presence from native applications over a local named pipe:
// no bot token, OAuth flow or server is involved, only a public application id.
// The connection is best effort. Discord may be closed, start after yoanime or
// restart in the middle of an episode, so the manager reconnects on its own and
// silently does nothing when the pipe is unavailable.
package discord

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// AppID is yoanime's Discord application id. It is public by design: the id only
// names the application whose name and art appear in the activity. Release
// builds may replace it with -ldflags, so a fork can point at its own app.
var AppID = "1563963634797912286"

const (
	// activityTypeWatching renders the card as "Смотрит yoanime" instead of
	// "Играет в yoanime".
	activityTypeWatching = 3
	// maxTextRunes is Discord's limit for details and state.
	maxTextRunes = 128
	// minUpdateInterval keeps the manager well below Discord's rate limit of
	// five SET_ACTIVITY calls per twenty seconds. The activity only changes when
	// the episode does, so this delay is invisible in practice.
	minUpdateInterval = 4 * time.Second
	// reconnectDelay is how long to wait before dialing again when Discord is
	// not running. A late Discord start is picked up within this window.
	reconnectDelay = 10 * time.Second
)

// Presence is the text shown in the activity card. yoanime deliberately
// publishes no artwork: Discord renders a square image and would crop a
// portrait poster, and sending the cover through an image proxy is not worth
// the extra dependency. The app icon is enough.
type Presence struct {
	Title   string
	Episode string
	Total   int
	Movie   bool
}

// Details is the second line of the card. Serial episodes read
// "Название 5/12"; films and titles without a known episode or total read
// "Название". The episode suffix is kept whole: a long title is the part that
// gets shortened, so the episode number always stays visible. The voiceover is
// deliberately not included, as requested.
func (p Presence) Details() string {
	title := strings.TrimSpace(p.Title)
	suffix := ""
	if !p.Movie && title != "" && p.Episode != "" && p.Total > 0 {
		suffix = fmt.Sprintf(" %s/%d", p.Episode, p.Total)
	}
	if suffix == "" {
		return truncate(title, maxTextRunes)
	}
	budget := maxTextRunes - runeLen(suffix)
	if budget <= 0 {
		return truncate(strings.TrimSpace(suffix), maxTextRunes)
	}
	return truncate(title, budget) + suffix
}

// key identifies the activity content. The manager skips updates whose key did
// not change, which keeps the polling loop in the UI from touching the pipe.
func (p Presence) key() string {
	return p.Details()
}

// activity builds the SET_ACTIVITY payload. The application name is left to
// Discord ("yoanime"), and the default status display type keeps the compact
// line free of the "Watching" verb. Details carries the title and episode.
func (p Presence) activity() map[string]any {
	return map[string]any{
		"type":    activityTypeWatching,
		"details": p.Details(),
	}
}

// Manager owns the single presence connection. Set and Clear may be called from
// any goroutine; they only update the desired state and wake the worker.
type Manager struct {
	appID string
	pid   int

	mu      sync.Mutex
	pending *Presence

	wake chan struct{}
	lost chan *ipcClient
	quit chan struct{}
	done chan struct{}

	closeOnce sync.Once
}

// New returns a manager for the application id. An empty id disables the
// integration without nil checks at every call site.
func New(appID string) *Manager {
	m := &Manager{
		appID: strings.TrimSpace(appID),
		pid:   os.Getpid(),
		wake:  make(chan struct{}, 1),
		lost:  make(chan *ipcClient, 4),
		quit:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	if m.appID == "" {
		close(m.done)
		return m
	}
	go m.loop()
	return m
}

// Set publishes the presence. Repeated calls with the same content are cheap.
func (m *Manager) Set(p Presence) {
	if m == nil || m.appID == "" {
		return
	}
	value := p
	m.mu.Lock()
	m.pending = &value
	m.mu.Unlock()
	m.signal()
}

// Clear removes the activity from the profile, for example when playback ends
// or the user turns the integration off.
func (m *Manager) Clear() {
	if m == nil || m.appID == "" {
		return
	}
	m.mu.Lock()
	m.pending = nil
	m.mu.Unlock()
	m.signal()
}

// Close clears the activity and stops the worker. It is safe to call twice.
func (m *Manager) Close() {
	if m == nil || m.appID == "" {
		return
	}
	m.closeOnce.Do(func() { close(m.quit) })
	select {
	case <-m.done:
	case <-time.After(2 * time.Second):
	}
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) take() (*Presence, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return nil, false
	}
	p := *m.pending
	return &p, true
}

// loop keeps the desired presence and the pipe in sync. It owns the client, so
// the watcher only reports which client broke and never touches the connection
// itself.
func (m *Manager) loop() {
	defer close(m.done)
	var (
		client   *ipcClient
		lastSent time.Time
		lastKey  string
		nextDial time.Time
	)
	drop := func() {
		if client != nil {
			client.close()
			client = nil
		}
		lastKey = ""
	}
	defer func() {
		if client == nil {
			return
		}
		if lastKey != "" {
			_ = client.clear()
		}
		client.close()
	}()

	for {
		presence, ok := m.take()
		if !ok {
			if client != nil {
				if lastKey != "" {
					_ = client.clear()
				}
				drop()
			}
			select {
			case <-m.wake:
			case <-m.quit:
				return
			case <-m.lost:
				// Drain the report of the client that was just dropped;
				// nobody owns the pipe right now.
			}
			continue
		}

		key := presence.key()
		if client != nil && key == lastKey {
			select {
			case <-m.wake:
			case <-m.quit:
				return
			case broken := <-m.lost:
				if broken == client {
					drop()
				}
			}
			continue
		}

		if client == nil {
			// Do not dial on every wake: a missing Discord would otherwise
			// spend the whole loop inside pipe timeouts.
			if wait := time.Until(nextDial); wait > 0 {
				select {
				case <-m.wake:
				case <-m.quit:
					return
				case <-m.lost:
				case <-time.After(wait):
				}
				continue
			}
			c, err := dialIPC(m.appID)
			if err != nil {
				nextDial = time.Now().Add(reconnectDelay)
				continue
			}
			client = c
			lastKey = ""
			go func(c *ipcClient) {
				if err := c.watch(); err != nil {
					select {
					case m.lost <- c:
					case <-m.quit:
					}
				}
			}(c)
		}

		if wait := minUpdateInterval - time.Since(lastSent); wait > 0 {
			select {
			case <-m.wake:
				continue
			case <-m.quit:
				return
			case broken := <-m.lost:
				if broken == client {
					drop()
					continue
				}
			case <-time.After(wait):
			}
		}

		if err := client.setActivity(presence.activity()); err != nil {
			drop()
			continue
		}
		lastSent = time.Now()
		lastKey = key

		select {
		case <-m.wake:
		case <-m.quit:
			return
		case broken := <-m.lost:
			if broken == client {
				drop()
			}
		}
	}
}

func truncate(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func runeLen(text string) int {
	return len([]rune(text))
}
