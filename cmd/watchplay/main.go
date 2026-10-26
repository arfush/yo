// Command watchplay is a console watch-party client. It drives the real mpv
// playback service and room protocol without the desktop UI, so the sync
// behaviour can be tested from two terminals:
//
//	watchplay create <slug> <озвучка> <серия>
//	watchplay open <ключ комнаты> [секрет]
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arfush/yonime/internal/appdata"
	"github.com/arfush/yonime/internal/browser"
	"github.com/arfush/yonime/internal/library"
	"github.com/arfush/yonime/internal/playback"
	"github.com/arfush/yonime/party"
)

const (
	// The host publishes often enough for a late guest to converge quickly and
	// rarely enough to keep the free relay quota intact.
	heartbeatPlaying = 4 * time.Second
	heartbeatIdle    = 10 * time.Second
	syncTick         = time.Second
	roomTimeout      = 20 * time.Second
	loadTimeout      = 3 * time.Minute

	// Drift below a second is fixed by a gentle rate change, anything larger by
	// an exact seek that leads the host a little.
	seekThreshold  = 0.9
	speedThreshold = 0.22
	seekLead       = 0.15
	speedGain      = 0.25
	minRate        = 0.94
	maxRate        = 1.06

	// A guest whose mpv failed retries after a pause; closing the player on
	// purpose leaves the room, exactly like the desktop app.
	retryCooldown = 45 * time.Second

	// A guest that just sent a command must not let the drift loop undo it
	// before the host's confirming anchor arrives.
	controlHold = 1500 * time.Millisecond
	// A jump larger than this between two ticks is a person seeking in mpv.
	userSeekThreshold = 1.5
)

var errUsage = errors.New("неверные аргументы")

func main() {
	providerFlag := flag.String("provider", "", "реле для создания комнаты: cloudflare или supabase")
	flag.Usage = usage
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, *providerFlag, flag.Args()); err != nil {
		if errors.Is(err, errUsage) {
			usage()
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "watchplay: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, "watchplay — консольный клиент совместного просмотра Yonime.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Использование:")
	fmt.Fprintln(out, "  watchplay create <slug AnimeLIB> <озвучка> <серия>")
	fmt.Fprintln(out, "  watchplay open <ключ комнаты> [секрет]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Флаги:")
	flag.PrintDefaults()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Реле берётся из watchplay.json рядом с программой, из")
	fmt.Fprintf(out, "%s\n", partyConfigHint())
	fmt.Fprintln(out, "или из адресов, вшитых при сборке.")
}

func partyConfigHint() string {
	if path, err := appdata.PartyConfigPath(); err == nil {
		return path
	}
	return "%APPDATA%\\Yonime\\party.json"
}

type command struct {
	name string
	args []string
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, errUsage
	}
	switch strings.ToLower(args[0]) {
	case "create":
		if len(args) != 4 {
			return command{}, fmt.Errorf("%w: create требует slug, озвучку и серию", errUsage)
		}
		return command{name: "create", args: args[1:]}, nil
	case "open":
		if len(args) < 2 || len(args) > 3 {
			return command{}, fmt.Errorf("%w: open требует ключ комнаты и, если комната закрыта, секрет", errUsage)
		}
		return command{name: "open", args: args[1:]}, nil
	case "help", "-h", "--help":
		return command{name: "help"}, nil
	}
	return command{}, fmt.Errorf("%w: неизвестная команда %q", errUsage, args[0])
}

func run(ctx context.Context, providerFlag string, args []string) error {
	cmd, err := parseCommand(args)
	if err != nil {
		return err
	}
	if cmd.name == "help" {
		usage()
		return nil
	}

	cfg, source, err := loadConfig()
	if err != nil {
		return err
	}
	var provider party.Provider
	if cmd.name == "create" {
		provider, err = chooseProvider(cfg, providerFlag)
	} else {
		// The host's key names the relay; --provider is a create-only flag.
		provider, err = party.ProviderFromKey(cmd.args[0])
	}
	if err != nil {
		return err
	}
	if !cfg.Configured(provider) {
		return fmt.Errorf("%s не настроен", provider.Label())
	}
	fmt.Printf("Реле: %s (%s)\n", provider.Label(), source)

	databasePath, err := appdata.DatabasePath()
	if err != nil {
		return err
	}
	tokenPath, err := appdata.TokenPath()
	if err != nil {
		return err
	}
	lib, err := library.Open(databasePath, tokenPath)
	if err != nil {
		return fmt.Errorf("открыть базу: %w", err)
	}
	defer lib.Close()

	manager := browser.New()
	service := playback.New(lib, manager)
	defer service.Close()

	a := &app{lib: lib, playback: service, rate: 1}
	switch cmd.name {
	case "create":
		return a.host(ctx, cfg, provider, cmd.args)
	case "open":
		return a.guest(ctx, cfg, cmd.args[0], cmd.args[1:])
	}
	return errUsage
}

// loadConfig reads the relay settings the same way the UI does, with one extra
// fallback: a watchplay.json next to the binary or in the working directory.
func loadConfig() (party.Config, string, error) {
	if path, err := appdata.PartyConfigPath(); err == nil {
		cfg := cleanConfig(party.LoadConfig(path))
		if usable(cfg) {
			return cfg, path, nil
		}
	}
	for _, path := range configCandidates() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		cfg := cleanConfig(party.LoadConfig(path))
		if usable(cfg) {
			if absolute, err := filepath.Abs(path); err == nil {
				return cfg, absolute, nil
			}
			return cfg, path, nil
		}
	}
	return party.Config{}, "", errors.New("реле не настроено: заполните watchplay.json своим адресом")
}

func configCandidates() []string {
	paths := []string{"watchplay.json"}
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), "watchplay.json"))
	}
	return paths
}

func usable(cfg party.Config) bool {
	return cfg.Configured(party.Cloudflare) || cfg.Configured(party.Supabase)
}

// cleanConfig drops the placeholders shipped in the repository file. Without
// this a plain source build would pretend to have a relay and fail at connect
// time with a confusing message.
func cleanConfig(cfg party.Config) party.Config {
	if isPlaceholder(cfg.CloudflareURL) {
		cfg.CloudflareURL = ""
	}
	if isPlaceholder(cfg.SupabaseURL) {
		cfg.SupabaseURL = ""
	}
	if isPlaceholder(cfg.SupabaseKey) {
		cfg.SupabaseKey = ""
	}
	return cfg
}

func isPlaceholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "example.") ||
		strings.Contains(value, "abcdefghijkl") ||
		strings.HasSuffix(value, "...")
}

func chooseProvider(cfg party.Config, wanted string) (party.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(wanted)) {
	case "":
		if cfg.Configured(party.Cloudflare) {
			return party.Cloudflare, nil
		}
		if cfg.Configured(party.Supabase) {
			return party.Supabase, nil
		}
		return "", errors.New("реле не настроено")
	case "cloudflare":
		if !cfg.Configured(party.Cloudflare) {
			return "", errors.New("Cloudflare не настроен")
		}
		return party.Cloudflare, nil
	case "supabase":
		if !cfg.Configured(party.Supabase) {
			return "", errors.New("Supabase не настроен")
		}
		return party.Supabase, nil
	}
	return "", fmt.Errorf("неизвестное реле %q: используйте cloudflare или supabase", wanted)
}

type mediaMeta struct {
	slug    string
	name    string
	voice   string
	episode string
}

// app owns the single mpv session and the room. It mirrors what ui/party.go
// does for the desktop UI, only without the window and repaint plumbing.
type app struct {
	lib      *library.Library
	playback *playback.Service

	mu       sync.Mutex
	session  *playback.Session
	meta     mediaMeta
	starting bool
	done     party.State
	doneAt   time.Time

	// epoch identifies the newest local player. A finish callback from a
	// session that has already been replaced must not clear its successor.
	epoch int

	// role names the mpv window, so a host and a guest started on one machine
	// do not look like two identical players.
	role string

	// rate is only touched by the guest loop goroutine.
	rate float64

	// The guest loop samples mpv between ticks to notice input the person made
	// in the player window. These fields are also guest-loop-only.
	lastPos    float64
	lastSample time.Time
	lastPaused bool
	holdUntil  time.Time
	seeded     bool

	// leave closes the command loop when the guest's player is closed. The
	// guest then leaves the room instead of waiting for the host.
	leave func()
}

func (a *app) host(ctx context.Context, cfg party.Config, provider party.Provider, args []string) error {
	slug, voice, episode := args[0], args[1], args[2]
	a.role = "создатель"

	roomCtx, roomCancel := context.WithTimeout(ctx, roomTimeout)
	room, err := party.Create(roomCtx, cfg, provider, "watchplay-host")
	roomCancel()
	if err != nil {
		return err
	}
	defer room.Close()
	fmt.Printf("Комната создана: %s\n", room.Key)
	fmt.Printf("Гость подключается так: watchplay open %s\n", room.Key)

	fmt.Printf("Загружаю %s (озвучка %q, серия %s)…\n", slug, voice, episode)
	loadCtx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()
	title, err := a.lib.Load(loadCtx, slug)
	if err != nil {
		return err
	}
	plan, err := a.lib.PlaybackPlan(loadCtx, title, voice, episode)
	if err != nil {
		return err
	}

	if _, err := a.start(loadCtx, plan, plan.ResumeAt, false, nil); err != nil {
		return err
	}
	fmt.Printf("Смотрю: %s, серия %s, озвучка %s\n", plan.Russian, plan.Episode, plan.Voice)

	room.SetSource(a.hostState)
	if state, ok := a.hostState(); ok {
		_ = room.Publish(state)
	}
	go a.hostLoop(ctx, room)
	go a.controlLoop(ctx, room)

	a.printCommands(true)
	return a.commands(ctx, room, true)
}

func (a *app) guest(ctx context.Context, cfg party.Config, key string, extra []string) error {
	roomCtx, cancel := context.WithTimeout(ctx, roomTimeout)
	room, err := party.Join(roomCtx, cfg, key, "watchplay-guest")
	cancel()
	if err != nil {
		return err
	}
	defer room.Close()
	a.role = "гость"
	// Wait for the host before waiting for its card: a reachable relay with an
	// empty room must report "room not found" instead of the version-mismatch
	// error that WaitCard returns when no one answers.
	hostCtx, hostCancel := context.WithTimeout(ctx, roomTimeout)
	err = room.WaitHost(hostCtx)
	hostCancel()
	if err != nil {
		return err
	}
	cardCtx, cardCancel := context.WithTimeout(ctx, roomTimeout)
	err = room.WaitCard(cardCtx)
	cardCancel()
	if err != nil {
		return err
	}
	if room.NeedsSecret() {
		if len(extra) == 0 {
			return errors.New("комната закрыта секретом: watchplay open <ключ> <секрет>")
		}
		if err := room.SubmitSecret(extra[0]); err != nil {
			return err
		}
		fmt.Printf("Секрет принят: %s\n", party.FormatSecret(room.Secret()))
	}
	// Closing mpv ends the session: the command loop stops and the room is
	// closed by the deferred Close above.
	ctx, leave := context.WithCancel(ctx)
	defer leave()
	a.mu.Lock()
	a.leave = leave
	a.mu.Unlock()
	fmt.Printf("Комната: %s\n", room.Key)
	fmt.Println("Ожидаю создателя комнаты…")

	go a.guestLoop(ctx, room)

	a.printCommands(false)
	return a.commands(ctx, room, false)
}

// start launches mpv through the regular playback service and remembers the
// session for the sync loops.
func (a *app) start(ctx context.Context, plan *library.PlaybackPlan, resumeAt float64, inputLocked bool, onFinish func(error)) (*playback.Session, error) {
	plan.ResumeAt = resumeAt
	title := "Yonime"
	if a.role != "" {
		title += " — " + a.role
	}
	a.mu.Lock()
	a.epoch++
	epoch := a.epoch
	a.mu.Unlock()
	// A guest follows the host's timeline; only the room owner skips chapters.
	autoSkip := a.lib.AutoSkipSegments()
	if a.role == "гость" {
		autoSkip = nil
	}
	options := playback.StartOptions{
		Quality:          a.lib.PreferredQuality(),
		Title:            title,
		AutoSkipSegments: autoSkip,
		InputLocked:      inputLocked,
		OnFinish: func(err error) {
			a.mu.Lock()
			if a.epoch != epoch {
				// A newer player already owns the room; this one is history.
				a.mu.Unlock()
				return
			}
			a.session = nil
			a.mu.Unlock()
			if onFinish != nil {
				onFinish(err)
				return
			}
			if err != nil {
				fmt.Printf("\nmpv: %v\n", err)
			} else {
				fmt.Println("\nВоспроизведение завершено.")
			}
		},
	}
	session, err := a.playback.Start(ctx, plan, options)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.epoch != epoch {
		// The player exited (or was replaced) while Start was returning. The
		// finish callback has already reported it, so do not resurrect it.
		a.mu.Unlock()
		return session, nil
	}
	a.session = session
	a.meta = mediaMeta{
		slug:    plan.Slug,
		name:    plan.Russian,
		voice:   plan.Voice,
		episode: plan.Episode,
	}
	a.mu.Unlock()
	return session, nil
}

func (a *app) current() *playback.Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session
}

// hostState snapshots the running mpv session into a room anchor.
func (a *app) hostState() (party.State, bool) {
	a.mu.Lock()
	session := a.session
	meta := a.meta
	a.mu.Unlock()
	if session == nil || meta.slug == "" {
		return party.State{}, false
	}
	number, index := session.Episode()
	if number == "" {
		number = meta.episode
	}
	control := a.lib.SharedControlEnabled()
	return party.State{
		TitleSlug:      meta.slug,
		TitleName:      meta.name,
		Voice:          meta.voice,
		Episode:        number,
		EpisodeIndex:   index,
		Playing:        !session.Paused(),
		Position:       session.LivePosition(),
		ControlEnabled: &control,
	}, true
}

// hostLoop publishes anchors while the host is watching. The interval shrinks
// to four seconds during playback and grows to ten on pause.
func (a *app) hostLoop(ctx context.Context, room *party.Session) {
	ticker := time.NewTicker(syncTick)
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
		case <-ctx.Done():
			room.SetSource(nil)
			return
		case now = <-ticker.C:
		}

		if a.current() == nil {
			// mpv is gone: clear the room so guests stop instead of
			// extrapolating a finished episode forever.
			room.SetSource(nil)
			_ = room.Publish(party.State{})
			return
		}

		state, ok := a.hostState()
		if !ok {
			continue
		}
		expected := lastPosition
		if lastPlaying && !lastSample.IsZero() {
			expected += now.Sub(lastSample).Seconds()
		}
		seeked := !lastSample.IsZero() && math.Abs(state.Position-expected) > 1.5
		changed := first || state.Episode != lastEpisode || state.Playing != lastPlaying
		interval := heartbeatPlaying
		if !state.Playing {
			interval = heartbeatIdle
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

func (a *app) guestLoop(ctx context.Context, room *party.Session) {
	ticker := time.NewTicker(syncTick)
	defer ticker.Stop()
	hostSeen := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if room.HostGone() {
			// The host closed the room or its reconnect grace window expired.
			// A brief disconnect keeps the guest playing on the last anchor,
			// exactly like in the desktop app.
			fmt.Println("\nСоздатель комнаты вышел — выхожу.")
			a.stopGuest()
			a.mu.Lock()
			leave := a.leave
			a.mu.Unlock()
			if leave != nil {
				leave()
			}
			return
		}
		if !room.HostSeen() {
			// The first greeting has not arrived, or the host is inside its
			// reconnect window; keep following the last anchor either way.
			if !hostSeen {
				continue
			}
		}
		hostSeen = true
		state, ok := room.State()
		if !ok {
			continue
		}
		if !state.Started() {
			if a.stopGuest() {
				fmt.Println("Создатель закрыл воспроизведение.")
			}
			continue
		}
		a.guestTick(ctx, room, state)
	}
}

func (a *app) guestTick(ctx context.Context, room *party.Session, state party.State) {
	a.mu.Lock()
	starting := a.starting
	current := a.session
	done := a.done
	doneAt := a.doneAt
	a.mu.Unlock()
	if starting {
		return
	}
	if current != nil {
		number, _ := current.Episode()
		if number == "" || number == state.Episode {
			// mpv is still opening the file, or it already plays the right
			// episode and only needs the drift correction.
			a.drift(room, state, current)
			return
		}
		a.restartGuest(ctx, room, state)
		return
	}
	if done.SamePlayback(state) && (doneAt.IsZero() || time.Since(doneAt) < retryCooldown) {
		return
	}
	a.restartGuest(ctx, room, state)
}

// restartGuest opens the media the host is watching and seeks to the
// extrapolated position. Resolving takes seconds, so it runs off the tick.
func (a *app) restartGuest(ctx context.Context, room *party.Session, state party.State) {
	a.mu.Lock()
	if a.starting {
		a.mu.Unlock()
		return
	}
	a.starting = true
	previous := a.session
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.starting = false
			a.mu.Unlock()
		}()
		if previous != nil {
			previous.Stop()
			deadline := time.Now().Add(3 * time.Second)
			for a.playback.Active() && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if a.playback.Active() {
				a.guestFailed(state, errors.New("mpv не успел завершить прошлую серию"))
				return
			}
		}

		startCtx, cancel := context.WithTimeout(ctx, loadTimeout)
		defer cancel()
		title, err := a.lib.Load(startCtx, state.TitleSlug)
		if err != nil {
			a.guestFailed(state, err)
			return
		}
		plan, err := a.lib.PlaybackPlan(startCtx, title, state.Voice, state.Episode)
		if err != nil {
			a.guestFailed(state, err)
			return
		}
		position := 0.0
		if want, _, ok := room.ExpectedPosition(time.Now()); ok {
			position = want
		}
		if _, err := a.start(startCtx, plan, position, !a.guestControlAllowed(state), func(err error) {
			a.guestFinished(state, err)
		}); err != nil {
			a.guestFailed(state, err)
			return
		}
		if a.current() == nil {
			// mpv died before Start returned; the finish callback has already
			// told the user, so do not claim the episode is playing.
			return
		}
		fmt.Printf("\nСмотрю: %s, серия %s, озвучка %s\n", plan.Russian, plan.Episode, plan.Voice)
	}()
}

// guestControlAllowed mirrors the desktop UI: the host's anchor decides, and
// only a room from an older build falls back to the local setting.
func (a *app) guestControlAllowed(state party.State) bool {
	if allowed, ok := state.GuestsControl(); ok {
		return allowed
	}
	return a.lib.SharedControlEnabled()
}

// drift keeps the local mpv close to the extrapolated host position. It also
// forwards input the person makes in the guest's mpv when shared control is on.
func (a *app) drift(room *party.Session, state party.State, current *playback.Session) {
	allowed := a.guestControlAllowed(state)
	_ = current.SetInputLocked(!allowed)
	number, _ := current.Episode()
	if number == "" || number != state.Episode {
		a.rate = 1
		a.seeded = false
		return
	}
	now := time.Now()
	position := current.LivePosition()
	paused := current.Paused()
	if !a.seeded {
		a.seeded = true
	} else if allowed {
		// Between ticks only a person can change the player: the drift loop
		// acts below and records its own corrections in the baseline.
		switch {
		case paused != a.lastPaused:
			action := party.ControlPlay
			if paused {
				action = party.ControlPause
			}
			_ = room.SendControl(action, 0)
			a.holdUntil = now.Add(controlHold)
		case !a.lastPaused && !a.lastSample.IsZero():
			expected := a.lastPos + now.Sub(a.lastSample).Seconds()
			// A frozen position means a network stall, not a seek: do not drag
			// the host back to a stalled guest.
			if position != a.lastPos && math.Abs(position-expected) > userSeekThreshold {
				_ = room.SendControl(party.ControlSeek, position)
				a.holdUntil = now.Add(controlHold)
			}
		}
	}
	a.lastPaused = paused
	a.lastSample = now
	tracked := position
	if now.Before(a.holdUntil) {
		// The command is still travelling; correcting the drift now would undo
		// the guest's own pause or seek before the host confirms it.
		a.lastPos = tracked
		return
	}

	if !state.Playing {
		if !current.Paused() {
			_ = current.SetPaused(true)
		}
		if math.Abs(position-state.Position) > 0.8 {
			_ = current.SeekTo(state.Position)
			tracked = state.Position
		}
		if a.rate != 1 {
			_ = current.SetSpeed(1)
			a.rate = 1
		}
		a.lastPaused = current.Paused()
		a.lastPos = tracked
		return
	}
	if current.Paused() {
		_ = current.SetPaused(false)
	}
	want, _, ok := room.ExpectedPosition(now)
	if !ok {
		a.lastPaused = current.Paused()
		a.lastPos = tracked
		return
	}
	drift := position - want
	switch {
	case math.Abs(drift) > seekThreshold:
		target := want + seekLead
		_ = current.SeekTo(target)
		tracked = target
		if a.rate != 1 {
			_ = current.SetSpeed(1)
			a.rate = 1
		}
	case math.Abs(drift) > speedThreshold:
		next := math.Max(minRate, math.Min(maxRate, 1-drift*speedGain))
		if math.Abs(next-a.rate) > 0.004 {
			_ = current.SetSpeed(next)
			a.rate = next
		}
	default:
		if a.rate != 1 {
			_ = current.SetSpeed(1)
			a.rate = 1
		}
	}
	// Record what we just enforced, otherwise the next tick would mistake our
	// own pause or seek for input from the person.
	a.lastPaused = current.Paused()
	a.lastPos = tracked
}

// controlLoop applies guest commands on the host. Shared control is the host's
// decision, so a command is ignored when the setting is off.
func (a *app) controlLoop(ctx context.Context, room *party.Session) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-room.Updates():
			if update.Kind != party.UpdateControl || !a.lib.SharedControlEnabled() {
				continue
			}
			current := a.current()
			if current == nil {
				continue
			}
			switch update.Control.Action {
			case party.ControlPause:
				_ = current.SetPaused(true)
			case party.ControlPlay:
				_ = current.SetPaused(false)
			case party.ControlSeek:
				_ = current.SeekTo(update.Control.Position)
			default:
				continue
			}
			a.publish(room)
			fmt.Println("\nГость изменил воспроизведение.")
		}
	}
}

func (a *app) stopGuest() bool {
	a.mu.Lock()
	running := a.session
	a.session = nil
	a.mu.Unlock()
	if running == nil {
		return false
	}
	running.Stop()
	return true
}

func (a *app) guestFailed(state party.State, err error) {
	a.mu.Lock()
	a.done = state
	a.doneAt = time.Now()
	a.mu.Unlock()
	fmt.Printf("\nНе удалось включить серию: %v\n", err)
}

func (a *app) guestFinished(state party.State, err error) {
	if err == nil {
		// Closing the player is how a guest leaves the room. Waiting for the
		// host here kept a room alive that nobody was watching any more.
		fmt.Println("\nПлеер закрыт — выхожу из комнаты.")
		a.mu.Lock()
		leave := a.leave
		a.mu.Unlock()
		if leave != nil {
			leave()
		}
		return
	}
	a.mu.Lock()
	a.done = state
	a.doneAt = time.Now()
	a.mu.Unlock()
	fmt.Printf("\nmpv: %v\n", err)
	fmt.Println("Плеер закрылся с ошибкой; попробую открыть его снова.")
}

func (a *app) control(action func(*playback.Session) error) error {
	current := a.current()
	if current == nil {
		return playback.ErrNotRunning
	}
	return action(current)
}

func (a *app) printCommands(host bool) {
	fmt.Println()
	if host {
		fmt.Println("Команды: pause, play, seek <сек|+сек|-сек>, lock, unlock, status, quit")
	} else {
		fmt.Println("Команды: pause, play, seek <сек|+сек|-сек>, secret <секрет>, status, reopen, quit")
		fmt.Println("Управлять можно, если создатель включает совместное управление.")
	}
	fmt.Println()
}

func (a *app) commands(ctx context.Context, room *party.Session, host bool) error {
	lines := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		close(lines)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-lines:
			if !ok {
				// Piped input or a closed console must not kill playback;
				// Ctrl+C still does.
				fmt.Println("Ввод закрыт, комната работает до Ctrl+C.")
				<-ctx.Done()
				return nil
			}
			if !a.runCommand(ctx, line, room, host) {
				return nil
			}
		}
	}
}

// runCommand executes one typed command and reports whether to keep going.
func (a *app) runCommand(ctx context.Context, line string, room *party.Session, host bool) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return true
	}
	switch strings.ToLower(fields[0]) {
	case "help", "?":
		a.printCommands(host)
	case "status", "st":
		a.printStatus(room, host)
	case "lock", "unlock":
		if !host {
			fmt.Println("Закрыть комнату может только создатель.")
			return true
		}
		if strings.ToLower(fields[0]) == "unlock" {
			if err := room.Unlock(); err != nil {
				fmt.Println("комната:", err)
				return true
			}
			fmt.Println("Комната открыта: гостю достаточно ключа.")
			return true
		}
		secret, err := party.NewSecret()
		if err != nil {
			fmt.Println("секрет:", err)
			return true
		}
		if _, err := room.Lock(secret); err != nil {
			fmt.Println("комната:", err)
			return true
		}
		fmt.Printf("Комната закрыта. Секрет: %s\n", party.FormatSecret(secret))
		fmt.Printf("Гость: watchplay open %s %s\n", room.Key, secret)
	case "secret":
		if host {
			fmt.Println("Секрет вводит гость; создатель меняет его командой lock.")
			return true
		}
		if len(fields) < 2 {
			fmt.Println("Использование: secret <секрет>")
			return true
		}
		if err := room.SubmitSecret(fields[1]); err != nil {
			fmt.Println("секрет:", err)
			return true
		}
		fmt.Println("Секрет принят.")
	case "reopen":
		if host {
			fmt.Println("Команда только для гостя: свой плеер создатель открывает заново самим запуском.")
			return true
		}
		if a.current() != nil {
			fmt.Println("Плеер уже открыт.")
			return true
		}
		state, ok := room.State()
		if !ok || !state.Started() {
			fmt.Println("Создатель ещё не включил серию.")
			return true
		}
		// Forget the finish so the guest loop starts this media again instead
		// of treating the closed player as a deliberate exit.
		a.mu.Lock()
		a.done = party.State{}
		a.doneAt = time.Time{}
		a.mu.Unlock()
		fmt.Println("Открываю плеер заново…")
		a.restartGuest(ctx, room, state)
	case "pause", "play":
		pause := strings.ToLower(fields[0]) == "pause"
		if !host {
			if !a.lib.SharedControlEnabled() {
				fmt.Println("Совместное управление выключено — команду отправить нельзя.")
				return true
			}
			action := party.ControlPlay
			if pause {
				action = party.ControlPause
			}
			if err := room.SendControl(action, 0); err != nil {
				fmt.Println("комната:", err)
				return true
			}
			fmt.Println("Команда отправлена создателю комнаты.")
			return true
		}
		err := a.control(func(current *playback.Session) error {
			return current.SetPaused(pause)
		})
		if err != nil {
			fmt.Println("mpv:", err)
			return true
		}
		a.publish(room)
		if pause {
			fmt.Println("Пауза.")
		} else {
			fmt.Println("Продолжаю.")
		}
	case "seek":
		if !host && !a.lib.SharedControlEnabled() {
			fmt.Println("Совместное управление выключено — команду отправить нельзя.")
			return true
		}
		if len(fields) < 2 {
			fmt.Println("Формат: seek <секунды|+секунды|-секунды>")
			return true
		}
		current := a.current()
		if current == nil {
			fmt.Println("mpv не запущен")
			return true
		}
		target, err := parseSeek(fields[1], current.LivePosition())
		if err != nil {
			fmt.Println(err)
			return true
		}
		if !host {
			if err := room.SendControl(party.ControlSeek, target); err != nil {
				fmt.Println("комната:", err)
				return true
			}
			fmt.Printf("Команда перемотки на %.1f с отправлена создателю комнаты.\n", target)
			return true
		}
		if err := current.SeekTo(target); err != nil {
			fmt.Println("mpv:", err)
			return true
		}
		a.publish(room)
		fmt.Printf("Перемотка на %.1f с.\n", target)
	case "quit", "exit", "q":
		return false
	default:
		fmt.Printf("Неизвестная команда %q. help — список команд.\n", fields[0])
	}
	return true
}

// parseSeek parses an absolute position or a relative offset in seconds.
func parseSeek(arg string, current float64) (float64, error) {
	arg = strings.TrimSpace(strings.ReplaceAll(arg, ",", "."))
	if arg == "" {
		return 0, errors.New("не понимаю позицию")
	}
	relative := arg[0] == '+' || arg[0] == '-'
	value, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return 0, fmt.Errorf("не понимаю позицию %q", arg)
	}
	if !relative {
		if value < 0 {
			value = 0
		}
		return value, nil
	}
	next := current + value
	if next < 0 {
		next = 0
	}
	return next, nil
}

func (a *app) publish(room *party.Session) {
	if state, ok := a.hostState(); ok {
		_ = room.Publish(state)
	}
}

func (a *app) printStatus(room *party.Session, host bool) {
	role := "гость"
	if host {
		role = "создатель"
	}
	fmt.Printf("Комната %s, роль: %s\n", room.Key, role)
	current := a.current()
	if current == nil {
		fmt.Println("mpv не запущен")
	} else {
		number, _ := current.Episode()
		fmt.Printf("Серия %s, позиция %.1f с, пауза: %t\n", number, current.LivePosition(), current.Paused())
	}
	state, ok := room.State()
	if !ok {
		fmt.Println("Якорей от создателя ещё не было")
		return
	}
	if !state.Started() {
		fmt.Println("Создатель ещё не включил серию")
		return
	}
	want, _, ok := room.ExpectedPosition(time.Now())
	if !ok {
		fmt.Println("Якорь ещё не готов")
		return
	}
	drift := 0.0
	if current != nil {
		drift = current.LivePosition() - want
	}
	fmt.Printf("Хост: %s, серия %s, ожидаемая позиция %.1f с, расхождение %.2f с\n",
		state.TitleSlug, state.Episode, want, drift)
}
