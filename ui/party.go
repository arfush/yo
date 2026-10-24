package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arfush/yonime/internal/appdata"
	"github.com/arfush/yonime/internal/library"
	"github.com/arfush/yonime/internal/playback"
	"github.com/arfush/yonime/internal/watchparty"
)

const (
	// The host publishes a fresh anchor often enough that a guest joining in
	// the middle of an episode converges within a few seconds, and rarely
	// enough that a Supabase free project lasts through months of watching.
	partyHeartbeatPlaying = 4 * time.Second
	partyHeartbeatIdle    = 10 * time.Second
	partyHostTick         = time.Second
	partyGuestTick        = time.Second
	partyCreateTimeout    = 20 * time.Second

	// Drift is corrected in two ways: a gentle rate change hides errors below a
	// second, a seek fixes anything larger. The seek leads by a little because
	// the host kept watching while the command travelled.
	partySeekThreshold  = 0.9
	partySpeedThreshold = 0.22
	partySeekLead       = 0.15
	partySpeedGain      = 0.25
	partyMinRate        = 0.94
	partyMaxRate        = 1.06
)

// partySnapshot is a consistent copy of the room state for the UI.
type partySnapshot struct {
	Provider watchparty.Provider
	Key      string
	Role     watchparty.Role
	Active   bool
	Started  bool
	HostSeen bool
	State    watchparty.State
	Notice   string
}

type hostMeta struct {
	Slug    string
	Name    string
	Voice   string
	Episode string
}

// partyManager owns the room connection and both sync loops. It is the only
// place that knows how a host publishes and how a guest follows.
type partyManager struct {
	app *application
	cfg watchparty.Config

	mu       sync.Mutex
	provider watchparty.Provider
	session  *watchparty.Session
	state    watchparty.State
	hasState bool
	started  bool
	hostSeen bool
	notice   string

	hostSession *playback.Session
	hostMeta    hostMeta
	hostStop    chan struct{}

	guestRunning  watchparty.State
	guestStarting bool
	guestDone     watchparty.State
	guestDoneAt   time.Time
	guestStop     chan struct{}
	consumeStop   chan struct{}
}

func newPartyManager(app *application) *partyManager {
	manager := &partyManager{
		app:      app,
		provider: watchparty.Cloudflare,
	}
	if path, err := appdata.WatchPartyConfigPath(); err == nil {
		manager.cfg = watchparty.LoadConfig(path)
	}
	// A user who configured only one relay should not have to click past the
	// other one before the first attempt.
	if !manager.cfg.Configured(watchparty.Cloudflare) && manager.cfg.Configured(watchparty.Supabase) {
		manager.provider = watchparty.Supabase
	}
	return manager
}

// Provider returns the backend shown in the chip.
func (m *partyManager) Provider() watchparty.Provider {
	if m == nil {
		return watchparty.Cloudflare
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.provider
}

// SetProvider switches the chip while no room is open.
func (m *partyManager) SetProvider(provider watchparty.Provider) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.provider = provider
	m.mu.Unlock()
}

// Create opens a fresh room and returns its key.
func (m *partyManager) Create(provider watchparty.Provider) (string, error) {
	if m == nil {
		return "", errors.New("совместный просмотр недоступен")
	}
	if !m.cfg.Configured(provider) {
		return "", fmt.Errorf("%s не настроен", provider.Label())
	}
	m.mu.Lock()
	busy := m.session != nil
	m.mu.Unlock()
	if busy {
		return "", errors.New("комната уже создана")
	}

	ctx, cancel := context.WithTimeout(context.Background(), partyCreateTimeout)
	defer cancel()
	session, err := watchparty.Create(ctx, m.cfg, provider)
	if err != nil {
		return "", err
	}
	m.adopt(session)
	return session.Key, nil
}

// Join connects to an existing room.
func (m *partyManager) Join(provider watchparty.Provider, key string) error {
	if m == nil {
		return errors.New("совместный просмотр недоступен")
	}
	if !m.cfg.Configured(provider) {
		return fmt.Errorf("%s не настроен", provider.Label())
	}
	if _, err := watchparty.NormalizeKey(key); err != nil {
		return err
	}
	m.mu.Lock()
	busy := m.session != nil
	m.mu.Unlock()
	if busy {
		m.Delete()
	}

	ctx, cancel := context.WithTimeout(context.Background(), partyCreateTimeout)
	defer cancel()
	session, err := watchparty.Join(ctx, m.cfg, provider, key)
	if err != nil {
		return err
	}
	m.adopt(session)
	return nil
}

// Delete closes the room and forgets the key.
func (m *partyManager) Delete() {
	if m == nil {
		return
	}
	m.mu.Lock()
	session := m.session
	m.session = nil
	m.state = watchparty.State{}
	m.hasState = false
	m.started = false
	m.hostSeen = false
	m.notice = ""
	// hostSession/hostMeta describe the running mpv session, not the room:
	// keep them so a room created later can publish the current episode.
	m.guestRunning = watchparty.State{}
	m.guestDone = watchparty.State{}
	hostStop := m.hostStop
	m.hostStop = nil
	guestStop := m.guestStop
	m.guestStop = nil
	consumeStop := m.consumeStop
	m.consumeStop = nil
	m.mu.Unlock()

	closeStop(hostStop)
	closeStop(guestStop)
	closeStop(consumeStop)
	if session != nil {
		_ = session.Close()
	}
	m.invalidate()
}

// HasRoom reports whether a room is currently open.
func (m *partyManager) HasRoom() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session != nil
}

// Snapshot copies the room state for the current frame.
func (m *partyManager) Snapshot() partySnapshot {
	if m == nil {
		return partySnapshot{Provider: watchparty.Cloudflare}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := partySnapshot{
		Provider: m.provider,
		Key:      "",
		Started:  m.started,
		HostSeen: m.hostSeen,
		State:    m.state,
		Notice:   m.notice,
	}
	if m.session != nil {
		snapshot.Active = true
		snapshot.Key = m.session.Key
		snapshot.Role = m.session.Role
		snapshot.Provider = m.session.Provider
	}
	return snapshot
}

// ExpectedPosition extrapolates the host position on the local clock.
func (m *partyManager) ExpectedPosition() (float64, bool) {
	if m == nil {
		return 0, false
	}
	m.mu.Lock()
	session := m.session
	m.mu.Unlock()
	if session == nil {
		return 0, false
	}
	position, _, ok := session.ExpectedPosition(time.Now())
	return position, ok
}

// Close stops the room and both loops during application shutdown.
func (m *partyManager) Close() {
	if m == nil {
		return
	}
	m.Delete()
}

func (m *partyManager) adopt(session *watchparty.Session) {
	m.mu.Lock()
	previous := m.session
	hostStop := m.hostStop
	guestStop := m.guestStop
	consumeStop := m.consumeStop
	m.session = session
	m.provider = session.Provider
	m.state = watchparty.State{}
	m.hasState = false
	m.started = false
	m.hostSeen = false
	m.notice = ""
	// hostSession/hostMeta describe the running mpv session, not the room: a
	// room created in the middle of an episode publishes it right away.
	m.guestRunning = watchparty.State{}
	m.guestDone = watchparty.State{}
	m.guestStarting = false
	stop := make(chan struct{})
	m.guestStop = stop
	m.hostStop = nil
	stopUpdates := make(chan struct{})
	m.consumeStop = stopUpdates
	running := m.hostSession
	m.mu.Unlock()

	closeStop(hostStop)
	closeStop(guestStop)
	closeStop(consumeStop)
	if previous != nil {
		_ = previous.Close()
	}

	go m.consume(session, stopUpdates)
	if session.Role == watchparty.Guest {
		go m.guestLoop(session, stop)
	} else if running != nil && m.app != nil && m.app.playback != nil && m.app.playback.Current() == running {
		m.startHostLoop(session, running)
	}
	m.invalidate()
}

func (m *partyManager) consume(session *watchparty.Session, stop chan struct{}) {
	for {
		var update watchparty.Update
		select {
		case <-stop:
			return
		case update = <-session.Updates():
		}
		if !m.owns(session) {
			return
		}
		m.mu.Lock()
		switch update.Kind {
		case watchparty.UpdateState:
			m.state = update.State
			m.hasState = true
			m.started = update.State.Started()
			m.hostSeen = true
			m.notice = ""
		case watchparty.UpdateHostSeen:
			m.hostSeen = true
		case watchparty.UpdateHostLeft:
			m.hostSeen = false
			m.notice = "Создатель комнаты отключился"
		case watchparty.UpdateError:
			if update.Err != nil {
				m.notice = update.Err.Error()
			}
		}
		state := m.state
		started := m.started
		m.mu.Unlock()

		m.invalidate()
		if session.Role != watchparty.Guest {
			continue
		}
		if started {
			m.startGuest(state)
		} else {
			// The host closed playback, so the guest stops too instead of
			// rolling on through its own queue alone.
			m.stopGuest()
		}
	}
}

// stopGuest ends the mpv session a guest is following, if there is one.
func (m *partyManager) stopGuest() {
	m.mu.Lock()
	running := m.guestRunning.Started()
	m.guestRunning = watchparty.State{}
	m.guestStarting = false
	m.mu.Unlock()
	if !running || m.app == nil || m.app.playback == nil {
		return
	}
	if current := m.app.playback.Current(); current != nil {
		current.Stop()
	}
}

func (m *partyManager) owns(session *watchparty.Session) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session == session
}

// startGuest launches mpv for a guest when the host opens another title or
// episode. Duplicate calls for the same media are ignored.
func (m *partyManager) startGuest(state watchparty.State) {
	if m.app == nil || m.app.playback == nil || !state.Started() {
		return
	}
	// mpv already owns a queue of following episodes, so when the host rolls
	// over to the next one the guest's player has usually rolled over too.
	// Restarting in that case would stop the picture for no reason; only a
	// jump to another file needs a new session.
	if current := m.app.playback.Current(); current != nil {
		if number, _ := current.Episode(); number != "" && number == state.Episode {
			m.mu.Lock()
			m.guestRunning = state
			m.mu.Unlock()
			return
		}
	}
	m.mu.Lock()
	if m.app.playback.Current() == nil && !m.guestStarting {
		m.guestRunning = watchparty.State{}
	}
	if m.guestStarting {
		m.mu.Unlock()
		return
	}
	if m.guestRunning.Started() && m.guestRunning.SamePlayback(state) {
		m.mu.Unlock()
		return
	}
	// guestDone means the local player already finished this media. A failure is
	// retried after a pause; a player the guest closed on purpose stays closed
	// until the host switches to another episode or title.
	if state.SamePlayback(m.guestDone) {
		if m.guestDoneAt.IsZero() || time.Since(m.guestDoneAt) < 45*time.Second {
			m.mu.Unlock()
			return
		}
	}
	m.guestStarting = true
	m.guestRunning = state
	m.mu.Unlock()

	go func() {
		err := m.app.startPartyPlayback(state)
		m.mu.Lock()
		m.guestStarting = false
		if err != nil {
			m.guestRunning = watchparty.State{}
			m.notice = err.Error()
		}
		m.mu.Unlock()
		m.invalidate()
	}()
}

// guestFailed is called when mpv could not start for a guest.
func (m *partyManager) guestFailed(err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.guestRunning.Started() {
		m.guestDone = m.guestRunning
		m.guestDoneAt = time.Now()
	}
	m.guestRunning = watchparty.State{}
	m.guestStarting = false
	if err != nil {
		m.notice = err.Error()
	}
	m.mu.Unlock()
	m.invalidate()
}

// guestFinished is called when a guest's mpv exits on its own, either because
// the guest closed it or because the queue ran out.
func (m *partyManager) guestFinished() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.guestRunning.Started() {
		m.guestDone = m.guestRunning
		// A zero time keeps this playback closed until the host moves on.
		m.guestDoneAt = time.Time{}
	}
	m.guestRunning = watchparty.State{}
	m.mu.Unlock()
	m.invalidate()
}

// guestLoop keeps the local mpv close to the extrapolated host position.
func (m *partyManager) guestLoop(session *watchparty.Session, stop chan struct{}) {
	ticker := time.NewTicker(partyGuestTick)
	defer ticker.Stop()
	rate := 1.0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if m.app == nil || m.app.playback == nil {
			continue
		}
		current := m.app.playback.Current()
		if current == nil {
			continue
		}
		state, ok := session.State()
		if !ok || !state.Started() {
			continue
		}
		number, _ := current.Episode()
		if number == "" || number != state.Episode {
			// mpv is still opening the file or the host moved on; the drift
			// numbers are meaningless until file-loaded arrives.
			rate = 1
			continue
		}
		if !state.Playing {
			if !current.Paused() {
				_ = current.SetPaused(true)
			}
			if math.Abs(current.LivePosition()-state.Position) > 0.8 {
				_ = current.SeekTo(state.Position)
			}
			if rate != 1 {
				_ = current.SetSpeed(1)
				rate = 1
			}
			continue
		}
		if current.Paused() {
			_ = current.SetPaused(false)
		}
		want, _, ok := session.ExpectedPosition(time.Now())
		if !ok {
			continue
		}
		drift := current.LivePosition() - want
		switch {
		case math.Abs(drift) > partySeekThreshold:
			_ = current.SeekTo(want + partySeekLead)
			if rate != 1 {
				_ = current.SetSpeed(1)
				rate = 1
			}
		case math.Abs(drift) > partySpeedThreshold:
			next := clampRate(1 - drift*partySpeedGain)
			if math.Abs(next-rate) > 0.004 {
				_ = current.SetSpeed(next)
				rate = next
			}
		default:
			if rate != 1 {
				_ = current.SetSpeed(1)
				rate = 1
			}
		}
	}
}

// hostStarted remembers the running mpv session and, when a host room is
// already open, publishes it. Remembering even without a room lets a user open
// a room in the middle of an episode and start sharing right away.
func (m *partyManager) hostStarted(session *playback.Session, title library.Title, voice, episode string) {
	if m == nil || session == nil {
		return
	}
	m.mu.Lock()
	m.hostSession = session
	m.hostMeta = hostMeta{
		Slug:    title.Slug,
		Name:    title.Russian,
		Voice:   voice,
		Episode: episode,
	}
	room := m.session
	m.mu.Unlock()
	if room == nil || room.Role != watchparty.Host {
		return
	}
	m.startHostLoop(room, session)
}

// startHostLoop (re)connects the heartbeat of an existing room to the running
// mpv session.
func (m *partyManager) startHostLoop(room *watchparty.Session, session *playback.Session) {
	m.mu.Lock()
	oldStop := m.hostStop
	stop := make(chan struct{})
	m.hostStop = stop
	m.mu.Unlock()

	closeStop(oldStop)
	room.SetSource(func() (watchparty.State, bool) { return m.hostState() })
	go m.hostLoop(room, stop, session)
	// Publish once immediately so a guest does not wait for the first tick.
	if state, ok := m.hostState(); ok {
		_ = room.Publish(state)
	}
	m.invalidate()
}

// hostState snapshots the running mpv session into a room anchor.
func (m *partyManager) hostState() (watchparty.State, bool) {
	m.mu.Lock()
	session := m.hostSession
	meta := m.hostMeta
	m.mu.Unlock()
	if session == nil || meta.Slug == "" {
		return watchparty.State{}, false
	}
	number, index := session.Episode()
	if number == "" {
		number = meta.Episode
	}
	return watchparty.State{
		TitleSlug:    meta.Slug,
		TitleName:    meta.Name,
		Voice:        meta.Voice,
		Episode:      number,
		EpisodeIndex: index,
		Playing:      !session.Paused(),
		Position:     session.LivePosition(),
	}, true
}

func (m *partyManager) hostLoop(room *watchparty.Session, stop chan struct{}, session *playback.Session) {
	ticker := time.NewTicker(partyHostTick)
	defer ticker.Stop()

	var lastPublish time.Time
	var lastPosition float64
	var lastSample time.Time
	var lastPlaying bool
	var lastEpisode string
	first := true

	for {
		var now time.Time
		select {
		case <-stop:
			room.SetSource(nil)
			return
		case now = <-ticker.C:
		}
		if m.app == nil || m.app.playback.Current() != session {
			// mpv exited: clear the room so guests show the waiting screen
			// instead of extrapolating a finished episode forever.
			m.mu.Lock()
			if m.hostSession == session {
				m.hostSession = nil
			}
			m.mu.Unlock()
			room.SetSource(nil)
			_ = room.Publish(watchparty.State{})
			m.invalidate()
			return
		}

		state, ok := m.hostState()
		if !ok {
			continue
		}
		expected := lastPosition
		if lastPlaying && !lastSample.IsZero() {
			expected += now.Sub(lastSample).Seconds()
		}
		seeked := !lastSample.IsZero() && math.Abs(state.Position-expected) > 1.5
		changed := first || state.Episode != lastEpisode || state.Playing != lastPlaying
		interval := partyHeartbeatPlaying
		if !state.Playing {
			interval = partyHeartbeatIdle
		}

		if changed || seeked || now.Sub(lastPublish) >= interval {
			if err := room.Publish(state); err == nil {
				lastPublish = now
			}
			lastEpisode = state.Episode
			lastPlaying = state.Playing
			first = false
		}
		lastPosition = state.Position
		lastSample = now
	}
}

func (m *partyManager) invalidate() {
	if m == nil || m.app == nil || m.app.window == nil {
		return
	}
	m.app.window.Invalidate()
}

func closeStop(stop chan struct{}) {
	if stop == nil {
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
}

func clampRate(rate float64) float64 {
	return math.Max(partyMinRate, math.Min(partyMaxRate, rate))
}
