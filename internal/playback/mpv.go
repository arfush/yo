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

	"github.com/arfush/yoanime/internal/metadata"
	"github.com/arfush/yoanime/internal/mpv"
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
	Paused   bool
	Reason   string
	Err      error
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
}

// inputLockArgs disables the player's built-in controls for a guest whose host
// does not allow shared control. The native window controls keep the guest able
// to leave.
func inputLockArgs(locked bool) []string {
	if !locked {
		return nil
	}
	// The timeline is blocked by the yoanime-no-timeline input section, not by
	// disabling every binding. Only the OSC has to go: its buttons and seek bar
	// would bypass the section.
	return []string{"--osc=no"}
}

func startMPV(ctx context.Context, geometry, inputConfPath, title string, inputLocked bool) (*mpvProcess, error) {
	path, err := findMPV()
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "yoanime"
	}
	pipe := mpvPipePath("yoanime-mpv-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36))
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
		"--http-header-fields=Referer: https://v5.animelib.org/,Origin: https://v5.animelib.org",
		"--background-color=#0d0e14",
	}
	if geometry != "" {
		args = append(args, "--geometry="+geometry)
	}
	args = append(args, inputLockArgs(inputLocked)...)
	if inputConfPath != "" {
		args = append(args, "--input-conf="+inputConfPath)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	configureMPVCommand(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("запустить mpv: %w", err)
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
		return nil, fmt.Errorf("не удалось подключиться к mpv: %w", openErr)
	}
	m.conn = conn
	if inputLocked {
		if _, err := conn.Call("enable-section", guestInputSection); err != nil {
			_ = conn.Close()
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
			return nil, fmt.Errorf("включить блокировку перемотки mpv: %w", err)
		}
	}
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
				m.send(mpvEvent{Kind: mpvEventEndFile, Reason: event.Reason, Position: m.playlistPos()})
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

func (m *mpvProcess) playlistPos() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPlaylistPos
}

func (m *mpvProcess) currentPlaylistPos() int {
	if m.conn != nil {
		if value, err := m.conn.Call("get_property", "playlist-pos"); err == nil {
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

func (m *mpvProcess) call(arguments ...interface{}) (interface{}, error) {
	if m.conn == nil {
		return nil, errors.New("mpv не подключён")
	}
	return m.conn.Call(arguments...)
}

// setHeaders replaces the HTTP headers mpv sends for the next file. mpv takes a
// single comma-separated string, so header values must not contain commas.
func (m *mpvProcess) setHeaders(headers []string) error {
	if len(headers) == 0 {
		return nil
	}
	_, err := m.call("set_property", "http-header-fields", strings.Join(headers, ","))
	return err
}

func (m *mpvProcess) load(url string, headers []string) error {
	if err := m.setHeaders(headers); err != nil {
		return err
	}
	_, err := m.call("loadfile", url, "replace")
	return err
}

func (m *mpvProcess) insert(index int, url string, headers []string) error {
	if err := m.setHeaders(headers); err != nil {
		return err
	}
	// playlist-insert was removed from mpv's command interface. loadfile
	// insert-at is the supported way to add an entry at an exact position;
	// an index at or past the end appends the entry.
	_, err := m.call("loadfile", url, "insert-at", index)
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
	if m.conn != nil {
		_ = m.conn.Close()
	}
}

func findMPV() (string, error) {
	return mpv.Locate()
}

// geometryString converts a physical window rectangle to mpv's --geometry form.
func geometryString(x, y, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d%+d%+d", width, height, x, y)
}
