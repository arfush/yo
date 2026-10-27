package party

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/arfush/yoanime/log"
)

const (
	reconnectMin = time.Second
	reconnectMax = 15 * time.Second
	// helloThrottle keeps a burst of joining guests from making the host
	// publish once per guest; one fresh anchor answers all of them. The card is
	// answered without the throttle, because it is tiny and a guest cannot
	// verify a typed secret before it arrives.
	helloThrottle = 700 * time.Millisecond
	updatesBuffer = 8
)

// A guest treats the room as abandoned when no host message arrives for this
// long. The host publishes every ten seconds while paused, so two missed
// anchors in a row fit inside the window while a dead process does not.
// Cloudflare usually reports the same event earlier through bye; this covers
// Supabase and a host that hangs instead of closing its socket.
//
// A reported disconnect is not final: the host keeps a grace window to come
// back after a Wi-Fi hiccup, and guests keep watching the last anchor meanwhile.
// Only the explicit close message or an expired grace window ends the room.
//
// All three values are variables so tests can shorten the waits.
var (
	hostSilence   = 25 * time.Second
	hostGrace     = 30 * time.Second
	hostWatchTick = time.Second
)

// ErrVersionMismatch reports a room whose host speaks another protocol
// version. A guest leaves instead of applying anchors it may misread.
var ErrVersionMismatch = errors.New("разные версии приложения — обновите yoanime")

// ErrRoomNotFound reports a reachable relay that has no host in the room. A
// guest uses it before opening the waiting screen, so a mistyped key fails
// immediately instead of waiting for the regular host-silence timeout.
var ErrRoomNotFound = errors.New("комната не найдена — проверьте ключ")

// ErrHostOnly reports a guest that tried to use a host action.
var ErrHostOnly = errors.New("действие доступно только создателю комнаты")

// Session is one joined room. It owns the WebSocket, reconnects on its own and
// keeps the newest host anchor plus the clock estimate.
type Session struct {
	// Key is the room key the two people shared. It never reaches the relay:
	// the transport dials Address instead.
	Key      string
	Address  string
	Provider Provider
	Role     Role

	cfg Config

	mu         sync.Mutex
	name       string
	conn       transport
	state      State
	hasState   bool
	hostSeen   bool
	hostName   string
	lastHost   time.Time
	hostLostAt time.Time
	hostGone   bool
	closed     bool
	lastErr    error
	lastHello  time.Time
	source     func() (State, bool)
	clock      Clock

	// The content key is derived once per card and then reused for every
	// frame. Card, epoch and secret describe the key generation currently in
	// effect, so a rotation can be recognised and a stale frame dropped.
	key         []byte
	card        Card
	hasCard     bool
	needsSecret bool
	secret      string

	updates chan Update

	cancel context.CancelFunc
}

// Create allocates a fresh room key and opens it as the host. The key carries
// the relay, so a guest joins the same backend without picking one by hand.
func Create(ctx context.Context, cfg Config, provider Provider, name string) (*Session, error) {
	key, err := NewKey(provider)
	if err != nil {
		return nil, err
	}
	return open(ctx, cfg, provider, key, name, Host)
}

// Join reads the relay from the key, normalises it and opens the room as a
// guest.
func Join(ctx context.Context, cfg Config, key, name string) (*Session, error) {
	normalized, err := NormalizeKey(key)
	if err != nil {
		return nil, err
	}
	provider, err := ProviderFromKey(normalized)
	if err != nil {
		return nil, err
	}
	return open(ctx, cfg, provider, normalized, name, Guest)
}

func open(ctx context.Context, cfg Config, provider Provider, key, name string, role Role) (*Session, error) {
	if !provider.Valid() || !cfg.Configured(provider) {
		return nil, ErrNotConfigured
	}
	// The key is the only source of the relay for a guest, and a host must not
	// mint a key for another backend, so a mismatch here is a programming bug.
	if keyProvider, err := ProviderFromKey(key); err != nil || keyProvider != provider {
		return nil, ErrInvalidKey
	}
	address, err := Address(key)
	if err != nil {
		return nil, err
	}
	session := &Session{
		Key:      key,
		Address:  address,
		Provider: provider,
		Role:     role,
		name:     name,
		cfg:      cfg,
		updates:  make(chan Update, updatesBuffer),
	}
	if role == Host {
		// A new room starts public: the first card is derived from the room key
		// alone, so even a public room has exactly one code path.
		card, contentKey, err := newCard(key, "", false, 1)
		if err != nil {
			return nil, err
		}
		session.card = card
		session.hasCard = true
		session.key = contentKey
	}
	log.Info("opening watch-party room", "provider", provider, "role", role)
	conn, err := session.dial(ctx)
	if err != nil {
		log.Error("cannot open watch-party room", "provider", provider, "role", role, "error", err)
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	session.cancel = cancel
	// The caller publishes its first anchor as soon as Create or Join returns,
	// often before the reader goroutine has run. Attach the socket here so that
	// publish does not lose the race and report a missing connection.
	session.mu.Lock()
	session.conn = conn
	session.mu.Unlock()
	go session.run(runCtx, conn)
	if role == Guest {
		go session.watchHost(runCtx)
	}
	return session, nil
}

// Updates reports room events for the UI. It never blocks: the newest state is
// also available through State, so a dropped notification is harmless.
func (s *Session) Updates() <-chan Update { return s.updates }

// State returns the newest host anchor.
func (s *Session) State() (State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.hasState
}

// Name returns the nickname this client sends to the other side.
func (s *Session) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// SetName replaces the nickname in a room that is already open. The next
// anchor carries it, so a guest that renames itself does not have to rejoin.
func (s *Session) SetName(name string) {
	s.mu.Lock()
	s.name = name
	s.mu.Unlock()
}

// HostName returns the host nickname learned from the newest anchor.
func (s *Session) HostName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostName
}

// Card returns the current room header. A guest uses it to decide whether a
// secret prompt is needed before the first anchor arrives.
func (s *Session) Card() (Card, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.card, s.hasCard
}

// NeedsSecret reports that the room is locked and this client has not found a
// secret that opens it yet.
func (s *Session) NeedsSecret() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsSecret
}

// Secret returns the secret currently in effect on this side. The host shows it
// to share; a guest only ever knows the value it typed.
func (s *Session) Secret() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secret
}

// SubmitSecret derives the content key from a typed secret and asks the host
// for a fresh anchor. A wrong secret is refused locally by the card check, so
// the host never learns that somebody tried.
func (s *Session) SubmitSecret(secret string) error {
	if s.Role != Guest {
		return ErrHostOnly
	}
	normalized, err := NormalizeSecret(secret)
	if err != nil || normalized == "" {
		return ErrInvalidSecret
	}
	s.mu.Lock()
	card, has := s.card, s.hasCard
	s.mu.Unlock()
	if !has {
		return ErrNoCard
	}
	key, ok := card.Key(s.Key, normalized)
	if !ok {
		return ErrWrongSecret
	}
	s.mu.Lock()
	s.key = key
	s.secret = normalized
	s.needsSecret = false
	s.mu.Unlock()
	log.Info("watch-party secret accepted", "provider", s.Provider, "role", s.Role)
	s.notify(Update{Kind: UpdateCard})
	// The host may have published anchors the guest could not read; ask for one
	// sealed with the key that was just accepted.
	return s.write(frame{Type: messageHello})
}

// Lock closes the room with a fresh secret and returns it. Rotating a secret on
// an already locked room is the same operation with a new value.
func (s *Session) Lock(secret string) (string, error) {
	normalized, err := NormalizeSecret(secret)
	if err != nil || normalized == "" {
		return "", ErrInvalidSecret
	}
	if err := s.changeSecret(normalized, true); err != nil {
		return "", err
	}
	return normalized, nil
}

// Unlock opens a locked room again. Guests need no action: the room key alone
// derives the new key, and the card tells them that the secret is gone.
func (s *Session) Unlock() error {
	return s.changeSecret("", false)
}

func (s *Session) changeSecret(secret string, locked bool) error {
	if s.Role != Host {
		return ErrHostOnly
	}
	s.mu.Lock()
	epoch := s.card.Epoch + 1
	s.mu.Unlock()

	card, key, err := newCard(s.Key, secret, locked, epoch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.card = card
	s.hasCard = true
	s.key = key
	s.secret = secret
	s.needsSecret = false
	state, has := s.state, s.hasState
	s.mu.Unlock()

	// The card goes first: a guest cannot read the anchor that follows without
	// the new salt, and it would drop the frame instead of re-syncing.
	s.sendCard()
	s.notify(Update{Kind: UpdateCard})
	log.Info("watch-party lock changed", "provider", s.Provider, "role", s.Role, "locked", locked)
	if has {
		_ = s.Publish(state)
	}
	return nil
}

// HostSeen reports whether the host is currently answering. It turns false as
// soon as the relay reports a disconnect, even though the grace window keeps
// the room alive.
func (s *Session) HostSeen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostSeen
}

// WaitHost blocks until the host proves it is in the room. A guest calls it
// right after Join to distinguish a real room from a key the relay accepted but
// nobody is hosting. The relay's welcome message is enough; the host does not
// have to publish a playback anchor for an empty room to count as existing.
//
// A host throttles answers to hello bursts, so a probe repeats its hello once a
// second. Without that retry two guests joining together on Supabase would make
// the second one time out even though the room is alive.
func (s *Session) WaitHost(ctx context.Context) error {
	if s.Role != Guest {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	lastHello := time.Now()
	for {
		if s.HostSeen() {
			return nil
		}
		if s.HostGone() {
			return ErrRoomNotFound
		}
		if err := s.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrRoomNotFound
			}
			return ctx.Err()
		case <-ticker.C:
			if time.Since(lastHello) >= time.Second {
				lastHello = time.Now()
				_ = s.write(frame{Type: messageHello, Role: s.Role})
			}
		}
	}
}

// WaitCard blocks until the host card arrives. A guest cannot know whether a
// secret is required without it, so Join waits for the card before the UI
// decides which screen to open. A host that never sends one is necessarily a
// build from before protocol 2.0.
func (s *Session) WaitCard(ctx context.Context) error {
	if s.Role != Guest {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	lastHello := time.Now()
	for {
		s.mu.Lock()
		has, gone := s.hasCard, s.hostGone
		s.mu.Unlock()
		if has {
			return nil
		}
		if gone {
			return ErrRoomNotFound
		}
		if err := s.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrVersionMismatch
			}
			return ctx.Err()
		case <-ticker.C:
			if time.Since(lastHello) >= time.Second {
				lastHello = time.Now()
				_ = s.write(frame{Type: messageHello, Role: s.Role})
			}
		}
	}
}

// HostGone reports whether the room is over: the host closed it explicitly or
// the reconnect grace window expired.
func (s *Session) HostGone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostGone
}

// Err returns the last relay problem.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// SetSource lets a host answer a late joiner with a fresh anchor instead of the
// last heartbeat, which would be a few seconds behind.
func (s *Session) SetSource(source func() (State, bool)) {
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
}

// Publish sends the host anchor. The anchor timestamp is stamped here so it
// matches the moment the packet leaves the process.
func (s *Session) Publish(state State) error {
	if s.Role != Host {
		return errors.New("публиковать состояние может только создатель комнаты")
	}
	state.Anchor = time.Now().UnixMilli()
	s.mu.Lock()
	s.state = state
	s.hasState = true
	s.mu.Unlock()
	s.notify(Update{Kind: UpdateState, State: state})
	data, err := s.sealPayload(payload{Name: s.Name(), State: &state})
	if err != nil {
		return err
	}
	return s.write(frame{Type: messageState, Data: data})
}

// SendControl forwards a playback command to the host. Only guests use it;
// applying the command is the host's decision, because the shared-control
// setting lives there.
func (s *Session) SendControl(action string, position float64) error {
	if s.Role != Guest {
		return errors.New("управлять воспроизведением может только гость")
	}
	control := Control{Action: action, Position: position}
	if !control.Valid() {
		return errors.New("неизвестное действие комнаты")
	}
	data, err := s.sealPayload(payload{Name: s.Name(), Control: &control})
	if err != nil {
		return err
	}
	return s.write(frame{Type: messageControl, Data: data})
}

// ExpectedPosition returns where the host expects playback to be right now.
func (s *Session) ExpectedPosition(now time.Time) (float64, State, bool) {
	state, ok := s.State()
	if !ok {
		return 0, State{}, false
	}
	return state.PositionAt(s.clock.HostNow(now)), state, true
}

// RequestRefresh asks the host for a fresh anchor. A guest writes no other
// periodic traffic, so a half-open socket (NAT dropped the mapping, the server
// still has the peer) never surfaces on its own: the anchor feed dies silently
// and the player keeps extrapolating past a host pause or seek. The guest loop
// calls this only when no fresh anchor has arrived for a while, and the hello
// is tiny, so the relay quota sees nothing periodic.
func (s *Session) RequestRefresh() error {
	if s == nil {
		return nil
	}
	if s.Role != Guest {
		return errors.New("запрашивать обновление может только гость")
	}
	return s.write(frame{Type: messageHello, Role: s.Role})
}

// Close leaves the room and stops reconnecting.
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	conn := s.conn
	s.mu.Unlock()
	log.Info("closing watch-party room", "provider", s.Provider, "role", s.Role)

	if conn != nil && s.Role == Host {
		// Only the host's departure ends the room. A guest saying bye would be
		// broadcast on Supabase and make the remaining guests think the host
		// left; the relay notices a guest socket closing on its own.
		//
		// close means "the host chose to leave", so guests must not wait for a
		// reconnect. bye is kept for Cloudflare, which also synthesises it when
		// a socket drops without warning.
		_ = s.write(frame{Type: messageClose})
		_ = s.write(frame{Type: messageBye})
	}

	s.mu.Lock()
	s.closed = true
	s.conn = nil
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.notify(Update{Kind: UpdateClosed})
	return nil
}

func (s *Session) dial(ctx context.Context) (transport, error) {
	switch s.Provider {
	case Cloudflare:
		return dialCloudflare(ctx, s.cfg, s.Address, s.Role)
	case Supabase:
		return dialSupabase(ctx, s.cfg, s.Address, s.Role)
	}
	return nil, ErrNotConfigured
}

func (s *Session) run(ctx context.Context, first transport) {
	conn := first
	backoff := reconnectMin
	for {
		if ctx.Err() != nil {
			return
		}
		if conn == nil {
			var err error
			conn, err = s.dial(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Warn("watch-party reconnect failed", "provider", s.Provider, "role", s.Role, "error", err)
				s.reportError(err)
				if !sleep(ctx, backoff) {
					return
				}
				backoff = nextBackoff(backoff)
				continue
			}
		}
		backoff = reconnectMin
		s.setConn(conn)
		s.greet()
		err := conn.run(ctx, s.handle)
		s.setConn(nil)
		conn.close()
		conn = nil
		if ctx.Err() != nil {
			return
		}
		s.reportError(err)
		if !sleep(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

// greet re-announces the room after a (re)connect. Hosts push the card and an
// anchor so guests learn the host is present even before playback starts;
// guests ask for both.
func (s *Session) greet() {
	if s.Role == Host {
		s.sendCard()
		state, ok := s.currentState()
		if !ok {
			// An empty state still proves to a waiting guest that the host is
			// in the room.
			state = State{}
		}
		_ = s.Publish(state)
		return
	}
	_ = s.write(frame{Type: messageHello, Role: s.Role})
}

// sendCard republishes the current room header. It is cheap and answered on
// every hello, so a guest that missed a rotation can still catch up.
func (s *Session) sendCard() {
	s.mu.Lock()
	card, has := s.card, s.hasCard
	s.mu.Unlock()
	if !has {
		return
	}
	_ = s.write(frame{Type: messageCard, Card: &card})
}

func (s *Session) handle(msg frame) {
	received := time.Now().UnixMilli()
	// welcome and error come from the relay, which has no protocol version.
	if msg.Type != messageWelcome && msg.Type != messageError &&
		msg.Version != "" && msg.Version != ProtocolVersion {
		// A guest cannot know the host version before the first host frame. A
		// card or a state is enough to tell: leave before storing an anchor
		// that this build may misinterpret.
		if s.Role == Guest && (msg.Type == messageState || msg.Type == messageCard) {
			s.rejectVersion()
		}
		return
	}
	switch msg.Type {
	case messageCard:
		// A host is the source of the card. Supabase broadcasts every frame to
		// every participant, so without this guard a guest could replace the
		// host's own key with a forged card.
		if s.Role != Guest {
			return
		}
		s.acceptCard(msg.Card)
	case messageState:
		// Likewise, a host must never apply an anchor from a guest.
		if s.Role != Guest {
			return
		}
		if msg.Data == "" {
			return
		}
		// Any frame with the right version proves the host process is alive,
		// even when the guest still lacks the secret to read it. Without this
		// a locked room would look abandoned while it waits for the password.
		s.markHostAlive()
		body, err := s.openPayload(msg.Data)
		if err != nil || body.State == nil {
			return
		}
		s.applyState(*body.State, body.Name, received)
	case messageWelcome:
		if msg.Host == nil {
			return
		}
		s.applyWelcome(*msg.Host)
	case messageClose:
		if s.Role != Guest {
			return
		}
		s.mu.Lock()
		s.hostSeen = false
		s.hostLostAt = time.Time{}
		s.hostGone = true
		s.mu.Unlock()
		s.notify(Update{Kind: UpdateHostClosed})
	case messageHello:
		if s.Role == Host {
			s.sendCard()
			s.publishSoon()
		}
	case messageControl:
		if s.Role != Host {
			return
		}
		body, err := s.openPayload(msg.Data)
		if err != nil || body.Control == nil || !body.Control.Valid() {
			return
		}
		s.notify(Update{Kind: UpdateControl, Control: *body.Control})
	case messageBye:
		// bye is also synthesised by Cloudflare when a socket drops, so it
		// starts the reconnect grace instead of ending the room. The explicit
		// close message above is the only immediate ending.
		if s.Role != Guest {
			return
		}
		s.mu.Lock()
		if s.hostSeen {
			s.hostSeen = false
			s.hostLostAt = time.Now()
		}
		s.mu.Unlock()
	case messageError:
		text := msg.Error
		if text == "" {
			text = "ошибка комнаты"
		}
		s.reportError(errors.New(text))
	}
}

// acceptCard adopts a room header. The derivation runs on the reader goroutine
// once per card change; the result is cached, so no anchor ever pays for it.
func (s *Session) acceptCard(card *Card) {
	if card == nil || !card.Valid() {
		return
	}
	s.mu.Lock()
	if s.hasCard && card.Epoch <= s.card.Epoch {
		// A late duplicate must not throw away a key that already works.
		s.mu.Unlock()
		return
	}
	// An unlocked room uses the room key alone, so every guest can open it
	// without typing anything. A locked room starts from the secret this guest
	// already proved, if any.
	secret := s.secret
	if !card.Locked {
		secret = ""
	}
	s.mu.Unlock()

	key, ok := card.Key(s.Key, secret)

	s.mu.Lock()
	s.card = *card
	s.hasCard = true
	first := false
	if ok {
		s.key = key
		s.secret = secret
		s.needsSecret = false
		first = s.markHostLocked()
	} else {
		s.key = nil
		s.secret = ""
		s.needsSecret = card.Locked
	}
	s.mu.Unlock()
	s.notify(Update{Kind: UpdateCard})
	if first {
		s.notify(Update{Kind: UpdateHostSeen})
	}
}

// markHostLocked records that the host answered and reports whether this is the
// first frame after a gap. The caller must hold s.mu. A host that was gone may
// have restarted on another machine, so its wall clock is not comparable with
// the old samples and the estimate starts over.
func (s *Session) markHostLocked() bool {
	first := !s.hostSeen
	if first {
		s.clock.Reset()
	}
	s.hostSeen = true
	s.lastHost = time.Now()
	s.hostLostAt = time.Time{}
	s.hostGone = false
	return first
}

// markHostAlive records that the host process answered, independently of
// whether this client can decrypt the frame.
func (s *Session) markHostAlive() {
	if s.Role != Guest {
		return
	}
	s.mu.Lock()
	first := s.markHostLocked()
	s.mu.Unlock()
	if first {
		s.notify(Update{Kind: UpdateHostSeen})
	}
}

func (s *Session) applyState(state State, hostName string, received int64) {
	s.mu.Lock()
	first := false
	if s.Role == Guest {
		first = s.markHostLocked()
	}
	s.state = state
	s.hasState = true
	if hostName != "" {
		s.hostName = hostName
	}
	s.mu.Unlock()
	s.clock.Add(state.Anchor, received)
	if first {
		s.notify(Update{Kind: UpdateHostSeen})
	}
	s.notify(Update{Kind: UpdateState, State: state})
}

func (s *Session) applyWelcome(present bool) {
	s.mu.Lock()
	previous := s.hostSeen
	if present {
		// A welcome can arrive after a reconnect while the host restarted in
		// the meantime, so the next anchor starts a fresh clock estimate.
		s.markHostLocked()
	} else {
		s.hostSeen = false
		if previous {
			s.hostLostAt = time.Now()
		}
	}
	s.mu.Unlock()
	if present && !previous {
		s.notify(Update{Kind: UpdateHostSeen})
	}
}

// watchHost notices a host that stopped answering. The relay protocol has no
// disconnect event on Supabase, so a guest would otherwise wait forever on the
// last anchor; Cloudflare also benefits when a host freezes instead of closing
// its socket. A disconnect starts the grace window rather than ending the room,
// so a short network drop does not throw the guests out of the episode.
func (s *Session) watchHost(ctx context.Context) {
	ticker := time.NewTicker(hostWatchTick)
	defer ticker.Stop()
	for {
		var now time.Time
		select {
		case <-ctx.Done():
			return
		case now = <-ticker.C:
		}
		s.mu.Lock()
		if s.Role == Guest && !s.closed && s.hostSeen && !s.lastHost.IsZero() &&
			now.Sub(s.lastHost) >= hostSilence {
			s.hostSeen = false
			s.hostLostAt = now
		}
		expired := s.Role == Guest &&
			!s.closed &&
			!s.hostGone &&
			!s.hostLostAt.IsZero() &&
			now.Sub(s.hostLostAt) >= hostGrace
		if expired {
			s.hostGone = true
			s.hostLostAt = time.Time{}
		}
		s.mu.Unlock()
		if expired {
			s.notify(Update{Kind: UpdateHostLeft})
		}
	}
}

// publishSoon answers a hello, but not more than once per throttle window.
func (s *Session) publishSoon() {
	s.mu.Lock()
	recent := time.Since(s.lastHello) < helloThrottle
	s.lastHello = time.Now()
	s.mu.Unlock()
	if recent {
		return
	}
	state, ok := s.currentState()
	if !ok {
		state = State{}
	}
	_ = s.Publish(state)
}

func (s *Session) currentState() (State, bool) {
	s.mu.Lock()
	source := s.source
	state := s.state
	has := s.hasState
	s.mu.Unlock()
	if source != nil {
		if fresh, ok := source(); ok {
			return fresh, true
		}
	}
	return state, has
}

func (s *Session) setConn(conn transport) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
}

func (s *Session) write(msg frame) error {
	msg.Version = ProtocolVersion
	s.mu.Lock()
	conn := s.conn
	closed := s.closed
	s.mu.Unlock()
	if closed || conn == nil {
		return errNotConnected
	}
	return conn.write(msg)
}

// sealPayload encrypts the room body with the current content key. A guest
// without a secret cannot publish, which is exactly the intended behaviour.
func (s *Session) sealPayload(body payload) (string, error) {
	s.mu.Lock()
	key := s.key
	s.mu.Unlock()
	if len(key) == 0 {
		return "", ErrNoCard
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return sealContent(key, raw)
}

func (s *Session) openPayload(data string) (payload, error) {
	s.mu.Lock()
	key := s.key
	s.mu.Unlock()
	if len(key) == 0 {
		return payload{}, ErrNoCard
	}
	raw, err := openContent(key, data)
	if err != nil {
		return payload{}, err
	}
	var body payload
	if err := json.Unmarshal(raw, &body); err != nil {
		return payload{}, err
	}
	return body, nil
}

// rejectVersion stops a guest that reached a room speaking another protocol.
// The error is reported before closing so the UI can explain the mismatch
// instead of showing a generic disconnect.
func (s *Session) rejectVersion() {
	s.reportError(ErrVersionMismatch)
	_ = s.Close()
}

func (s *Session) notify(update Update) {
	select {
	case s.updates <- update:
	default:
	}
}

func (s *Session) reportError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	s.mu.Lock()
	s.lastErr = err
	s.mu.Unlock()
	log.Warn("watch-party relay error", "provider", s.Provider, "role", s.Role, "error", err)
	s.notify(Update{Kind: UpdateError, Err: err})
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > reconnectMax {
		return reconnectMax
	}
	return next
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
