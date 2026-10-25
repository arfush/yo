package discord

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestPresenceDetailsSeries(t *testing.T) {
	presence := Presence{
		Title:   "Провожающая в последний путь Фрирен",
		Episode: "5",
		Total:   12,
	}
	want := "Провожающая в последний путь Фрирен 5/12"
	if got := presence.Details(); got != want {
		t.Fatalf("details = %q, want %q", got, want)
	}
}

func TestPresenceDetailsMovieHasNoEpisodes(t *testing.T) {
	presence := Presence{
		Title:   "Кими но На ва",
		Episode: "1",
		Total:   1,
		Movie:   true,
	}
	if got, want := presence.Details(), "Кими но На ва"; got != want {
		t.Fatalf("details = %q, want %q", got, want)
	}
}

func TestPresenceDetailsWithoutEpisodeOrTotal(t *testing.T) {
	for _, presence := range []Presence{
		{Title: "Фрирен", Episode: "", Total: 28},
		{Title: "Фрирен", Episode: "5", Total: 0},
	} {
		if got, want := presence.Details(), "Фрирен"; got != want {
			t.Fatalf("details = %q, want %q", got, want)
		}
	}
}

// TestPresenceDetailsKeepsTheEpisodeWhenTheTitleIsLong guards the truncation
// order: Discord cuts the whole line at 128 characters, so the title has to
// shrink and the episode must survive.
func TestPresenceDetailsKeepsTheEpisodeWhenTheTitleIsLong(t *testing.T) {
	presence := Presence{
		Title:   strings.Repeat("Очень длинное название ", 12),
		Episode: "5",
		Total:   12,
	}
	details := presence.Details()
	if !strings.HasSuffix(details, " 5/12") {
		t.Fatalf("details = %q, want the episode suffix intact", details)
	}
	if !strings.HasSuffix(details, "… 5/12") {
		t.Fatalf("details = %q, want a shortened title", details)
	}
	if runeLen(details) != maxTextRunes {
		t.Fatalf("details has %d runes, want %d", runeLen(details), maxTextRunes)
	}
}

// TestPresenceActivityKeepsTheAppNameAndHasNoArtwork locks the requested
// shape: Discord shows the application name "Yonime", the compact status line
// stays on the default name, and no cover is uploaded anywhere.
func TestPresenceActivityKeepsTheAppNameAndHasNoArtwork(t *testing.T) {
	presence := Presence{Title: "Фрирен", Episode: "5", Total: 12}
	activity := presence.activity()

	if activity["type"] != activityTypeWatching {
		t.Fatalf("type = %v, want %d", activity["type"], activityTypeWatching)
	}
	if activity["details"] != "Фрирен 5/12" {
		t.Fatalf("details = %v", activity["details"])
	}
	for _, key := range []string{"assets", "name", "status_display_type"} {
		if _, ok := activity[key]; ok {
			t.Fatalf("%s must be absent: %#v", key, activity)
		}
	}
}

func TestClientSendsSetActivityFrame(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := &ipcClient{conn: clientConn, r: bufio.NewReader(clientConn)}

	want := Presence{Title: "Фрирен", Episode: "5", Total: 12}
	go func() {
		_ = client.setActivity(want.activity())
	}()

	op, payload := readTestFrame(t, serverConn)
	if op != opFrame {
		t.Fatalf("opcode = %d, want %d", op, opFrame)
	}
	var frame struct {
		Cmd   string `json:"cmd"`
		Nonce string `json:"nonce"`
		Args  struct {
			PID      int            `json:"pid"`
			Activity map[string]any `json:"activity"`
		} `json:"args"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Cmd != "SET_ACTIVITY" || frame.Nonce == "" {
		t.Fatalf("frame = %#v", frame)
	}
	if frame.Args.PID <= 0 {
		t.Fatalf("pid = %d", frame.Args.PID)
	}
	if frame.Args.Activity["details"] != "Фрирен 5/12" {
		t.Fatalf("details = %v", frame.Args.Activity["details"])
	}
	if _, ok := frame.Args.Activity["name"]; ok {
		t.Fatalf("name must stay unset so Discord uses the app name: %#v", frame.Args.Activity)
	}
	if _, ok := frame.Args.Activity["assets"]; ok {
		t.Fatalf("assets must stay unset: %#v", frame.Args.Activity)
	}
}

// TestClientClearSendsNullActivity keeps the explicit null: an empty object
// would leave the old card on the profile.
func TestClientClearSendsNullActivity(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := &ipcClient{conn: clientConn, r: bufio.NewReader(clientConn)}

	go func() {
		_ = client.clear()
	}()

	_, payload := readTestFrame(t, serverConn)
	var frame struct {
		Args struct {
			Activity *map[string]any `json:"activity"`
		} `json:"args"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Args.Activity != nil {
		t.Fatalf("activity = %#v, want null", *frame.Args.Activity)
	}
}

// TestNewClientWaitsForReady guards the handshake: Discord ignores commands
// sent before it answers with READY, which made the first activity of a session
// disappear until the player closed.
func TestNewClientWaitsForReady(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	done := make(chan error, 1)
	go func() {
		client, err := newClient(clientConn, "123")
		if client != nil {
			client.close()
		}
		done <- err
	}()

	if op, _ := readTestFrame(t, serverConn); op != opHandshake {
		t.Fatalf("opcode = %d, want handshake", op)
	}
	select {
	case err := <-done:
		t.Fatalf("newClient returned before READY: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	writeTestFrame(t, serverConn, opFrame, map[string]any{"cmd": "DISPATCH", "evt": "READY"})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("newClient: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("newClient did not return after READY")
	}
}

func TestClientAnswersPing(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := &ipcClient{conn: clientConn, r: bufio.NewReader(clientConn)}

	go func() {
		_ = client.watch()
	}()
	writeTestFrame(t, serverConn, opPing, map[string]any{"v": 1})

	op, _ := readTestFrame(t, serverConn)
	if op != opPong {
		t.Fatalf("opcode = %d, want pong", op)
	}
}

func TestManagerWithoutAppIDIsInert(t *testing.T) {
	manager := New("")
	manager.Set(Presence{Title: "Фрирен"})
	manager.Clear()

	done := make(chan struct{})
	go func() {
		manager.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not return")
	}
}

func readTestFrame(t *testing.T, r io.Reader) (int, []byte) {
	t.Helper()
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		t.Fatal(err)
	}
	size := int(binary.LittleEndian.Uint32(header[4:8]))
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		t.Fatal(err)
	}
	return int(binary.LittleEndian.Uint32(header[0:4])), payload
}

func writeTestFrame(t *testing.T, w io.Writer, op int, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(frame[0:4], uint32(op))
	binary.LittleEndian.PutUint32(frame[4:8], uint32(len(data)))
	copy(frame[8:], data)
	if _, err := w.Write(frame); err != nil {
		t.Fatal(err)
	}
}
