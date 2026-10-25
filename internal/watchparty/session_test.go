package watchparty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// testHub is a miniature version of the Cloudflare relay: one host, any number
// of guests, and the last state replayed to a newcomer. It lets the session,
// clock and reconnect code be exercised without an external service.
type testHub struct {
	server *httptest.Server
	url    string

	mu     sync.Mutex
	host   *websocket.Conn
	guests map[*websocket.Conn]bool
	last   []byte
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
	key := r.URL.Query().Get("key")
	if !ValidKey(key) {
		http.Error(w, "bad key", http.StatusBadRequest)
		return
	}
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
		h.last = nil
		h.mu.Unlock()
		h.send(conn, message{Type: messageWelcome, Host: boolPtr(true)})

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				break
			}
			var msg message
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			switch msg.Type {
			case messageState:
				h.mu.Lock()
				h.last = data
				guests := h.guestList()
				h.mu.Unlock()
				for _, guest := range guests {
					h.write(guest, data)
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
	last := h.last
	h.mu.Unlock()
	h.send(conn, message{Type: messageWelcome, Host: boolPtr(hosted)})
	if hosted && last != nil {
		h.write(conn, last)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		var msg message
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
			h.mu.Unlock()
			if host != nil {
				h.send(host, message{Type: messageHello, Role: Guest})
			} else {
				h.send(conn, message{Type: messageWelcome, Host: boolPtr(false)})
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
	h.last = nil
	guests := h.guestList()
	h.mu.Unlock()
	for _, guest := range guests {
		h.send(guest, message{Type: messageBye})
	}
}

func (h *testHub) guestByeCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.guestByes
}

func (h *testHub) guestList() []*websocket.Conn {
	list := make([]*websocket.Conn, 0, len(h.guests))
	for guest := range h.guests {
		list = append(list, guest)
	}
	return list
}

func (h *testHub) send(conn *websocket.Conn, msg message) {
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

	host, err := Create(context.Background(), cfg, Cloudflare)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	if !ValidKey(host.Key) {
		t.Fatalf("Create returned key %q", host.Key)
	}

	guest, err := Join(context.Background(), cfg, Cloudflare, strings.ToLower(host.Key))
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

func TestGuestLearnsWhenTheHostLeaves(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	guest, err := Join(context.Background(), cfg, Cloudflare, host.Key)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	waitFor(t, 3*time.Second, guest.HostSeen)
	if err := host.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return !guest.HostSeen() })
}

func TestGuestLeavesWithoutAnnouncingIt(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, Cloudflare, host.Key)
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

	host, err := Create(context.Background(), cfg, Cloudflare)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, Cloudflare, host.Key)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	waitFor(t, 3*time.Second, guest.HostSeen)

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

func TestGuestNoticesASilentHost(t *testing.T) {
	oldSilence, oldTick := hostSilence, hostWatchTick
	hostSilence, hostWatchTick = 120*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() {
		hostSilence, hostWatchTick = oldSilence, oldTick
	})

	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	host, err := Create(context.Background(), cfg, Cloudflare)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer host.Close()
	guest, err := Join(context.Background(), cfg, Cloudflare, host.Key)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()

	// The test hub never publishes heartbeats, so the greeting is the last
	// thing the guest hears and the watchdog has to declare the host gone.
	waitFor(t, 3*time.Second, guest.HostSeen)
	waitFor(t, 3*time.Second, func() bool { return !guest.HostSeen() })
}

func TestJoinRejectsAMistypedKey(t *testing.T) {
	hub := newTestHub(t)
	cfg := Config{CloudflareURL: hub.url}

	if _, err := Join(context.Background(), cfg, Cloudflare, "YOFBQOPG"); err == nil {
		t.Fatal("Join accepted a key that is too short")
	}
	if _, err := Join(context.Background(), cfg, Cloudflare, "XXFBQOPGZ"); err == nil {
		t.Fatal("Join accepted a key without the YO prefix")
	}
}

func TestCreateNeedsAConfiguredProvider(t *testing.T) {
	if _, err := Create(context.Background(), Config{}, Cloudflare); err == nil {
		t.Fatal("Create accepted an empty Cloudflare configuration")
	}
	if _, err := Create(context.Background(), Config{SupabaseURL: "https://x"}, Supabase); err == nil {
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

	host, err := Create(context.Background(), cfg, Cloudflare)
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

	guest, err := Join(context.Background(), cfg, Cloudflare, host.Key)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer guest.Close()
	waitFor(t, 3*time.Second, func() bool {
		state, ok := guest.State()
		return ok && state.TitleSlug == want.TitleSlug
	})
}
