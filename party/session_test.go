package party

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestMain lowers the Argon2id cost for the unit tests. Every card costs one
// derivation, and a room test derives a handful of them; at production cost the
// suite would spend most of its time in the KDF. The parameter defaults stay in
// the source for the release build.
func TestMain(m *testing.M) {
	kdfTime, kdfMemory, kdfThreads = 1, 8*1024, 1
	os.Exit(m.Run())
}

// testHub is a miniature version of the Cloudflare relay: one host, any number
// of guests, the card and the last state replayed to a newcomer. It lets the
// session, clock and reconnect code be exercised without an external service.
// Like the real Worker it only reads the frame type and never the encrypted
// body.
type testHub struct {
	server *httptest.Server
	url    string

	mu     sync.Mutex
	host   *websocket.Conn
	guests map[*websocket.Conn]bool
	card   []byte
	last   []byte
	rooms  []string
	// quiet mimics Supabase, which neither announces the host on connect nor
	// replays the last anchor. The guest has to make the host answer a hello.
	quiet bool
	// guestByes counts departures a guest announced. A guest must leave
	// silently: only the host's bye ends the room.
	guestByes int
}

func newTestHub(t *testing.T) *testHub {
	t.Helper()
	hub := &testHub{guests: make(map[*websocket.Conn]bool)}
	hub.server = httptest.NewServer(http.HandlerFunc(hub.serve))
	hub.url = "ws" + strings.TrimPrefix(hub.server.URL, "http")
	t.Cleanup(func() {
		hub.server.Close()
	})
	return hub
}

func (h *testHub) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/room" {
		http.NotFound(w, r)
		return
	}
	room := r.URL.Query().Get("room")
	if !validAddress(room) {
		http.Error(w, "bad room address", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	h.rooms = append(h.rooms, room)
	h.mu.Unlock()

	role := r.URL.Query().Get("role")
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	if role == string(Host) {
		h.mu.Lock()
		if h.host != nil {
			_ = h.host.Close(websocket.StatusNormalClosure, "replaced")
		}
		h.host = conn
		h.card = nil
		h.last = nil
		h.mu.Unlock()
		h.send(conn, frame{Type: messageWelcome, Host: boolPtr(true)})

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				break
			}
			var msg frame
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			switch msg.Type {
			case messageCard:
				h.mu.Lock()
				h.card = data
				guests := h.guestList()
				h.mu.Unlock()
				for _, guest := range guests {
					h.write(guest, data)
				}
			case messageState:
				h.mu.Lock()
				h.last = data
				guests := h.guestList()
				h.mu.Unlock()
				for _, guest := range guests {
					h.write(guest, data)
				}
			case messageClose:
				h.mu.Lock()
				guests := h.guestList()
				h.mu.Unlock()
				for _, guest := range guests {
					h.send(guest, frame{Type: messageClose})
				}
			case messageBye:
				h.dropHost(conn)
			}
		}
		h.dropHost(conn)
		return
	}

	h.mu.Lock()
	h.guests[conn] = true
	hosted := h.host != nil
	card := h.card
	last := h.last
	quiet := h.quiet
	h.mu.Unlock()
	if !quiet {
		h.send(conn, frame{Type: messageWelcome, Host: boolPtr(hosted)})
		if hosted && card != nil {
			h.write(conn, card)
		}
		if hosted && last != nil {
			h.write(conn, last)
		}
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		var msg frame
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case messageBye:
			h.mu.Lock()
			h.guestByes++
			h.mu.Unlock()
		case messageControl:
			h.mu.Lock()
			host := h.host
			h.mu.Unlock()
			if host != nil {
				h.send(host, msg)
			}
		case messageHello:
			h.mu.Lock()
			host := h.host
			quiet := h.quiet
			h.mu.Unlock()
			if host != nil {
				h.send(host, frame{Type: messageHello, Role: Guest})
			} else if !quiet {
				h.send(conn, frame{Type: messageWelcome, Host: boolPtr(false)})
			}
		}
	}

	h.mu.Lock()
	delete(h.guests, conn)
	h.mu.Unlock()
}

func (h *testHub) dropHost(conn *websocket.Conn) {
	h.mu.Lock()
	if h.host == conn {
		h.host = nil
	}
	h.card = nil
	h.last = nil
	guests := h.guestList()
	h.mu.Unlock()
	for _, guest := range guests {
		h.send(guest, frame{Type: messageBye})
	}
}

// signalBye mimics Cloudflare reporting a dropped host socket without the host
// actually closing the room, which is exactly the network hiccup the grace
// window has to survive.
func (h *testHub) signalBye() {
	h.mu.Lock()
	guests := h.guestList()
	h.mu.Unlock()
	for _, guest := range guests {
		h.send(guest, frame{Type: messageBye})
	}
}

func (h *testHub) guestByeCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.guestByes
}

func (h *testHub) seenRooms() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.rooms...)
}

func (h *testHub) guestList() []*websocket.Conn {
	list := make([]*websocket.Conn, 0, len(h.guests))
	for guest := range h.guests {
		list = append(list, guest)
	}
	return list
}

func (h *testHub) send(conn *websocket.Conn, msg frame) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.write(conn, data)
}

func (h *testHub) write(conn *websocket.Conn, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Write(ctx, websocket.MessageText, data)
}

func boolPtr(value bool) *bool { return &value }

func validAddress(address string) bool {
	if len(address) != 32 {
		return false
	}
	for _, r := range address {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition did not become true within %v", timeout)
}

func TestHostAndGuestShareAnAnchor(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "quiet-tanuki")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	if !ValidKey(host.Key) {
		t.Fatalf("Create returned key %q", host.Key)
	}

	guest, err := Join(context.Background(), cfg, strings.ToLower(host.Key), "brave-otter")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	// The host answers the guest's hello even before playback starts, which is
	// what tells the guest the key is real.
	waitFor(t, 3*time.Second, guest.HostSeen)

	want := State{
		TitleSlug: "demo-title",
		Voice:     "voice",
		Episode:   "3",
		Playing:   true,
		Position:  120,
	}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})

	state, ok := guest.State()
	if !ok {
		t.Fatal("guest has no state")
	}
	if state.Voice != want.Voice || state.Episode != want.Episode {
		t.Fatalf("guest state = %+v, want voice %q episode %q", state, want.Voice, want.Episode)
	}
	if state.Anchor <= 0 {
		t.Fatalf("guest state has no anchor: %+v", state)
	}
	if name := guest.HostName(); name != "quiet-tanuki" {
		t.Fatalf("host name = %q, want quiet-tanuki", name)
	}

	// The guest extrapolates on its own clock, so the position must be at or
	// slightly past the anchor position, never far away from it.
	position, _, ok := guest.ExpectedPosition(time.Now())
	if !ok {
		t.Fatal("ExpectedPosition is not ready")
	}
	if position < want.Position || position > want.Position+2 {
		t.Fatalf("expected position = %v, want around %v", position, want.Position)
	}
}

func TestRelayOnlySeesTheHashedAddress(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	address, err := Address(host.Key)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	if host.Address != address {
		t.Fatalf("session address = %q, want %q", host.Address, address)
	}
	rooms := hub.seenRooms()
	if len(rooms) == 0 {
		t.Fatal("the relay saw no room")
	}
	for _, room := range rooms {
		if room != address {
			t.Fatalf("relay saw room %q, want the address %q", room, address)
		}
		if strings.EqualFold(room, host.Key) {
			t.Fatalf("relay saw the key %q", room)
		}
	}
}

func TestWaitHostConfirmsAnExistingRoom(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()

	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := guest.WaitHost(ctx); err != nil {
		t.Fatalf("WaitHost: %v", err)
	}
}

func TestWaitHostRetriesAThrottledHello(t *testing.T) {
	hub := newTestHub(t)
	hub.quiet = true
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()

	first, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("first Join: %v", err)
	}
	defer first.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := first.WaitHost(ctx); err != nil {
		t.Fatalf("first WaitHost: %v", err)
	}

	// The second hello lands inside the host's helloThrottle window. WaitHost
	// has to ask again instead of declaring the room empty.
	second, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("second Join: %v", err)
	}
	defer second.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := second.WaitHost(ctx); err != nil {
		t.Fatalf("second WaitHost after a throttled hello: %v", err)
	}
}

func TestWaitHostReportsAnEmptyRoom(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	key, err := NewKey(Cloudflare)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	guest, err := Join(context.Background(), cfg, key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := guest.WaitHost(ctx); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("WaitHost = %v, want ErrRoomNotFound", err)
	}
}

func TestGuestLearnsWhenTheHostLeaves(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	waitFor(t, 3*time.Second, guest.HostSeen)
	if err := host.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// An intentional close carries the explicit close message, so it must not
	// wait for the reconnect grace window.
	waitFor(t, time.Second, guest.HostGone)
	if guest.HostSeen() {
		t.Fatal("the host is still marked as present after an explicit close")
	}
}

func TestHostGetsAReconnectWindow(t *testing.T) {
	oldSilence, oldGrace, oldTick := hostSilence, hostGrace, hostWatchTick
	hostSilence, hostGrace, hostWatchTick = 150*time.Millisecond, 250*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() {
		hostSilence, hostGrace, hostWatchTick = oldSilence, oldGrace, oldTick
	})

	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	waitFor(t, 3*time.Second, guest.HostSeen)
	hub.signalBye()
	waitFor(t, time.Second, func() bool { return !guest.HostSeen() })
	// The guest keeps the room and the last anchor during the grace window.
	if guest.HostGone() {
		t.Fatal("a dropped socket ended the room immediately")
	}

	// The host comes back before the window expires and publishes a fresh
	// anchor; the room must continue as if nothing happened.
	want := State{TitleSlug: "demo-title", Episode: "2", Playing: true, Position: 42}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish after reconnect: %v", err)
	}
	waitFor(t, time.Second, guest.HostSeen)
	time.Sleep(300 * time.Millisecond)
	if guest.HostGone() {
		t.Fatal("the room ended even though the host reconnected")
	}

	// A second drop with no recovery ends the room after the window.
	hub.signalBye()
	waitFor(t, 2*time.Second, guest.HostGone)
}

func TestGuestLeavesWithoutAnnouncingIt(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	waitFor(t, 3*time.Second, guest.HostSeen)

	if err := guest.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Supabase broadcasts everything, so a guest's bye would reach the other
	// guests and be read as the host leaving. The relay notices the closed
	// socket by itself.
	time.Sleep(150 * time.Millisecond)
	if n := hub.guestByeCount(); n != 0 {
		t.Fatalf("guest sent %d bye messages, want none", n)
	}
}

func TestGuestControlReachesTheHost(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	waitFor(t, 3*time.Second, guest.HostSeen)
	// SendControl seals the payload with the room key derived from the card;
	// without the card the guest answers "комната ещё не ответила".
	waitFor(t, 3*time.Second, func() bool {
		_, ok := guest.Card()
		return ok
	})

	if err := host.SendControl(ControlPause, 0); err == nil {
		t.Fatal("a host must not send guest controls")
	}
	if err := guest.SendControl(ControlSeek, 321.5); err != nil {
		t.Fatalf("SendControl: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool {
		for {
			select {
			case update := <-host.Updates():
				if update.Kind == UpdateControl {
					if update.Control.Action != ControlSeek || update.Control.Position != 321.5 {
						t.Fatalf("control = %+v, want seek to 321.5", update.Control)
					}
					return true
				}
			default:
				return false
			}
		}
	})
}

// RequestRefresh asks the host to republish its anchor. The guest loop uses it
// when the anchor feed stalls while the player runs: a half-open socket never
// surfaces on read, and without it the guest would extrapolate past a pause or
// seek that never arrives.
func TestGuestRequestsAFreshAnchor(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	waitFor(t, 3*time.Second, guest.HostSeen)

	want := State{TitleSlug: "filmlib:685246", Voice: "v", Episode: "S1E1", Playing: true, Position: 30}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})
	before, ok := guest.State()
	if !ok {
		t.Fatal("guest has no state before the refresh")
	}
	// The host answers the hello with a fresh anchor, so the Anchor timestamp
	// must move forward. Sleep past helloThrottle so the republish is allowed.
	time.Sleep(helloThrottle + 100*time.Millisecond)
	if err := guest.RequestRefresh(); err != nil {
		t.Fatalf("RequestRefresh: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		after, ok := guest.State()
		return ok && after.Anchor != before.Anchor
	})
}

func TestGuestNoticesASilentHost(t *testing.T) {
	oldSilence, oldGrace, oldTick := hostSilence, hostGrace, hostWatchTick
	hostSilence, hostGrace, hostWatchTick = 120*time.Millisecond, 150*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() {
		hostSilence, hostGrace, hostWatchTick = oldSilence, oldGrace, oldTick
	})

	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	// The test hub never publishes heartbeats, so the greeting is the last
	// thing the guest hears. The silence watchdog starts the grace window and
	// the room only ends once it expires.
	waitFor(t, 3*time.Second, guest.HostSeen)
	waitFor(t, 3*time.Second, func() bool { return !guest.HostSeen() })
	if guest.HostGone() {
		t.Fatal("silence ended the room before the grace window expired")
	}
	waitFor(t, 3*time.Second, guest.HostGone)
}

func TestMessagesCarryTheProtocolVersion(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()

	want := State{TitleSlug: "secret-anime-title", Episode: "4", Playing: true, Position: 5}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return hub.last != nil
	})
	hub.mu.Lock()
	last := string(hub.last)
	hub.mu.Unlock()
	if !strings.Contains(last, `"v":"`+ProtocolVersion+`"`) {
		t.Fatalf("host state does not carry protocol %s: %s", ProtocolVersion, last)
	}
	if !strings.Contains(last, `"d":"`) {
		t.Fatalf("host state is not encrypted: %s", last)
	}
	if strings.Contains(last, want.TitleSlug) {
		t.Fatalf("host state leaks the title on the wire: %s", last)
	}
}

func TestGuestLeavesOnProtocolMismatch(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	key, err := NewKey(Cloudflare)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	guest, err := Join(context.Background(), cfg, key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	var guests []*websocket.Conn
	waitFor(t, 3*time.Second, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		guests = hub.guestList()
		return len(guests) > 0
	})
	// A host left over from an older build sends no version at all; a future
	// one sends another value. Both have to be refused before the anchor is
	// stored.
	hub.send(guests[0], frame{Type: messageState, Version: "0.9", Data: "AAAA"})

	deadline := time.After(3 * time.Second)
	for {
		select {
		case update := <-guest.Updates():
			if update.Kind != UpdateError || !errors.Is(update.Err, ErrVersionMismatch) {
				continue
			}
		case <-deadline:
			t.Fatal("guest did not report a protocol mismatch")
		}
		break
	}
	if _, ok := guest.State(); ok {
		t.Fatal("guest stored an anchor from a room with another protocol")
	}
	if !errors.Is(guest.Err(), ErrVersionMismatch) {
		t.Fatalf("guest error = %v, want ErrVersionMismatch", guest.Err())
	}
}

// Supabase broadcasts every frame to every participant. A guest must not be
// able to replace the host's card or anchor with a forged one.
func TestHostIgnoresFramesFromGuests(t *testing.T) {
	const key = "YOC4K7Q2M"
	card, contentKey, err := newCard(key, "", false, 1)
	if err != nil {
		t.Fatalf("newCard: %v", err)
	}
	host := &Session{
		Key:     key,
		Role:    Host,
		card:    card,
		hasCard: true,
		key:     contentKey,
		updates: make(chan Update, 8),
	}
	forgedCard, forgedKey, err := newCard(key, "4K7Q2M9XBT", true, 99)
	if err != nil {
		t.Fatalf("forged card: %v", err)
	}
	host.handle(frame{Version: ProtocolVersion, Type: messageCard, Card: &forgedCard})

	host.mu.Lock()
	active := host.card
	activeKey := append([]byte(nil), host.key...)
	host.mu.Unlock()
	if active.Epoch != 1 || active.Locked || !bytes.Equal(activeKey, contentKey) {
		t.Fatalf("host accepted a guest card: epoch=%d locked=%v", active.Epoch, active.Locked)
	}

	body, err := json.Marshal(payload{Name: "intruder", State: &State{TitleSlug: "forged"}})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	data, err := sealContent(forgedKey, body)
	if err != nil {
		t.Fatalf("seal payload: %v", err)
	}
	host.handle(frame{Version: ProtocolVersion, Type: messageState, Data: data})
	if _, ok := host.State(); ok {
		t.Fatal("host stored an anchor from a guest")
	}
	if name := host.HostName(); name != "" {
		t.Fatalf("host name came from a guest frame: %q", name)
	}

	host.mu.Lock()
	host.hostSeen = true
	host.mu.Unlock()
	host.handle(frame{Version: ProtocolVersion, Type: messageBye})
	if !host.HostSeen() {
		t.Fatal("a guest bye made the host think its own socket had left")
	}
}

func TestJoinRejectsAMistypedKey(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	if _, err := Join(context.Background(), cfg, "YOCBQ0PG", ""); err == nil {
		t.Fatal("Join accepted a key that is too short")
	}
	if _, err := Join(context.Background(), cfg, "XXCBQ0PGZ", ""); err == nil {
		t.Fatal("Join accepted a key without the YO prefix")
	}
	if _, err := Join(context.Background(), cfg, "YOFBQ0PGZ", ""); err == nil {
		t.Fatal("Join accepted a key without a known relay")
	}
}

func TestCreateNeedsAConfiguredProvider(t *testing.T) {
	if _, err := Create(context.Background(), Config{}, Cloudflare, ""); err == nil {
		t.Fatal("Create accepted an empty Cloudflare configuration")
	}
	if _, err := Create(context.Background(), Config{SupabaseURL: "https://x"}, Supabase, ""); err == nil {
		t.Fatal("Create accepted Supabase without an anon key")
	}
}

// TestHostPublishesRightAfterCreate keeps the first anchor from being lost to
// the goroutine that attaches the socket: a host publishes as soon as Create
// returns, and a guest waiting for that anchor would otherwise sit on the
// waiting screen until the next heartbeat.
func TestHostPublishesRightAfterCreate(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()

	// No sleep and no wait for the reader goroutine: this is exactly what the
	// host loop does when a room opens during playback.
	want := State{TitleSlug: "demo-title", Episode: "1", Playing: true, Position: 10}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish right after Create: %v", err)
	}

	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})
}

func TestLockedRoomNeedsTheSecret(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(secret); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := guest.WaitHost(ctx); err != nil {
		t.Fatalf("WaitHost: %v", err)
	}
	if err := guest.WaitCard(ctx); err != nil {
		t.Fatalf("WaitCard: %v", err)
	}
	if !guest.NeedsSecret() {
		t.Fatal("a locked room did not ask for a secret")
	}
	if _, ok := guest.State(); ok {
		t.Fatal("a guest without the secret read an anchor")
	}

	if err := guest.SubmitSecret("0000000000"); !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("SubmitSecret with a wrong secret = %v, want ErrWrongSecret", err)
	}
	if !guest.NeedsSecret() {
		t.Fatal("a wrong secret cleared the prompt")
	}
	if err := guest.SubmitSecret(secret); err != nil {
		t.Fatalf("SubmitSecret: %v", err)
	}
	if guest.NeedsSecret() {
		t.Fatal("the accepted secret did not clear the prompt")
	}

	want := State{TitleSlug: "demo-title", Episode: "7", Playing: true, Position: 60}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})
	// A wrong-secret attempt must never reach the host as a control or state
	// change; the card check refuses it locally.
	if got := guest.Secret(); got != secret {
		t.Fatalf("guest secret = %q, want %q", got, secret)
	}
}

func TestGuestInTheRoomIsAskedForANewSecret(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	want := State{TitleSlug: "demo-title", Episode: "1", Playing: true, Position: 5}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		_, ok := guest.State()
		return ok
	})

	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(secret); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	waitFor(t, 3*time.Second, guest.NeedsSecret)

	// The old anchor stays available for the drift loop until the UI stops the
	// guest playback, but no new anchor is readable.
	if err := guest.SubmitSecret(secret); err != nil {
		t.Fatalf("SubmitSecret: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return !guest.NeedsSecret() })
}

func TestUnlockLetsEveryGuestBackIn(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(secret); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := guest.WaitCard(ctx); err != nil {
		t.Fatalf("WaitCard: %v", err)
	}
	if !guest.NeedsSecret() {
		t.Fatal("the room was not locked")
	}
	if err := host.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return !guest.NeedsSecret() })

	want := State{TitleSlug: "demo-title", Episode: "2", Playing: true, Position: 9}
	if err := host.Publish(want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})
}

func TestSecretRotationInvalidatesTheOldSecret(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	first, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(first); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	guest, err := Join(context.Background(), cfg, host.Key, "")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := guest.WaitCard(ctx); err != nil {
		t.Fatalf("WaitCard: %v", err)
	}
	if err := guest.SubmitSecret(first); err != nil {
		t.Fatalf("SubmitSecret(first): %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return !guest.NeedsSecret() })

	second, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(second); err != nil {
		t.Fatalf("Lock again: %v", err)
	}
	waitFor(t, 3*time.Second, guest.NeedsSecret)
	if err := guest.SubmitSecret(first); !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("the old secret opened the rotated room: %v", err)
	}
	if err := guest.SubmitSecret(second); err != nil {
		t.Fatalf("SubmitSecret(second): %v", err)
	}
}

func TestRoomCardIsCleartextAndSecretIsNot(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	secret, err := NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if _, err := host.Lock(secret); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return hub.card != nil
	})
	hub.mu.Lock()
	card := string(hub.card)
	hub.mu.Unlock()
	if !strings.Contains(card, `"locked":true`) {
		t.Fatalf("card does not announce the lock: %s", card)
	}
	if strings.Contains(card, secret) || strings.Contains(card, host.Key) {
		t.Fatalf("card leaks a secret or the key: %s", card)
	}
}
