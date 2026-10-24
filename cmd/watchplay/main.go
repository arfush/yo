// Command watchplay is a console watch-party client. It drives the real mpv
// playback service and room protocol without the desktop UI, so the sync
// behaviour can be tested from two terminals:
//
//	watchplay create <slug> <озвучка> <серия>
//	watchplay open <ключ комнаты>
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
	"github.com/arfush/yonime/internal/watchparty"
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

	// A guest whose mpv failed to start retries after a pause; a player the
	// guest closed on purpose stays closed until the host changes the media.
	retryCooldown = 45 * time.Second
)

var errUsage = errors.New("неверные аргументы")

func main() {
	providerFlag := flag.String("provider", "", "реле: cloudflare или supabase")
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
	fmt.Fprintln(out, "  watchplay open <ключ комнаты>")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Флаги:")
	flag.PrintDefaults()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Реле берётся из watchplay.json рядом с программой, из")
	fmt.Fprintf(out, "%s\n", watchpartyConfigHint())
	fmt.Fprintln(out, "или из адресов, вшитых при сборке.")
}

func watchpartyConfigHint() string {
	if path, err := appdata.WatchPartyConfigPath(); err == nil {
		return path
	}
	return "%APPDATA%\\yonime\\watchparty.json"
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
		if len(args) != 2 {
			return command{}, fmt.Errorf("%w: open требует ключ комнаты", errUsage)
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
	provider, err := chooseProvider(cfg, providerFlag)
	if err != nil {
		return err
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
		return a.guest(ctx, cfg, provider, cmd.args[0])
	}
	return errUsage
}

// loadConfig reads the relay settings the same way the UI does, with one extra
// fallback: a watchplay.json next to the binary or in the working directory.
func loadConfig() (watchparty.Config, string, error) {
	if path, err := appdata.WatchPartyConfigPath(); err == nil {
		cfg := cleanConfig(watchparty.LoadConfig(path))
		if usable(cfg) {
			return cfg, path, nil
		}
	}
	for _, path := range configCandidates() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		cfg := cleanConfig(watchparty.LoadConfig(path))
		if usable(cfg) {
			if absolute, err := filepath.Abs(path); err == nil {
				return cfg, absolute, nil
			}
			return cfg, path, nil
		}
	}
	return watchparty.Config{}, "", errors.New("реле не настроено: заполните watchplay.json своим адресом")
}

func configCandidates() []string {
	paths := []string{"watchplay.json"}
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), "watchplay.json"))
	}
	return paths
}

func usable(cfg watchparty.Config) bool {
	return cfg.Configured(watchparty.Cloudflare) || cfg.Configured(watchparty.Supabase)
}

// cleanConfig drops the placeholders shipped in the repository file. Without
// this a plain source build would pretend to have a relay and fail at connect
// time with a confusing message.
func cleanConfig(cfg watchparty.Config) watchparty.Config {
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

func chooseProvider(cfg watchparty.Config, wanted string) (watchparty.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(wanted)) {
	case "":
		if cfg.Configured(watchparty.Cloudflare) {
			return watchparty.Cloudflare, nil
		}
		if cfg.Configured(watchparty.Supabase) {
			return watchparty.Supabase, nil
		}
		return "", errors.New("реле не настроено")
	case "cloudflare":
		if !cfg.Configured(watchparty.Cloudflare) {
			return "", errors.New("Cloudflare не настроен")
		}
		return watchparty.Cloudflare, nil
	case "supabase":
		if !cfg.Configured(watchparty.Supabase) {
			return "", errors.New("Supabase не настроен")
		}
		return watchparty.Supabase, nil
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
	done     watchparty.State
	doneAt   time.Time

	// epoch identifies the newest local player. A finish callback from a
	// session that has already been replaced must not clear its successor.
	epoch int

	// role names the mpv window, so a host and a guest started on one machine
	// do not look like two identical players.
	role string

	// rate is only touched by the guest loop goroutine.
	rate float64
}

func (a *app) host(ctx context.Context, cfg watchparty.Config, provider watchparty.Provider, args []string) error {
	slug, voice, episode := args[0], args[1], args[2]
	a.role = "создатель"

	roomCtx, roomCancel := context.WithTimeout(ctx, roomTimeout)
	room, err := watchparty.Create(roomCtx, cfg, provider)
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

	if _, err := a.start(loadCtx, plan, plan.ResumeAt, nil); err != nil {
		return err
	}
	fmt.Printf("Смотрю: %s, серия %s, озвучка %s\n", plan.Russian, plan.Episode, plan.Voice)

	room.SetSource(a.hostState)
	if state, ok := a.hostState(); ok {
		_ = room.Publish(state)
	}
	go a.hostLoop(ctx, room)

	a.printCommands(true)
	return a.commands(ctx, room, true)
}

func (a *app) guest(ctx context.Context, cfg watchparty.Config, provider watchparty.Provider, key string) error {
	roomCtx, cancel := context.WithTimeout(ctx, roomTimeout)
	room, err := watchparty.Join(roomCtx, cfg, provider, key)
	cancel()
	if err != nil {
		return err
	}
	defer room.Close()
	a.role = "гость"
	fmt.Printf("Комната: %s\n", room.Key)
	fmt.Println("Ожидаю создателя комнаты…")

	go a.guestLoop(ctx, room)

	a.printCommands(false)
	return a.commands(ctx, room, false)
}

// start launches mpv through the regular playback service and remembers the
// session for the sync loops.
func (a *app) start(ctx context.Context, plan *library.PlaybackPlan, resumeAt float64, onFinish func(error)) (*playback.Session, error) {
	plan.ResumeAt = resumeAt
	title := "Yonime"
	if a.role != "" {
		title += " — " + a.role
	}
	a.mu.Lock()
	a.epoch++
	epoch := a.epoch
	a.mu.Unlock()
	options := playback.StartOptions{
		Quality: a.lib.PreferredQuality(),
		Title:   title,
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
func (a *app) hostState() (watchparty.State, bool) {
	a.mu.Lock()
	session := a.session
	meta := a.meta
	a.mu.Unlock()
	if session == nil || meta.slug == "" {
		return watchparty.State{}, false
	}
	number, index := session.Episode()
	if number == "" {
		number = meta.episode
	}
	return watchparty.State{
		TitleSlug:    meta.slug,
		TitleName:    meta.name,
		Voice:        meta.voice,
		Episode:      number,
		EpisodeIndex: index,
		Playing:      !session.Paused(),
		Position:     session.LivePosition(),
	}, true
}

// hostLoop publishes anchors while the host is watching. The interval shrinks
// to four seconds during playback and grows to ten on pause.
func (a *app) hostLoop(ctx context.Context, room *watchparty.Session) {
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
			_ = room.Publish(watchparty.State{})
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

func (a *app) guestLoop(ctx context.Context, room *watchparty.Session) {
	ticker := time.NewTicker(syncTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
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

func (a *app) guestTick(ctx context.Context, room *watchparty.Session, state watchparty.State) {
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
func (a *app) restartGuest(ctx context.Context, room *watchparty.Session, state watchparty.State) {
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
		if _, err := a.start(startCtx, plan, position, func(err error) {
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

// drift keeps the local mpv close to the extrapolated host position.
func (a *app) drift(room *watchparty.Session, state watchparty.State, current *playback.Session) {
	number, _ := current.Episode()
	if number == "" || number != state.Episode {
		a.rate = 1
		return
	}
	if !state.Playing {
		if !current.Paused() {
			_ = current.SetPaused(true)
		}
		if math.Abs(current.LivePosition()-state.Position) > 0.8 {
			_ = current.SeekTo(state.Position)
		}
		if a.rate != 1 {
			_ = current.SetSpeed(1)
			a.rate = 1
		}
		return
	}
	if current.Paused() {
		_ = current.SetPaused(false)
	}
	want, _, ok := room.ExpectedPosition(time.Now())
	if !ok {
		return
	}
	drift := current.LivePosition() - want
	switch {
	case math.Abs(drift) > seekThreshold:
		_ = current.SeekTo(want + seekLead)
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

func (a *app) guestFailed(state watchparty.State, err error) {
	a.mu.Lock()
	a.done = state
	a.doneAt = time.Now()
	a.mu.Unlock()
	fmt.Printf("\nНе удалось включить серию: %v\n", err)
}

func (a *app) guestFinished(state watchparty.State, err error) {
	a.mu.Lock()
	a.done = state
	// A zero cooldown keeps this media closed until the host moves on.
	a.doneAt = time.Time{}
	a.mu.Unlock()
	if err != nil {
		fmt.Printf("\nmpv: %v\n", err)
	}
	fmt.Println("Плеер закрылся; жду, пока создатель сменит серию.")
	fmt.Println("Вернуть его на текущую серию: reopen")
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
		fmt.Println("Команды: pause, play, seek <сек|+сек|-сек>, status, quit")
	} else {
		fmt.Println("Команды: status, reopen, quit (воспроизведением управляет создатель комнаты)")
	}
	fmt.Println()
}

func (a *app) commands(ctx context.Context, room *watchparty.Session, host bool) error {
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
func (a *app) runCommand(ctx context.Context, line string, room *watchparty.Session, host bool) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return true
	}
	switch strings.ToLower(fields[0]) {
	case "help", "?":
		a.printCommands(host)
	case "status", "st":
		a.printStatus(room, host)
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
		a.done = watchparty.State{}
		a.doneAt = time.Time{}
		a.mu.Unlock()
		fmt.Println("Открываю плеер заново…")
		a.restartGuest(ctx, room, state)
	case "pause", "play":
		if !host {
			fmt.Println("Воспроизведением управляет создатель комнаты.")
			return true
		}
		pause := strings.ToLower(fields[0]) == "pause"
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
		if !host {
			fmt.Println("Воспроизведением управляет создатель комнаты.")
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

func (a *app) publish(room *watchparty.Session) {
	if state, ok := a.hostState(); ok {
		_ = room.Publish(state)
	}
}

func (a *app) printStatus(room *watchparty.Session, host bool) {
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
