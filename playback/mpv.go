// Package playback launches mpv, keeps its playlist in sync with the selected
// AnimeLIB voice and saves/resumes watch progress.
package playback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dexterlb/mpvipc"

	"github.com/arfush/yo/libsocial"
	"github.com/arfush/yo/metadata"
	"github.com/arfush/yo/mpv"
)

type mpvEventKind int

const (
	mpvEventFileLoaded mpvEventKind = iota
	mpvEventTimePos
	mpvEventPlaylistPos
	mpvEventPause
	mpvEventDuration
	mpvEventEndFile
	mpvEventExit
)

type mpvEvent struct {
	Kind     mpvEventKind
	Path     string
	Seconds  float64
	Position int
	// EntryID is mpv's playlist entry id for end-file. A retry replaces the
	// first entry, so an error that still carries the old id belongs to the
	// file that has already been abandoned and must not finish the new one.
	EntryID int64
	Paused  bool
	Reason  string
	Err     error
}

// mpvProcess owns one mpv child process and its IPC connection.
type mpvProcess struct {
	cmd  *exec.Cmd
	conn *mpvipc.Connection
	pipe string

	events     chan *mpvipc.Event
	stopEvents chan struct{}
	out        chan mpvEvent
	done       chan struct{}

	mu              sync.Mutex
	lastPlaylistPos int
	stopping        bool
	closeOnce       sync.Once
	lastErr         error

	// callMu serialises IPC commands. The host heartbeat, the UI, the
	// background loader and the open retry all talk to the same connection,
	// and mpvipc writes a JSON line in two steps: interleaved writes corrupt
	// the command stream, especially when a retry reload races a heartbeat.
	callMu sync.Mutex
}

func startMPV(ctx context.Context, geometry, title string) (*mpvProcess, error) {
	path, err := findMPV()
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "yo"
	}
	pipe := mpvPipePath("yo-mpv-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	args := []string{
		"--input-ipc-server=" + pipe,
		"--idle=yes",
		"--force-window=yes",
		"--keep-open=yes",
		"--no-terminal",
		"--input-terminal=no",
		"--ytdl=no",
		"--osc=yes",
		"--title=" + title,
		// Watch-party guests seek in small steps; without exact seeking mpv
		// rounds to the nearest keyframe and the correction overshoots.
		"--hr-seek=yes",
		"--http-header-fields=Referer: " + libsocial.WebOrigin + "/,Origin: " + libsocial.WebOrigin,
		"--background-color=#0d0e14",
	}
	if geometry != "" {
		args = append(args, "--geometry="+geometry)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	configureMPVCommand(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start mpv: %w", err)
	}
	m := &mpvProcess{
		cmd:             cmd,
		pipe:            pipe,
		events:          nil,
		out:             make(chan mpvEvent, 256),
		done:            make(chan struct{}),
		lastPlaylistPos: 0,
	}

	var conn *mpvipc.Connection
	var openErr error
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			_ = cmd.Process.Kill()
			return nil, ctx.Err()
		}
		candidate := mpvipc.NewConnection(pipe)
		if err := candidate.Open(); err == nil {
			conn = candidate
			break
		} else {
			openErr = err
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, ctx.Err()
		case <-time.After(80 * time.Millisecond):
		}
	}
	if conn == nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("cannot connect to mpv: %w", openErr)
	}
	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()
	m.events, m.stopEvents = conn.NewEventListener()
	for id, property := range map[int64]string{
		1: "time-pos",
		2: "playlist-pos",
		3: "pause",
		4: "duration",
	} {
		_, _ = conn.Call("observe_property", id, property)
	}

	go m.waitProcess()
	go m.eventLoop()
	return m, nil
}

func (m *mpvProcess) waitProcess() {
	err := m.cmd.Wait()
	m.mu.Lock()
	stopping := m.stopping
	m.lastErr = err
	m.mu.Unlock()
	if err != nil && !stopping {
		m.send(mpvEvent{Kind: mpvEventExit, Err: err})
	} else {
		m.send(mpvEvent{Kind: mpvEventExit})
	}
	_ = m.conn.Close()
	m.closeOnce.Do(func() { close(m.done) })
}

func (m *mpvProcess) eventLoop() {
	defer func() {
		if m.stopEvents != nil {
			close(m.stopEvents)
		}
	}()
	for {
		select {
		case <-m.done:
			return
		case event, ok := <-m.events:
			if !ok {
				return
			}
			if event == nil {
				continue
			}
			switch event.Name {
			case "file-loaded":
				// The playlist-pos property event can arrive after file-loaded.
				// Ask mpv which item is actually playing so progress and resume
				// are attributed to the right episode.
				m.send(mpvEvent{Kind: mpvEventFileLoaded, Position: m.currentPlaylistPos()})
			case "end-file":
				m.send(mpvEvent{
					Kind:     mpvEventEndFile,
					Reason:   event.Reason,
					Position: m.playlistPos(),
					EntryID:  playlistEntryID(event),
				})
			case "property-change":
				switch event.ID {
				case 1:
					if value, ok := event.Data.(float64); ok {
						m.send(mpvEvent{Kind: mpvEventTimePos, Seconds: value})
					}
				case 2:
					if value, ok := event.Data.(float64); ok {
						position := int(value)
						m.mu.Lock()
						m.lastPlaylistPos = position
						m.mu.Unlock()
						m.send(mpvEvent{Kind: mpvEventPlaylistPos, Position: position})
					}
				case 3:
					if paused, ok := event.Data.(bool); ok {
						m.send(mpvEvent{Kind: mpvEventPause, Paused: paused})
					}
				case 4:
					if value, ok := event.Data.(float64); ok {
						m.send(mpvEvent{Kind: mpvEventDuration, Seconds: value})
					}
				}
			}
		}
	}
}

func (m *mpvProcess) send(event mpvEvent) {
	select {
	case m.out <- event:
	case <-time.After(2 * time.Second):
	}
}

// playlistEntryID returns the mpv playlist entry the event belongs to. The
// field is absent on older mpv builds, in which case zero disables the stale
// end-file check rather than breaking playback.
func playlistEntryID(event *mpvipc.Event) int64 {
	if event == nil || event.ExtraData == nil {
		return 0
	}
	value, _ := event.ExtraData["playlist_entry_id"].(float64)
	return int64(value)
}

func (m *mpvProcess) playlistPos() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPlaylistPos
}

func (m *mpvProcess) currentPlaylistPos() int {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()
	if conn != nil {
		if value, err := m.call("get_property", "playlist-pos"); err == nil {
			if number, ok := value.(float64); ok && number >= 0 {
				position := int(number)
				m.mu.Lock()
				m.lastPlaylistPos = position
				m.mu.Unlock()
				return position
			}
		}
	}
	return m.playlistPos()
}

// exitError reports a crash, but treats a requested shutdown (window closed,
// application quitting or the final episode ending) as a normal finish.
func (m *mpvProcess) exitError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping {
		return nil
	}
	return m.lastErr
}

// mpvCallTimeout bounds one IPC command. mpv answers in milliseconds; a hung
// player must not freeze the party guest loop or the UI forever.
const mpvCallTimeout = 15 * time.Second

func (m *mpvProcess) call(arguments ...interface{}) (interface{}, error) {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()
	if conn == nil {
		return nil, errors.New("mpv is not connected")
	}

	// mpvipc.Connection.Call has no deadline of its own and waits forever if
	// mpv stops replying. Run it aside so the caller can fail and close the
	// pipe; every other caller then fails fast instead of queueing behind a
	// command that will never be answered.
	type reply struct {
		value interface{}
		err   error
	}
	done := make(chan reply, 1)
	go func() {
		value, err := conn.Call(arguments...)
		done <- reply{value: value, err: err}
	}()
	select {
	case result := <-done:
		return result.value, result.err
	case <-time.After(mpvCallTimeout):
		// A player that ignores IPC for this long is unusable: close the pipe
		// and stop the process so the session finishes and can be retried.
		_ = conn.Close()
		if m.cmd != nil && m.cmd.Process != nil {
			_ = m.cmd.Process.Kill()
		}
		command := ""
		if len(arguments) > 0 {
			command = fmt.Sprint(arguments[0])
		}
		return nil, fmt.Errorf("mpv command %q timed out after %s", command, mpvCallTimeout)
	}
}

// streamOptions builds the per-file options of one playlist entry. Passing the
// headers to loadfile instead of changing the global http-header-fields property
// keeps every Filmlib episode's own theatre Referer and bearer token with its
// entry: a global property would be overwritten by the next preload while the
// current HLS file is still fetching segments.
func streamOptions(headers []string) map[string]string {
	options := make(map[string]string, 1)
	if len(headers) > 0 {
		options["http-header-fields"] = strings.Join(headers, ",")
	}
	return options
}

func (m *mpvProcess) load(url string, headers []string) error {
	// The fourth argument is per-file options; -1 is the placeholder for the
	// insertion index that loadfile expects since mpv 0.38.
	_, err := m.call("loadfile", url, "replace", -1, streamOptions(headers))
	return err
}

func (m *mpvProcess) insert(index int, url string, headers []string) error {
	// playlist-insert was removed from mpv's command interface. loadfile
	// insert-at is the supported way to add an entry at an exact position;
	// an index at or past the end appends the entry.
	_, err := m.call("loadfile", url, "insert-at", index, streamOptions(headers))
	return err
}

// jump switches to an already-loaded playlist entry. Its per-file headers were
// attached when the entry was inserted, so selecting the entry applies them.
func (m *mpvProcess) jump(index int) error {
	_, err := m.call("set_property", "playlist-pos", index)
	return err
}

func (m *mpvProcess) seek(seconds float64) error {
	if seconds <= 0 {
		return nil
	}
	_, err := m.call("seek", seconds, "absolute")
	return err
}

func (m *mpvProcess) addSubtitle(url, title string) error {
	return m.addSubtitleFlags(url, title, "select")
}

// addSubtitleFlags adds one external subtitle track. mpv treats "select" as
// "make this track active" and "auto" as "add it but keep the current track",
// which is how several caption variants stay switchable in the player menu.
func (m *mpvProcess) addSubtitleFlags(url, title, flags string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	if strings.TrimSpace(flags) == "" {
		flags = "select"
	}
	arguments := []interface{}{"sub-add", url, flags}
	if title = strings.TrimSpace(title); title != "" {
		arguments = append(arguments, title)
	}
	_, err := m.call(arguments...)
	return err
}

func (m *mpvProcess) setChapters(chapters []metadata.Chapter) error {
	native := make([]map[string]interface{}, 0, len(chapters))
	for _, chapter := range chapters {
		native = append(native, map[string]interface{}{
			"title": chapter.Title,
			"time":  chapter.Time,
		})
	}
	_, err := m.call("set_property", "chapter-list", native)
	return err
}

func (m *mpvProcess) quit() {
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()
	if m.conn != nil {
		_, _ = m.call("quit")
	}
}

func (m *mpvProcess) close() {
	m.quit()
	select {
	case <-m.done:
	case <-time.After(3 * time.Second):
		if m.cmd != nil && m.cmd.Process != nil {
			_ = m.cmd.Process.Kill()
		}
		<-m.done
	}
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func findMPV() (string, error) {
	return mpv.Locate()
}

// geometryString converts a physical window rectangle to mpv's --geometry form.
// The size is deliberately omitted: mpv knows best how large to open its own
// window, and forcing the application rectangle made the player inherit a size
// it never chose. Only the remembered position is preserved.
func geometryString(x, y, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("%+d%+d", x, y)
}
