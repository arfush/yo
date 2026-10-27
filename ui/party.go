package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arfush/yoanime/library"
	"github.com/arfush/yoanime/log"
	"github.com/arfush/yoanime/party"
	"github.com/arfush/yoanime/platform/appdata"
	"github.com/arfush/yoanime/playback"
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
	// A guest checks that a room has a host before the waiting screen opens.
	// The relay handshake and the host's answer share this budget, so a bad
	// key or an unreachable relay cannot leave the button spinning forever.
	partyJoinTimeout = 8 * time.Second
	partyHostWait    = 5 * time.Second

	// Drift is corrected in two ways: a gentle rate change hides errors below a
	// second, a seek fixes anything larger. The seek leads by a little because
	// the host kept watching while the command travelled.
	partySeekThreshold  = 0.9
	partySpeedThreshold = 0.22
	partySeekLead       = 0.15
	partySpeedGain      = 0.25
	partyMinRate        = 0.94
	partyMaxRate        = 1.06

	// The host publishes every four seconds while playing and every ten while
	// paused. When neither arrives for far longer than that, the guest's socket
	// is probably half-open (a NAT dropped the mapping with the server none the
	// wiser): the player keeps extrapolating from the last anchor, so positions
	// look fine while a host pause or seek never arrives. The guest then asks
	// for a fresh anchor instead of running on the stale one.
	partyStaleAnchor    = 15 * time.Second
	partyRefreshBackoff = 20 * time.Second

	// A guest that just asked the host for a pause or a seek must not let the
	// drift loop undo its own input before the confirming anchor arrives.
	partyControlHold = 1500 * time.Millisecond
	// A position jump larger than this between two guest ticks is a person
	// seeking in mpv; the drift loop never moves the picture this far.
	partyUserSeekThreshold = 1.5
	// After our own seek, mpv may still report the old position for a moment
	// (especially on HLS). The guest ignores both drift and user-seek checks
	// until the new picture is visible, otherwise it mistakes its own delayed
	// correction for a person scrubbing and sends the same seek back to the
	// host.
	partySeekSettle = 1200 * time.Millisecond
	// A host restart closes mpv for a few seconds while the next episode is
	// resolved. Keep the guest's player alive through that gap; an intentionally
	// closed host session still stops the guest after this grace period.
	partyGuestStopGrace = 10 * time.Second
)

// partySnapshot is a consistent copy of the room state for the UI.
type partySnapshot struct {
	Provider party.Provider
	Key      string
	Role     party.Role
	Active   bool
	Started  bool
	HostSeen bool
	// HostEverSeen stays true across a reconnect grace window, so the waiting
	// screen does not mistake a dropped host for a wrong room key.
	HostEverSeen bool
	State        party.State
	Notice       string
	// Locked and NeedsSecret describe the secret gate. The host shows Secret on
	// its card; a guest that already proved one keeps it here too.
	Locked      bool
	NeedsSecret bool
	Secret      string
	HostName    string
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
	cfg party.Config

	mu       sync.Mutex
	provider party.Provider
	control  bool
	name     string
	session  *party.Session
	state    party.State
	hasState bool
	started  bool
	hostSeen bool
	// hostEverSeen keeps the waiting screen from treating a reconnect window
	// as a wrong key.
	hostEverSeen bool
	notice       string
	// secretAsked makes the secret prompt fire once per rotation instead of on
	// every card frame. secretNotice asks the app to open the party screen when
	// the room locks under a guest that is on the main page.
	secretAsked  bool
	secretNotice bool
	// closedNotice explains why the last guest room disappeared. The party
	// screen consumes it on the UI goroutine, so the fields it mutates stay
	// owned by layout.
	closedNotice string

	hostSession *playback.Session
	hostMeta    hostMeta
	hostStop    chan struct{}

	guestRunning  party.State
	guestStarting bool
	guestDone     party.State
	guestDoneAt   time.Time
	guestStop     chan struct{}
	consumeStop   chan struct{}
}

func newPartyManager(app *application) *partyManager {
	manager := &partyManager{
		app:      app,
		provider: party.Cloudflare,
		control:  true,
	}
	if path, err := appdata.PartyConfigPath(); err == nil {
		manager.cfg = party.LoadConfig(path)
	}
	saved := ""
	if app != nil && app.library != nil {
		manager.control = app.library.SharedControlEnabled()
		saved = app.library.PartyProvider()
	}
	manager.provider = preferredProvider(manager.cfg, saved)
	manager.name = loadPartyName(app)
	return manager
}

// loadPartyName restores the nickname from the settings, generating and storing
// a GitHub-style default on the first run.
func loadPartyName(app *application) string {
	if app != nil && app.library != nil {
		if saved := app.library.PartyName(); saved != "" {
			if normalized, err := party.NormalizeName(saved); err == nil {
				return normalized
			}
		}
	}
	name, err := party.NewName()
	if err != nil {
		return "yoanime"
	}
	if app != nil && app.library != nil {
		_ = app.library.SetPartyName(name)
	}
	return name
}

// preferredProvider is the relay a new room starts with: the one picked in the
// settings when this build has it, otherwise the only configured relay, and
// Cloudflare as the historical default.
func preferredProvider(cfg party.Config, saved string) party.Provider {
	if preferred := party.Provider(saved); preferred.Valid() && cfg.Configured(preferred) {
		return preferred
	}
	if !cfg.Configured(party.Cloudflare) && cfg.Configured(party.Supabase) {
		return party.Supabase
	}
	return party.Cloudflare
}

// Provider returns the relay a new room is created on.
func (m *partyManager) Provider() party.Provider {
	if m == nil {
		return party.Cloudflare
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.provider
}

// Config returns a copy of the relay settings currently in use.
func (m *partyManager) Config() party.Config {
	if m == nil {
		return party.Config{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// SetRelayConfig stores the endpoints entered in the settings and applies them
// to the next room. An already open room keeps the config it dialled with.
func (m *partyManager) SetRelayConfig(cfg party.Config) error {
	if m == nil {
		return errors.New("совместный просмотр недоступен")
	}
	cfg = cfg.Normalized()
	if err := cfg.Validate(); err != nil {
		return err
	}
	path, err := appdata.PartyConfigPath()
	if err != nil {
		return err
	}
	if err := party.SaveConfig(path, cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

// SetProvider remembers the relay chosen in the settings, so the next room
// starts with the same backend.
func (m *partyManager) SetProvider(provider party.Provider) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.provider = provider
	m.mu.Unlock()
	if m.app != nil && m.app.library != nil {
		_ = m.app.library.SetPartyProvider(string(provider))
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
	room := m.session
	m.mu.Unlock()
	if room == nil || room.Role != party.Host {
		return
	}
	// Publish the new decision immediately instead of waiting for the next
	// heartbeat, so a guest locks its player as soon as the host flips the
	// setting. The settings card runs on the UI thread, so the network write
	// stays in the background.
	go func() {
		if state, ok := m.hostState(); ok {
			_ = room.Publish(state)
		}
	}()
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

func guestControlAllowed(state party.State, local bool) bool {
	if allowed, ok := state.GuestsControl(); ok {
		return allowed
	}
	return local
}

// Name returns the nickname this client sends to the other side.
func (m *partyManager) Name() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.name
}

// SetName validates and stores the nickname, applies it to the open room and
// keeps it for the next one.
func (m *partyManager) SetName(name string) error {
	if m == nil {
		return errors.New("совместный просмотр недоступен")
	}
	normalized, err := party.NormalizeName(name)
	if err != nil {
		return err
	}
	if m.app != nil && m.app.library != nil {
		if err := m.app.library.SetPartyName(normalized); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.name = normalized
	room := m.session
	m.mu.Unlock()
	if room != nil {
		room.SetName(normalized)
	}
	return nil
}

// Create opens a fresh room and returns its key.
func (m *partyManager) Create(provider party.Provider) (string, error) {
	if m == nil {
		return "", errors.New("совместный просмотр недоступен")
	}
	cfg := m.Config()
	if !cfg.Configured(provider) {
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
	session, err := party.Create(ctx, cfg, provider, m.Name())
	if err != nil {
		return "", err
	}
	m.adopt(session)
	return session.Key, nil
}

// Join connects to an existing room. The relay comes from the key, not from
// the provider selected in the settings: the host's room lives on its own
// backend and a guest must reach the same one.
func (m *partyManager) Join(key string) error {
	if m == nil {
		return errors.New("совместный просмотр недоступен")
	}
	normalized, err := party.NormalizeKey(key)
	if err != nil {
		return err
	}
	provider, err := party.ProviderFromKey(normalized)
	if err != nil {
		return err
	}
	cfg := m.Config()
	if !cfg.Configured(provider) {
		return fmt.Errorf("для этого ключа нужен %s — он не настроен в этой сборке", provider.Label())
	}
	m.mu.Lock()
	busy := m.session != nil
	m.mu.Unlock()
	if busy {
		m.Delete()
	}

	ctx, cancel := context.WithTimeout(context.Background(), partyJoinTimeout)
	defer cancel()
	session, err := party.Join(ctx, cfg, normalized, m.Name())
	if err != nil {
		return err
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, partyHostWait)
	err = session.WaitHost(probeCtx)
	probeCancel()
	if err != nil {
		_ = session.Close()
		return err
	}
	// The card tells the UI whether the room needs a secret before the first
	// anchor arrives. Waiting for it also rejects a host from an older build,
	// which would never send one.
	cardCtx, cardCancel := context.WithTimeout(ctx, partyHostWait)
	err = session.WaitCard(cardCtx)
	cardCancel()
	if err != nil {
		_ = session.Close()
		return err
	}
	m.adopt(session)
	return nil
}

// LockRoom closes the open room with a fresh secret and returns it. Calling it
// again on a locked room rotates the secret: the room address, and therefore
// the relay object, stays the same.
func (m *partyManager) LockRoom() (string, error) {
	room, err := m.hostRoom()
	if err != nil {
		return "", err
	}
	secret, err := party.NewSecret()
	if err != nil {
		return "", err
	}
	if _, err := room.Lock(secret); err != nil {
		return "", err
	}
	m.mu.Lock()
	m.secretAsked = false
	m.mu.Unlock()
	m.invalidate()
	return secret, nil
}

// UnlockRoom opens a locked room again, so the key alone is enough.
func (m *partyManager) UnlockRoom() error {
	room, err := m.hostRoom()
	if err != nil {
		return err
	}
	if err := room.Unlock(); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

// SubmitSecret lets a guest prove the secret and asks the host for a fresh
// anchor. A wrong value is refused locally by the room card.
func (m *partyManager) SubmitSecret(secret string) error {
	if m == nil {
		return errors.New("комната не создана")
	}
	m.mu.Lock()
	room := m.session
	m.mu.Unlock()
	if room == nil || room.Role != party.Guest {
		return errors.New("секрет вводит только гость комнаты")
	}
	if err := room.SubmitSecret(secret); err != nil {
		return err
	}
	m.mu.Lock()
	m.secretAsked = false
	m.mu.Unlock()
	m.invalidate()
	return nil
}

// hostRoom returns the open room if this client is its creator.
func (m *partyManager) hostRoom() (*party.Session, error) {
	if m == nil {
		return nil, errors.New("комната не создана")
	}
	m.mu.Lock()
	room := m.session
	m.mu.Unlock()
	if room == nil || room.Role != party.Host {
		return nil, party.ErrHostOnly
	}
	return room, nil
}

// takeSecretNotice reports once that the open room started asking for a secret,
// so the app can bring the guest to the secret form.
func (m *partyManager) takeSecretNotice() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	notice := m.secretNotice
	m.secretNotice = false
	return notice
}

// Delete closes the room and forgets the key.
func (m *partyManager) Delete() {
	if m == nil {
		return
	}
	m.mu.Lock()
	session := m.session
	m.session = nil
	m.state = party.State{}
	m.hasState = false
	m.started = false
	m.hostSeen = false
	m.hostEverSeen = false
	m.notice = ""
	m.secretAsked = false
	m.secretNotice = false
	// hostSession/hostMeta describe the running mpv session, not the room:
	// keep them so a room created later can publish the current episode.
	m.guestRunning = party.State{}
	m.guestDone = party.State{}
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
		return partySnapshot{Provider: party.Cloudflare}
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
		if card, ok := m.session.Card(); ok {
			snapshot.Locked = card.Locked
		}
		snapshot.NeedsSecret = m.session.NeedsSecret()
		snapshot.Secret = m.session.Secret()
		snapshot.HostName = m.session.HostName()
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

func (m *partyManager) adopt(session *party.Session) {
	m.mu.Lock()
	previous := m.session
	hostStop := m.hostStop
	guestStop := m.guestStop
	consumeStop := m.consumeStop
	m.session = session
	m.provider = session.Provider
	m.state = party.State{}
	m.hasState = false
	m.started = false
	m.hostSeen = false
	m.hostEverSeen = false
	m.notice = ""
	m.secretAsked = false
	m.secretNotice = false
	// hostSession/hostMeta describe the running mpv session, not the room: a
	// room created in the middle of an episode publishes it right away.
	m.guestRunning = party.State{}
	m.guestDone = party.State{}
	m.guestStarting = false
	stop := make(chan struct{})
	m.guestStop = stop
	m.hostStop = nil
	stopUpdates := make(chan struct{})
	m.consumeStop = stopUpdates
	running := m.hostSession
	if running != nil && (m.app == nil || m.app.playback == nil || m.app.playback.Current() != running) {
		// The player exited while no room was open, so the remembered session
		// is stale. A presence heartbeat must not wait for it.
		running = nil
		m.hostSession = nil
		m.hostMeta = hostMeta{}
	}
	m.mu.Unlock()

	closeStop(hostStop)
	closeStop(guestStop)
	closeStop(consumeStop)
	if previous != nil {
		_ = previous.Close()
	}

	go m.consume(session, stopUpdates)
	if session.Role == party.Guest {
		go m.guestLoop(session, stop)
	} else {
		// A host room starts its heartbeat even without a running title: the
		// guest must see that the host is still online while it waits.
		m.startHostLoop(session, running)
	}
	m.invalidate()
}

func (m *partyManager) consume(session *party.Session, stop chan struct{}) {
	for {
		var update party.Update
		select {
		case <-stop:
			return
		case update = <-session.Updates():
		}
		if !m.owns(session) {
			return
		}
		var control party.Control
		goneNotice := ""
		m.mu.Lock()
		switch update.Kind {
		case party.UpdateState:
			m.state = update.State
			m.hasState = true
			m.started = update.State.Started()
			m.hostSeen = true
			m.hostEverSeen = true
			m.notice = ""
			// A readable anchor proves the key generation in effect, so the
			// next rotation has to prompt again.
			m.secretAsked = false
		case party.UpdateCard:
			if session.Role == party.Guest {
				if session.NeedsSecret() {
					if !m.secretAsked {
						m.secretAsked = true
						m.secretNotice = true
					}
					// The anchor belongs to the previous key generation:
					// stop following it while the secret is unknown instead
					// of drifting away from the host.
					m.state = party.State{}
					m.hasState = false
					m.started = false
				} else {
					m.secretAsked = false
				}
			}
		case party.UpdateHostSeen:
			m.hostSeen = true
			m.hostEverSeen = true
		case party.UpdateHostLeft:
			// The grace window expired: the host had its chance to reconnect,
			// so the room is over and the guest returns to the join form.
			m.hostSeen = false
			m.notice = hostLostNotice
			if session.Role == party.Guest {
				goneNotice = hostLostNotice
			}
		case party.UpdateHostClosed:
			m.hostSeen = false
			m.notice = hostClosedNotice
			if session.Role == party.Guest {
				goneNotice = hostClosedNotice
			}
		case party.UpdateControl:
			control = update.Control
		case party.UpdateError:
			if update.Err != nil {
				if errors.Is(update.Err, party.ErrVersionMismatch) {
					m.notice = versionMismatchNotice
					if session.Role == party.Guest {
						goneNotice = versionMismatchNotice
					}
				} else {
					m.notice = update.Err.Error()
				}
			}
		}
		state := m.state
		started := m.started
		m.mu.Unlock()

		if goneNotice != "" {
			m.hostGone(goneNotice)
			return
		}
		if update.Kind == party.UpdateControl {
			m.applyControl(session, control)
			continue
		}

		m.invalidate()
		if session.Role != party.Guest {
			continue
		}
		// An empty anchor can mean a host restart, so the guest loop owns the
		// grace period before it stops local playback. Starting is only valid
		// for a readable anchor.
		if started {
			m.startGuest(state)
		}
	}
}

// The reasons a guest room disappears: the host closed it on purpose, the
// reconnect grace window expired, or the app versions turned out to differ.
const (
	hostClosedNotice      = "Создатель закрыл комнату"
	hostLostNotice        = "Создатель комнаты вышел"
	versionMismatchNotice = "Разные версии приложения — обновите yoanime"
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
func (m *partyManager) applyControl(session *party.Session, control party.Control) {
	if m.app == nil || m.app.playback == nil || !m.SharedControl() {
		return
	}
	m.mu.Lock()
	running := m.hostSession
	owned := m.session == session && session.Role == party.Host
	m.mu.Unlock()
	if !owned || running == nil || m.app.playback.Current() != running {
		return
	}
	var err error
	switch control.Action {
	case party.ControlPause:
		err = running.SetPaused(true)
	case party.ControlPlay:
		err = running.SetPaused(false)
	case party.ControlSeek:
		err = running.SeekTo(control.Position)
	default:
		return
	}
	if err != nil {
		log.Warn("party control apply failed", "action", control.Action, "error", err)
		return
	}
	log.Info("party guest control applied", "action", control.Action, "position", control.Position)
	if state, ok := m.hostState(); ok {
		if err := session.Publish(state); err != nil {
			log.Warn("party host publish failed", "provider", session.Provider, "error", err)
		}
	}
}

// stopGuest ends the mpv session a guest is following, if there is one.
func (m *partyManager) stopGuest() {
	m.mu.Lock()
	running := m.guestRunning.Started()
	m.guestRunning = party.State{}
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

func (m *partyManager) owns(session *party.Session) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session == session
}

// startGuest launches mpv for a guest when the host opens another title or
// episode. Duplicate calls for the same media are ignored.
func (m *partyManager) startGuest(state party.State) {
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
		// The host moved to another episode of the same title and voice. The
		// local player already carries a resolved window of that voice (mpv
		// resolves Kinopoisk episodes too slowly for a restart chase), so
		// follow with a playlist jump and let the drift loop find the position.
		if current.Matches(state.TitleSlug, state.Voice) {
			if err := current.JumpTo(state.Episode); err == nil {
				log.Info("party guest jumped to host episode", "episode", state.Episode)
				m.mu.Lock()
				m.guestRunning = state
				m.mu.Unlock()
				return
			}
		}
	}
	m.mu.Lock()
	if m.app.playback.Current() == nil && !m.guestStarting {
		m.guestRunning = party.State{}
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
	log.Info("party guest starting playback", "slug", state.TitleSlug, "voice", state.Voice, "episode", state.Episode, "position", state.Position)

	go func() {
		err := m.app.startPartyPlayback(state)
		m.mu.Lock()
		m.guestStarting = false
		if err != nil {
			// Keep the retry backoff consistent with a failure reported through
			// the playback result channel: by the time the UI drains that
			// result, guestRunning would already be empty and the failed media
			// would be retried on the very next heartbeat.
			if m.guestRunning.Started() {
				m.guestDone = m.guestRunning
				m.guestDoneAt = time.Now()
			}
			m.guestRunning = party.State{}
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
	m.guestRunning = party.State{}
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
	m.guestRunning = party.State{}
	m.guestStarting = false
	m.notice = "Плеер закрылся, пробую снова"
	m.mu.Unlock()
	m.invalidate()
}

// guestLoop keeps the local mpv close to the extrapolated host position. It
// also watches for input the person makes in mpv (pause, seek) and forwards it
// to the host when shared control is on.
// guestControlFromInput recognises the pause or seek a person just made in the
// guest's mpv. The baseline is the previous tick's picture, not the host
// anchor: the drift loop corrects the anchor itself, so anything larger than a
// network stall is deliberate input. The paused state is part of the baseline
// so scrubbing a paused guest reaches the host too.
func guestControlFromInput(now, lastSample time.Time, lastPosition float64, lastPaused bool, position float64, paused bool, settleUntil time.Time) (string, float64, bool) {
	if paused != lastPaused {
		if paused {
			return party.ControlPause, 0, true
		}
		return party.ControlPlay, 0, true
	}
	if lastSample.IsZero() || now.Before(settleUntil) {
		return "", 0, false
	}
	expected := lastPosition
	if !lastPaused {
		expected += now.Sub(lastSample).Seconds()
	}
	if position == lastPosition || math.Abs(position-expected) <= partyUserSeekThreshold {
		return "", 0, false
	}
	return party.ControlSeek, position, true
}

func (m *partyManager) guestLoop(session *party.Session, stop chan struct{}) {
	ticker := time.NewTicker(partyGuestTick)
	defer ticker.Stop()
	rate := 1.0
	var lastPosition float64
	var lastSample time.Time
	var lastPaused bool
	var holdUntil time.Time
	var settleUntil time.Time
	var seeded bool
	var mismatchLogged bool
	// anchorAt is when the last *new* host anchor arrived. A half-open socket
	// stops delivering anchors without ever erroring; the refresh below pulls
	// the player back into the room. It must work while paused too: a paused
	// guest that loses the socket otherwise never hears the host resume.
	var anchorAt time.Time
	var lastAnchor int64
	var lastRefresh time.Time
	var emptySince time.Time
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
			emptySince = time.Time{}
			continue
		}
		state, ok := session.State()
		now := time.Now()
		if !ok || !state.Started() {
			// The host may be restarting mpv. Give the new session a short
			// window to publish its first anchor before ending local playback.
			if emptySince.IsZero() {
				emptySince = now
			}
			if now.Sub(emptySince) >= partyGuestStopGrace {
				log.Info("party host playback stayed closed, stopping guest")
				m.stopGuest()
				emptySince = time.Time{}
				seeded = false
			}
			rate = 1
			continue
		}
		emptySince = time.Time{}
		if anchorAt.IsZero() {
			// A host that joined but never published still needs a way to be
			// poked; measure staleness from the first observed state.
			anchorAt = now
		}
		if state.Anchor != lastAnchor {
			lastAnchor = state.Anchor
			anchorAt = now
		}
		number, _ := current.Episode()
		if number == "" || number != state.Episode {
			// mpv is still opening the file or the host moved on; the drift
			// numbers are meaningless until file-loaded arrives.
			if !mismatchLogged {
				log.Info("party guest waiting for episode sync", "host", state.Episode, "player", number)
				mismatchLogged = true
			}
			rate = 1
			seeded = false
			continue
		}
		mismatchLogged = false
		// A guest that stopped hearing from the host for a long time is
		// running on a stale anchor: extrapolation still tracks the picture,
		// but a pause or seek from the host can no longer arrive. Ask once,
		// then back off — the hello makes the host republish immediately.
		stale := now.Sub(anchorAt) >= partyStaleAnchor && now.Sub(lastRefresh) >= partyRefreshBackoff
		if stale {
			log.Info("party guest anchor feed stale, requesting a fresh anchor", "since", now.Sub(anchorAt).Round(time.Second), "episode", number)
			if err := session.RequestRefresh(); err == nil {
				lastRefresh = now
			}
		}
		position := current.LivePosition()
		paused := current.Paused()
		// The host's anchor owns the shared-control decision. Read it from the
		// same state snapshot as the sync target: m.state is updated by the UI
		// consumer and may lag behind (or miss a dropped notification), which
		// used to leave a guest unable to control a room where control was on.
		controlAllowed := guestControlAllowed(state, m.SharedControl())
		if !seeded {
			seeded = true
		} else if controlAllowed {
			// Between two ticks only a person can change the player: the drift
			// loop acts below and records its own corrections in the baseline.
			if action, value, ok := guestControlFromInput(now, lastSample, lastPosition, lastPaused, position, paused, settleUntil); ok {
				log.Info("party guest sent control", "action", action, "position", value)
				if err := session.SendControl(action, value); err == nil {
					holdUntil = now.Add(partyControlHold)
				} else {
					log.Warn("party guest control send failed", "action", action, "error", err)
					_ = session.RequestRefresh()
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

		if !state.Playing {
			if !current.Paused() {
				log.Info("party guest pausing to host")
				_ = current.SetPaused(true)
			}
			if now.After(settleUntil) && math.Abs(position-state.Position) > 0.8 {
				log.Info("party guest seek to paused host", "host", state.Position, "player", position)
				if err := current.SeekTo(state.Position); err == nil {
					tracked = state.Position
					settleUntil = now.Add(partySeekSettle)
				}
			}
			if rate != 1 {
				_ = current.SetSpeed(1)
				rate = 1
			}
		} else {
			if current.Paused() {
				log.Info("party guest resuming to host")
				_ = current.SetPaused(false)
			}
			want, _, ok := session.ExpectedPosition(now)
			if !ok {
				lastPaused = current.Paused()
				lastPosition = tracked
				continue
			}
			log.Debug("party guest tick", "episode", number, "player", position, "want", want, "playing", state.Playing, "paused", paused)
			drift := position - want
			switch {
			case now.Before(settleUntil):
				// Our previous seek is still becoming visible. Leave the
				// picture alone until mpv catches up.
			case math.Abs(drift) > partySeekThreshold:
				target := want + partySeekLead
				log.Info("party guest seek", "to", target, "want", want, "player", position)
				if err := current.SeekTo(target); err == nil {
					tracked = target
					settleUntil = now.Add(partySeekSettle)
				}
				if rate != 1 {
					_ = current.SetSpeed(1)
					rate = 1
				}
			case math.Abs(drift) > partySpeedThreshold:
				next := clampRate(1 - drift*partySpeedGain)
				if math.Abs(next-rate) > 0.004 {
					log.Info("party guest speed", "rate", next, "want", want, "player", position)
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
	log.Info("party host attached playback", "slug", title.Slug, "voice", voice, "episode", episode)
	if room == nil || room.Role != party.Host {
		return
	}
	m.startHostLoop(room, session)
}

// startHostLoop (re)connects the heartbeat of an existing room. A nil session
// means the room is open but no title is playing: the host still sends empty
// presence anchors, otherwise a guest would count the host as gone and leave
// while it is simply waiting on the create screen.
func (m *partyManager) startHostLoop(room *party.Session, session *playback.Session) {
	m.mu.Lock()
	oldStop := m.hostStop
	stop := make(chan struct{})
	m.hostStop = stop
	m.mu.Unlock()

	closeStop(oldStop)
	room.SetSource(func() (party.State, bool) { return m.hostState() })
	go m.hostLoop(room, stop, session)
	// Publish once immediately so a guest does not wait for the first tick.
	if state, ok := m.hostState(); ok {
		if err := room.Publish(state); err != nil {
			log.Warn("party host publish failed", "provider", room.Provider, "error", err)
		}
	} else if err := room.Publish(party.State{}); err != nil {
		log.Warn("party host publish failed", "provider", room.Provider, "error", err)
	}
	m.invalidate()
}

// hostState snapshots the running mpv session into a room anchor.
func (m *partyManager) hostState() (party.State, bool) {
	m.mu.Lock()
	session := m.hostSession
	meta := m.hostMeta
	m.mu.Unlock()
	if session == nil || meta.Slug == "" {
		return party.State{}, false
	}
	number, index := session.Episode()
	if number == "" {
		number = meta.Episode
	}
	control := m.SharedControl()
	return party.State{
		TitleSlug:      meta.Slug,
		TitleName:      meta.Name,
		Voice:          meta.Voice,
		Episode:        number,
		EpisodeIndex:   index,
		Playing:        !session.Paused(),
		Position:       session.LivePosition(),
		ControlEnabled: &control,
	}, true
}

func (m *partyManager) hostLoop(room *party.Session, stop chan struct{}, session *playback.Session) {
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
			// A newer loop may already own this room. Only the loop that is
			// still current may clear the source.
			if m.releaseHostLoop(stop) {
				room.SetSource(nil)
			}
			return
		case now = <-ticker.C:
		}
		if m.app == nil || m.app.playback == nil {
			if m.releaseHostLoop(stop) {
				room.SetSource(nil)
			}
			return
		}

		if session != nil {
			if current := m.app.playback.Current(); current != session {
				// mpv exited: clear the room so guests show the waiting screen
				// instead of extrapolating a finished episode forever. Keep
				// publishing empty presence anchors so those guests stay in
				// the room while the host is still online.
				m.mu.Lock()
				replaced := m.hostSession != session
				if !replaced {
					m.hostSession = nil
				}
				m.mu.Unlock()
				if replaced {
					// A newer playback already took over the room.
					return
				}
				if err := room.Publish(party.State{}); err != nil {
					log.Warn("party host publish failed", "provider", room.Provider, "error", err)
				}
				lastPublish = now
				first = false
				session = nil
				m.invalidate()
				continue
			}
		} else {
			// A new title may have started between ticks; its own loop is
			// (or is about to be) installed, so do not publish an empty state
			// over the fresh anchor.
			m.mu.Lock()
			running := m.hostSession != nil
			m.mu.Unlock()
			if running {
				continue
			}
			if first || now.Sub(lastPublish) >= partyHeartbeatIdle {
				if err := room.Publish(party.State{}); err == nil {
					lastPublish = now
				} else {
					log.Warn("party host publish failed", "provider", room.Provider, "error", err)
				}
				first = false
			}
			continue
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
			log.Debug("party host anchor", "episode", state.Episode, "position", state.Position, "playing", state.Playing, "voice", state.Voice)
			if err := room.Publish(state); err != nil {
				log.Warn("party host publish failed", "provider", room.Provider, "error", err)
			} else {
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

// releaseHostLoop reports whether stop is still the room's active heartbeat
// and, if so, forgets it. An older loop must not detach the source installed by
// its replacement.
func (m *partyManager) releaseHostLoop(stop chan struct{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hostStop != stop {
		return false
	}
	m.hostStop = nil
	return true
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
