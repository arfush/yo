package watchparty

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	reconnectMin = time.Second
	reconnectMax = 15 * time.Second
	// helloThrottle keeps a burst of joining guests from making the host
	// publish once per guest; one fresh anchor answers all of them.
	helloThrottle = 700 * time.Millisecond
	updatesBuffer = 8
)

// Session is one joined room. It owns the WebSocket, reconnects on its own and
// keeps the newest host anchor plus the clock estimate.
type Session struct {
	Key      string
	Provider Provider
	Role     Role
	Name     string

	cfg Config

	mu        sync.Mutex
	conn      transport
	state     State
	hasState  bool
	hostSeen  bool
	closed    bool
	lastErr   error
	lastHello time.Time
	source    func() (State, bool)
	clock     Clock

	updates chan Update

	cancel context.CancelFunc
}

// Create allocates a fresh room key and opens it as the host.
func Create(ctx context.Context, cfg Config, provider Provider) (*Session, error) {
	key, err := NewKey()
	if err != nil {
		return nil, err
	}
	return open(ctx, cfg, provider, key, Host)
}

// Join normalises the key and opens the room as a guest.
func Join(ctx context.Context, cfg Config, provider Provider, key string) (*Session, error) {
	normalized, err := NormalizeKey(key)
	if err != nil {
		return nil, err
	}
	return open(ctx, cfg, provider, normalized, Guest)
}

func open(ctx context.Context, cfg Config, provider Provider, key string, role Role) (*Session, error) {
	if !provider.Valid() || !cfg.Configured(provider) {
		return nil, ErrNotConfigured
	}
	session := &Session{
		Key:      key,
		Provider: provider,
		Role:     role,
		cfg:      cfg,
		updates:  make(chan Update, updatesBuffer),
	}
	conn, err := session.dial(ctx)
	if err != nil {
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

// HostSeen reports whether the host has answered at least once.
func (s *Session) HostSeen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostSeen
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
	return s.write(message{Type: messageState, State: &state})
}

// ExpectedPosition returns where the host expects playback to be right now.
func (s *Session) ExpectedPosition(now time.Time) (float64, State, bool) {
	state, ok := s.State()
	if !ok {
		return 0, State{}, false
	}
	return state.PositionAt(s.clock.HostNow(now)), state, true
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

	if conn != nil {
		_ = s.write(message{Type: messageBye})
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
		return dialCloudflare(ctx, s.cfg, s.Key, s.Role)
	case Supabase:
		return dialSupabase(ctx, s.cfg, s.Key, s.Role, s.Name)
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

// greet re-announces the room after a (re)connect. Hosts push an anchor so
// guests learn the host is present even before playback starts; guests ask for
// one.
func (s *Session) greet() {
	if s.Role == Host {
		state, ok := s.currentState()
		if !ok {
			// An empty state still proves to a waiting guest that the host is
			// in the room.
			state = State{}
		}
		_ = s.Publish(state)
		return
	}
	_ = s.write(message{Type: messageHello, Role: s.Role})
}

func (s *Session) handle(msg message) {
	switch msg.Type {
	case messageState:
		if msg.State == nil {
			return
		}
		state := *msg.State
		s.mu.Lock()
		first := !s.hostSeen && s.Role == Guest
		s.state = state
		s.hasState = true
		s.hostSeen = true
		s.mu.Unlock()
		s.clock.Add(state.Anchor, time.Now().UnixMilli())
		if first {
			s.notify(Update{Kind: UpdateHostSeen})
		}
		s.notify(Update{Kind: UpdateState, State: state})
	case messageWelcome:
		if msg.Host == nil {
			return
		}
		seen := *msg.Host
		s.mu.Lock()
		previous := s.hostSeen
		s.hostSeen = seen
		s.mu.Unlock()
		if seen && !previous {
			s.notify(Update{Kind: UpdateHostSeen})
		}
		if !seen && previous {
			s.notify(Update{Kind: UpdateHostLeft})
		}
	case messageHello:
		if s.Role == Host {
			s.publishSoon()
		}
	case messageBye:
		s.mu.Lock()
		s.hostSeen = false
		s.mu.Unlock()
		s.notify(Update{Kind: UpdateHostLeft})
	case messageError:
		text := msg.Error
		if text == "" {
			text = "ошибка комнаты"
		}
		s.reportError(errors.New(text))
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

func (s *Session) write(msg message) error {
	s.mu.Lock()
	conn := s.conn
	closed := s.closed
	s.mu.Unlock()
	if closed || conn == nil {
		return errNotConnected
	}
	return conn.write(msg)
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
