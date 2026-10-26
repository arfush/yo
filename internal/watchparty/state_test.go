package watchparty

import (
	"testing"
	"time"
)

func TestStatePositionAt(t *testing.T) {
	anchor := time.UnixMilli(1_700_000_000_000)
	state := State{Position: 120, Anchor: anchor.UnixMilli(), Playing: true}

	if got := state.PositionAt(anchor.Add(5 * time.Second)); got != 125 {
		t.Fatalf("PositionAt(+5s) = %v, want 125", got)
	}
	if got := state.PositionAt(anchor.Add(-5 * time.Second)); got != 120 {
		t.Fatalf("PositionAt(-5s) = %v, want the anchor position 120", got)
	}

	paused := state
	paused.Playing = false
	if got := paused.PositionAt(anchor.Add(time.Hour)); got != 120 {
		t.Fatalf("paused PositionAt() = %v, want 120", got)
	}
}

func TestStateSamePlayback(t *testing.T) {
	first := State{TitleSlug: "a", Voice: "v", Episode: "1"}
	same := State{TitleSlug: "a", Voice: "v", Episode: "1", Position: 90, Playing: true}
	if !first.SamePlayback(same) {
		t.Fatal("SamePlayback() should ignore position and playing")
	}
	other := State{TitleSlug: "a", Voice: "v", Episode: "2"}
	if first.SamePlayback(other) {
		t.Fatal("SamePlayback() should notice another episode")
	}
}

func TestStateGuestsControl(t *testing.T) {
	if _, ok := (State{}).GuestsControl(); ok {
		t.Fatal("an anchor without the field must not claim a control setting")
	}
	off := false
	disallowed := State{ControlEnabled: &off}
	if allowed, ok := disallowed.GuestsControl(); !ok || allowed {
		t.Fatalf("GuestsControl() = %v, %v; want false, true", allowed, ok)
	}
	on := true
	allowedState := State{ControlEnabled: &on}
	if allowed, ok := allowedState.GuestsControl(); !ok || !allowed {
		t.Fatalf("GuestsControl() = %v, %v; want true, true", allowed, ok)
	}
}

func TestNormalizeDisplay(t *testing.T) {
	cases := map[string]string{
		"yoc4k7q2m":   "YOC4K7Q2M",
		"  YOC4K7Q2M": "YOC4K7Q2M",
		"":            "",
		"yoc":         "YOC",
	}
	for input, want := range cases {
		if got := NormalizeDisplay(input); got != want {
			t.Fatalf("NormalizeDisplay(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGuestCannotPublish(t *testing.T) {
	session := &Session{Role: Guest}
	if err := session.Publish(State{TitleSlug: "x"}); err == nil {
		t.Fatal("a guest must not be able to publish a state")
	}
	if _, ok := session.State(); ok {
		t.Fatal("a fresh session has no state")
	}
	if session.HostSeen() {
		t.Fatal("a fresh session has not seen the host")
	}
}

func TestProviderValidation(t *testing.T) {
	if !Cloudflare.Valid() || !Supabase.Valid() || Provider("x").Valid() {
		t.Fatal("Provider.Valid() is wrong")
	}
	if Cloudflare.Label() == "" || Supabase.Label() == "" {
		t.Fatal("a known provider needs a label")
	}
}

func TestConfigNormalizesEndpoints(t *testing.T) {
	cfg := Config{
		CloudflareURL: "https://party.example.workers.dev/",
		SupabaseURL:   "https://abcd.supabase.co/",
		SupabaseKey:   " anon ",
	}.normalized()

	if cfg.CloudflareURL != "wss://party.example.workers.dev" {
		t.Fatalf("cloudflare url = %q", cfg.CloudflareURL)
	}
	if cfg.SupabaseKey != "anon" {
		t.Fatalf("supabase key = %q", cfg.SupabaseKey)
	}

	address := "0123456789abcdef0123456789abcdef"
	endpoint, err := cfg.CloudflareEndpoint(address, Guest)
	if err != nil {
		t.Fatalf("CloudflareEndpoint: %v", err)
	}
	want := "wss://party.example.workers.dev/room?role=guest&room=" + address
	if endpoint != want {
		t.Fatalf("endpoint = %q, want %q", endpoint, want)
	}

	supabase, err := cfg.SupabaseEndpoint()
	if err != nil {
		t.Fatalf("SupabaseEndpoint: %v", err)
	}
	wantSupabase := "wss://abcd.supabase.co/realtime/v1/websocket?apikey=anon&vsn=1.0.0"
	if supabase != wantSupabase {
		t.Fatalf("supabase endpoint = %q, want %q", supabase, wantSupabase)
	}
}

func TestConfigConfigured(t *testing.T) {
	empty := Config{}
	if empty.Configured(Cloudflare) || empty.Configured(Supabase) {
		t.Fatal("an empty config must not look configured")
	}
	onlyKey := Config{SupabaseKey: "anon"}
	if onlyKey.Configured(Supabase) {
		t.Fatal("Supabase needs the project URL too")
	}
}
