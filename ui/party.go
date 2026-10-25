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

	// A guest that just asked the host for a pause or a seek must not let the
	// drift loop undo its own input before the confirming anchor arrives.
	partyControlHold = 1500 * time.Millisecond
	// A position jump larger than this between two guest ticks is a person
	// seeking in mpv; the drift loop never moves the picture this far.
	partyUserSeekThreshold = 1.5
)

// partySnapshot is a consistent copy of the room state for the UI.
type partySnapshot struct {
	Provider watchparty.Provider
	Key      string
	Role     watchparty.Role
	Active   bool
	Started  bool
	HostSeen bool
	// HostEverSeen stays true across a reconnect grace window, so the waiting
	// screen does not mistake a dropped host for a wrong room key.
	HostEverSeen bool
	State        watchparty.State
	Notice       string
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
	control  bool
	session  *watchparty.Session
	state    watchparty.State
	hasState bool
	started  bool
	hostSeen bool
	// hostEverSeen keeps the waiting screen from treating a reconnect window
	// as a wrong key.
	hostEverSeen bool
	notice       string
	// closedNotice explains why the last guest room disappeared. The party
	// screen consumes it on the UI goroutine, so the fields it mutates stay
	// owned by layout.
	closedNotice string

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
		control:  true,
	}
	if path, err := appdata.WatchPartyConfigPath(); err == nil {
		manager.cfg = watchparty.LoadConfig(path)
	}
	saved := ""
	if app != nil && app.library != nil {
		manager.control = app.library.SharedControlEnabled()
		saved = app.library.WatchPartyProvider()
	}
	manager.provider = preferredProvider(manager.cfg, saved)
	return manager
}

// preferredProvider is the relay a new room starts with: the one picked in the
// settings when this build has it, otherwise the only configured relay, and
// Cloudflare as the historical default.
func preferredProvider(cfg watchparty.Config, saved string) watchparty.Provider {
	if preferred := watchparty.Provider(saved); preferred.Valid() && cfg.Configured(preferred) {
		return preferred
	}
	if !cfg.Configured(watchparty.Cloudflare) && cfg.Configured(watchparty.Supabase) {
		return watchparty.Supabase
	}
	return watchparty.Cloudflare
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

// HasProviderChoice reports whether the chip has anything to switch to. The
// official build ships a single relay, so the chip stays a plain label there
// instead of a control that leads nowhere.
func (m *partyManager) HasProviderChoice() bool {
	if m == nil {
		return false
	}
	return m.cfg.Configured(watchparty.Cloudflare) && m.cfg.Configured(watchparty.Supabase)
}

// SetProvider switches the chip while no room is open and remembers the
// choice, so the next room starts with the same relay.
func (m *partyManager) SetProvider(provider watchparty.Provider) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.provider = provider
	m.mu.Unlock()
	if m.app != nil && m.app.library != nil {
		_ = m.app.library.SetWatchPartyProvider(string(provider))
	}
}

// SetSharedControl stores whether guests may control playback. The host reads
// it for incoming commands; a guest reads it before sending its own.
func (m *partyManager) SetSharedControl(enabled bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.control = enabled
	m.mu.Unlock()
}

// SharedControl reports the current shared-control setting.
func (m *partyManager) SharedControl() bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.control
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
	m.hostEverSeen = false
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
		Provider:     m.provider,
		Key:          "",
		Started:      m.started,
		HostSeen:     m.hostSeen,
		HostEverSeen: m.hostEverSeen,
		State:        m.state,
		Notice:       m.notice,
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
	m.hostEverSeen = false
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
		var control watchparty.Control
		goneNotice := ""
		m.mu.Lock()
		switch update.Kind {
		case watchparty.UpdateState:
			m.state = update.State
			m.hasState = true
			m.started = update.State.Started()
			m.hostSeen = true
			m.hostEverSeen = true
			m.notice = ""
		case watchparty.UpdateHostSeen:
			m.hostSeen = true
			m.hostEverSeen = true
		case watchparty.UpdateHostLeft:
			// The grace window expired: the host had its chance to reconnect,
			// so the room is over and the guest returns to the join form.
			m.hostSeen = false
			m.notice = hostLostNotice
			if session.Role == watchparty.Guest {
				goneNotice = hostLostNotice
			}
		case watchparty.UpdateHostClosed:
			m.hostSeen = false
			m.notice = hostClosedNotice
			if session.Role == watchparty.Guest {
				goneNotice = hostClosedNotice
			}
		case watchparty.UpdateControl:
			control = update.Control
		case watchparty.UpdateError:
			if update.Err != nil {
				m.notice = update.Err.Error()
			}
		}
		state := m.state
		started := m.started
		m.mu.Unlock()

		if goneNotice != "" {
			m.hostGone(goneNotice)
			return
		}
		if update.Kind == watchparty.UpdateControl {
			m.applyControl(session, control)
			continue
		}

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

// The two reasons a guest room disappears: the host closed it on purpose, or
// the reconnect grace window expired after the host dropped out.
const (
	hostClosedNotice = "Создатель закрыл комнату"
	hostLostNotice   = "Создатель комнаты вышел"
)

// hostGone tears the guest room down once the host is definitively gone. mpv
// is stopped without the finish callback, so the window has to be restored
// here; the party screen then shows the join form with the reason.
func (m *partyManager) hostGone(notice string) {
	m.stopGuest()
	m.Delete()
	m.mu.Lock()
	m.closedNotice = notice
	m.mu.Unlock()
	m.invalidate()
}

// takeClosedNotice returns the reason the last guest room ended, once. The
// caller is the party screen, which is the only owner of its UI fields.
func (m *partyManager) takeClosedNotice() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	text := m.closedNotice
	m.closedNotice = ""
	return text
}

// applyControl carries out a guest command when shared control is enabled. The
// host publishes a fresh anchor immediately, so every guest reacts within a
// frame instead of waiting for the next heartbeat tick.
func (m *partyManager) applyControl(session *watchparty.Session, control watchparty.Control) {
	if m.app == nil || m.app.playback == nil || !m.SharedControl() {
		return
	}
	m.mu.Lock()
	running := m.hostSession
	owned := m.session == session && session.Role == watchparty.Host
	m.mu.Unlock()
	if !owned || running == nil || m.app.playback.Current() != running {
		return
	}
	var err error
	switch control.Action {
	case watchparty.ControlPause:
		err = running.SetPaused(true)
	case watchparty.ControlPlay:
		err = running.SetPaused(false)
	case watchparty.ControlSeek:
		err = running.SeekTo(control.Position)
	default:
		return
	}
	if err != nil {
		return
	}
	if state, ok := m.hostState(); ok {
		_ = session.Publish(state)
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
	// Stop suppresses the finish callback that normally brings the window
	// back, so the guest would otherwise be left with no visible application.
	m.app.restorePlayerWindow()
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

// guestFinished is called when a guest's mpv exits on its own. Closing the
// player is how a guest leaves the room, so a clean exit drops the key and
// returns to the main screen instead of waiting for the host forever. A crash
// keeps the room and is retried after a pause.
func (m *partyManager) guestFinished(err error) {
	if m == nil {
		return
	}
	if err == nil {
		if m.app != nil {
			m.app.leaveParty()
			return
		}
		m.Delete()
		return
	}
	m.mu.Lock()
	if m.guestRunning.Started() {
		m.guestDone = m.guestRunning
		m.guestDoneAt = time.Now()
	}
	m.guestRunning = watchparty.State{}
	m.guestStarting = false
	m.notice = "Плеер закрылся, пробую снова"
	m.mu.Unlock()
	m.invalidate()
}

// guestLoop keeps the local mpv close to the extrapolated host position. It
// also watches for input the person makes in mpv (pause, seek) and forwards it
// to the host when shared control is on.
func (m *partyManager) guestLoop(session *watchparty.Session, stop chan struct{}) {
	ticker := time.NewTicker(partyGuestTick)
	defer ticker.Stop()
	rate := 1.0
	var lastPosition float64
	var lastSample time.Time
	var lastPaused bool
	var holdUntil time.Time
	var seeded bool
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
			seeded = false
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
			seeded = false
			continue
		}
		now := time.Now()
		position := current.LivePosition()
		paused := current.Paused()
		if !seeded {
			seeded = true
		} else if m.SharedControl() {
			// Between two ticks only a person can change the player: the drift
			// loop acts below and records its own corrections in the baseline.
			switch {
			case paused != lastPaused:
				action := watchparty.ControlPlay
				if paused {
					action = watchparty.ControlPause
				}
				_ = session.SendControl(action, 0)
				holdUntil = now.Add(partyControlHold)
			case !lastPaused && !lastSample.IsZero():
				expected := lastPosition + now.Sub(lastSample).Seconds()
				// A stalled network freezes the position instead of moving
				// it, so only a real jump counts as a seek; otherwise a slow
				// connection would drag the host back to the guest.
				if position != lastPosition && math.Abs(position-expected) > partyUserSeekThreshold {
					_ = session.SendControl(watchparty.ControlSeek, position)
					holdUntil = now.Add(partyControlHold)
				}
			}
		}
		// Keep the baseline fresh even while mpv is paused, so the next real
		// action is measured from the current picture.
		tracked := position
		lastPaused = paused
		lastSample = now
		if now.Before(holdUntil) {
			// The command is still travelling; correcting the drift now would
			// undo the guest's own pause or seek before the host confirms it.
			lastPosition = tracked
			continue
		}

		switch {
		case !state.Playing:
			if !current.Paused() {
				_ = current.SetPaused(true)
			}
			if math.Abs(position-state.Position) > 0.8 {
				_ = current.SeekTo(state.Position)
				tracked = state.Position
			}
			if rate != 1 {
				_ = current.SetSpeed(1)
				rate = 1
			}
		case current.Paused():
			_ = current.SetPaused(false)
			fallthrough
		default:
			want, _, ok := session.ExpectedPosition(now)
			if !ok {
				lastPaused = current.Paused()
				lastPosition = tracked
				continue
			}
			drift := position - want
			switch {
			case math.Abs(drift) > partySeekThreshold:
				target := want + partySeekLead
				_ = current.SeekTo(target)
				tracked = target
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
		// Record the state we just enforced, otherwise the next tick would
		// mistake our own pause or seek for input from the person.
		lastPaused = current.Paused()
		lastPosition = tracked
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
