package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arfush/yo/animelib"
	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/kodik"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/log"
	"github.com/arfush/yo/metadata"
)

// WindowRect is the physical screen rectangle mpv should occupy. It is the
// rectangle of the application window before it is hidden.
type WindowRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// StartOptions configures one playback session.
type StartOptions struct {
	Quality int
	Window  WindowRect
	// AutoSkipSegments lists the introdb chapter types that playback jumps
	// over automatically. It is empty for a watch-party guest: the room owner
	// decides what is skipped.
	AutoSkipSegments map[metadata.Segment]bool
	// Title overrides the mpv window title. The desktop window keeps the
	// default; the watch-party console client adds the role so a host and a
	// guest on one machine can tell their players apart.
	Title    string
	OnReady  func()
	OnFinish func(error)
}

// Service owns the Kodik browser and the single active mpv session.
type Service struct {
	lib      *library.Library
	resolver *kodik.Resolver
	// filmlib resolves Alloha streams through its own headless browser. It is
	// created with the service and never started until a Filmlib title plays.
	filmlib *filmlib.Resolver

	mu       sync.Mutex
	session  *Session
	starting bool
	closed   bool
}

// New returns a playback service. mpv is located lazily when Start is called.
func New(lib *library.Library, manager *browser.Manager) *Service {
	return &Service{
		lib:      lib,
		resolver: kodik.New(manager),
		filmlib:  filmlib.NewResolver(manager),
	}
}

// Start resolves the selected episode, launches mpv and starts filling the
// playlist around it. The current episode is resolved before mpv opens; all
// other episodes are resolved in the background.
func (s *Service) Start(ctx context.Context, plan *library.PlaybackPlan, options StartOptions) (*Session, error) {
	if s == nil || s.lib == nil {
		return nil, ErrUnavailable
	}
	if plan == nil || len(plan.Episodes) == 0 {
		return nil, ErrNoEpisodes
	}
	logger.Info("playback starting", "title", plan.Russian, "slug", plan.Slug, "voice", plan.Voice, "episode_index", plan.EpisodeIndex, "episodes", len(plan.Episodes), "quality", options.Quality)
	if plan.EpisodeIndex < 0 || plan.EpisodeIndex >= len(plan.Episodes) {
		return nil, ErrInvalidEpisode
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrClosing
	}
	if s.session != nil || s.starting {
		s.mu.Unlock()
		return nil, ErrAlreadyPlaying
	}
	s.starting = true
	s.mu.Unlock()

	// Clear the guard on every failed path. On success the session takes over
	// below, so another Start can only run after it has finished.
	started := false
	defer func() {
		if started {
			return
		}
		s.mu.Lock()
		s.starting = false
		s.mu.Unlock()
	}()

	token, err := s.lib.Token()
	if err != nil {
		// Filmlib playback has nothing to do with the AnimeLIB account, so an
		// expired token must not block it.
		if plan.Source != filmlib.Source {
			logger.Error("cannot read token for playback", "error", err)
			return nil, err
		}
		token = ""
	}
	sessionCtx, cancel := context.WithCancel(context.Background())
	var client *animelib.Client
	if plan.Source != filmlib.Source {
		client = animelib.New(token)
	}
	session := &Session{
		service:         s,
		plan:            plan,
		options:         options,
		client:          client,
		ctx:             sessionCtx,
		cancel:          cancel,
		resolved:        make(map[int]*playlistEntry),
		details:         make(map[int]*animelib.EpisodeDetail),
		detailFlight:    make(map[int]chan struct{}),
		chapterSem:      make(chan struct{}, 2),
		quality:         options.Quality,
		lastWrite:       time.Time{},
		lastPos:         plan.ResumeAt,
		currentEp:       -1,
		chaptersNoData:  make(map[string]bool),
		autoSkip:        options.AutoSkipSegments,
		skippedChapters: make(map[int]bool),
	}
	if session.quality <= 0 {
		session.quality = 1080
	}

	// Resolve the episode the user asked for before showing an mpv window. If
	// this fails the application stays visible and shows the error.
	entry, err := session.resolveEntry(ctx, plan.EpisodeIndex)
	if err != nil {
		logger.Error("cannot resolve the selected episode", "episode_index", plan.EpisodeIndex, "error", err)
		cancel()
		return nil, err
	}
	geometry := geometryString(options.Window.X, options.Window.Y, options.Window.Width, options.Window.Height)
	mpv, err := startMPV(ctx, geometry, options.Title)
	if err != nil {
		logger.Error("cannot start mpv", "error", err)
		cancel()
		return nil, err
	}
	session.mpv = mpv
	if err := mpv.load(entry.URL, headersFor(entry)); err != nil {
		logger.Error("cannot load the first video", "episode", entry.Episode.Number, "error", err)
		mpv.close()
		cancel()
		return nil, fmt.Errorf("cannot start the video: %w", err)
	}
	session.entries = []*playlistEntry{entry}
	session.current = 0
	session.currentEp = plan.EpisodeIndex
	session.currentNumber = entry.Episode.Number
	session.resolved[plan.EpisodeIndex] = entry
	session.resumePending = plan.ResumeAt

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		mpv.close()
		cancel()
		return nil, ErrClosing
	}
	s.session = session
	s.starting = false
	started = true
	s.mu.Unlock()

	logger.Info("playback started", "title", plan.Russian, "episode", entry.Episode.Number)
	if options.OnReady != nil {
		options.OnReady()
	}
	go session.eventLoop()
	return session, nil
}

// Active reports whether a playback session is starting or running. It lets the
// UI keep the application window away while mpv owns the screen, including the
// short window between Start and the first OnReady callback.
func (s *Service) Active() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session != nil || s.starting
}

// Current returns the running session, if any. The watch-party guest loop uses
// it to keep mpv in step with the host.
func (s *Service) Current() *Session {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

// Close stops the active session and the Kodik browser.
func (s *Service) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	session := s.session
	s.session = nil
	s.mu.Unlock()
	logger.Info("playback service closing")
	if session != nil {
		session.stop()
	}
	if s.resolver != nil {
		_ = s.resolver.Close()
	}
	if s.filmlib != nil {
		_ = s.filmlib.Close()
	}
}

func (s *Service) clearSession(session *Session) {
	s.mu.Lock()
	if s.session == session {
		s.session = nil
	}
	s.mu.Unlock()
}

type playlistEntry struct {
	EpisodeIndex  int
	Episode       library.PlaybackEpisode
	URL           string
	Kind          string
	Quality       int
	SubtitleURL   string
	SubtitleLabel string
	// Headers are the per-file HTTP headers mpv must send. A Filmlib stream
	// only opens with the theatre page as Referer and the session's
	// authorizations header.
	Headers []string
	// Subtitles are the caption tracks the source offers. Every track is added
	// to mpv so the player's own track menu can switch between them.
	Subtitles []filmlib.Subtitle

	chapters        []metadata.Chapter
	chaptersReady   bool
	chaptersLoading bool
	chaptersNoData  bool
}

// Session is one running mpv session with a lazily expanding playlist.
type Session struct {
	service *Service
	plan    *library.PlaybackPlan
	options StartOptions

	ctx    context.Context
	cancel context.CancelFunc

	client *animelib.Client
	mpv    *mpvProcess

	// details caches episode payloads fetched by the parallel batch prefetch so
	// resolveEntry does not request the same episode twice. detailFlight
	// deduplicates concurrent fetches of the same episode.
	detailMu     sync.Mutex
	details      map[int]*animelib.EpisodeDetail
	detailFlight map[int]chan struct{}

	mu             sync.Mutex
	entries        []*playlistEntry
	resolved       map[int]*playlistEntry
	current        int
	currentEp      int
	currentNumber  string
	lastPos        float64
	duration       float64
	paused         bool
	resumePending  float64
	resumeApplied  bool
	lastWrite      time.Time
	waitingForNext bool
	futureDone     bool
	suppressFinish bool
	finished       bool
	loadedOnce     bool
	// autoSkip holds the introdb segment types that are jumped over. The event
	// loop marks each chapter once so a manual rewind into an intro does not
	// trap the viewer in a seek loop.
	autoSkip        map[metadata.Segment]bool
	skippedChapters map[int]bool

	loadMu   sync.Mutex
	ensureMu sync.Mutex
	// preloadOnce starts the behind-the-selection preload on the first
	// file-loaded event. Filling the playlist before that is unnecessary while
	// the first stream is still opening; with a warm Filmlib cache the whole
	// window resolves in milliseconds and can compete with the initial load.
	preloadOnce sync.Once

	baseMu     sync.Mutex
	videoBases []string
	baseErr    error

	quality int

	// openRetried marks the single first-open retry: when mpv fails to open
	// the very first file, the session re-resolves it with a fresh signed URL
	// and loads the player again before giving up. freshResolve makes the next
	// Filmlib resolve skip the manifest cache.
	openRetried  bool
	freshResolve bool
	// failedEntryID is the mpv playlist entry id of the first file that could
	// not be opened. After a retry replaces it, mpv may still emit an end-file
	// error for the old entry; that event must not end the new attempt.
	failedEntryID int64

	chapterSem     chan struct{}
	chaptersMu     sync.Mutex
	chaptersNoData map[string]bool

	finishOnce sync.Once
}

// Stop ends the session without invoking the finish callback. It is used when
// the application itself is closing.
func (s *Session) Stop() {
	if s == nil {
		return
	}
	s.stop()
}

func (s *Session) stop() {
	s.mu.Lock()
	s.suppressFinish = true
	mpv := s.mpv
	s.mu.Unlock()
	s.onFinishProgress()
	s.cancel()
	if mpv != nil {
		mpv.close()
	}
	s.finishOnce.Do(func() {
		s.finished = true
		s.service.clearSession(s)
	})
}

func (s *Session) eventLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.mpv.done:
			// The process can disappear before the buffered exit event is
			// consumed. done is closed after the event is queued and is the
			// reliable signal that the application must become visible again.
			s.finish(s.mpv.exitError())
			return
		case event := <-s.mpv.out:
			switch event.Kind {
			case mpvEventFileLoaded:
				s.onFileLoaded(event.Position)
			case mpvEventPlaylistPos:
				s.onPlaylistPos(event.Position)
			case mpvEventTimePos:
				s.onTimePos(event.Seconds)
			case mpvEventDuration:
				s.onDuration(event.Seconds)
			case mpvEventPause:
				s.mu.Lock()
				s.paused = event.Paused
				s.mu.Unlock()
				if event.Paused {
					s.saveProgress(true, 0)
				}
			case mpvEventEndFile:
				s.onEndFile(event)
			case mpvEventExit:
				s.finish(event.Err)
				return
			}
		}
	}
}

func (s *Session) onFileLoaded(position int) {
	// Any file-loaded event proves mpv opened a stream. Set this before the
	// entry lookup: a stale playlist position must not make a working session
	// look unopened and trigger the first-open retry.
	s.mu.Lock()
	s.loadedOnce = true
	s.mu.Unlock()
	entry := s.entryAt(position)
	if entry == nil {
		return
	}
	s.mu.Lock()
	s.current = position
	s.currentEp = entry.EpisodeIndex
	s.currentNumber = entry.Episode.Number
	s.duration = 0
	resume := 0.0
	if !s.resumeApplied && entry.EpisodeIndex == s.plan.EpisodeIndex {
		resume = s.resumePending
		s.resumeApplied = true
	}
	s.lastPos = resume
	// Chapter indices belong to the previous file. A fresh map keeps the skip
	// state from leaking into the next episode.
	s.skippedChapters = make(map[int]bool)
	s.mu.Unlock()

	if resume > 0 {
		_ = s.mpv.seek(resume)
	}
	s.saveProgress(true, resume)
	// Chapters belong to the file that was just unloaded. Clear them before
	// the new episode's cached/fetched chapters are applied.
	_ = s.mpv.setChapters(nil)
	s.applyChapters(entry)
	s.applySubtitles(entry)
	// Resolving previous episodes can take seconds (Kodik) and must never block
	// the event loop: time-pos/duration events drive progress saving.
	go s.ensurePrevious(entry.EpisodeIndex)
	// Keep the forward window sliding: resolving runs in the background and
	// already-resolved episodes are skipped cheaply. The first file-loaded also
	// starts the behind-the-selection preload; inserting entries before the
	// first file opens is unnecessary and can compete with the initial stream.
	s.preloadOnce.Do(func() { go s.backgroundLoader() })
	go s.loadForward(entry.EpisodeIndex)
}

func (s *Session) onPlaylistPos(position int) {
	s.mu.Lock()
	s.current = position
	entry := s.entryAtLocked(position)
	index := -1
	if entry != nil {
		index = entry.EpisodeIndex
	}
	s.mu.Unlock()
	if index >= 0 {
		go s.ensurePrevious(index)
	}
}

func (s *Session) onTimePos(seconds float64) {
	s.mu.Lock()
	s.lastPos = seconds
	entry := s.entryAtLocked(s.current)
	paused := s.paused
	autoSkip := s.autoSkip
	s.mu.Unlock()

	if !paused && len(autoSkip) > 0 && entry != nil {
		s.applyAutoSkip(entry, seconds, autoSkip)
	}
	s.saveProgress(false, seconds)
}

// applyAutoSkip jumps over an enabled introdb chapter. The event loop is the
// only caller, so the skipped-chapter map needs no separate lock beyond s.mu.
func (s *Session) applyAutoSkip(entry *playlistEntry, position float64, enabled map[metadata.Segment]bool) {
	s.chaptersMu.Lock()
	chapters := append([]metadata.Chapter(nil), entry.chapters...)
	s.chaptersMu.Unlock()
	if len(chapters) < 2 {
		return
	}

	s.mu.Lock()
	target, index, ok := autoSkipTarget(chapters, position, enabled, s.skippedChapters)
	if ok {
		s.skippedChapters[index] = true
	}
	mpv := s.mpv
	s.mu.Unlock()
	if ok && mpv != nil {
		_ = mpv.seek(target)
	}
}

func (s *Session) onDuration(seconds float64) {
	if seconds <= 0 {
		return
	}
	s.mu.Lock()
	s.duration = seconds
	position := s.lastPos
	number := s.currentNumber
	s.mu.Unlock()
	if s.plan.TitleID != 0 && number != "" {
		_ = s.service.lib.SaveProgress(s.plan.TitleID, number, position, seconds)
	}
}

func (s *Session) onEndFile(event mpvEvent) {
	if event.Reason == "error" {
		s.failUnopened(event)
		return
	}
	if event.Reason != "eof" && event.Reason != "quit" {
		return
	}
	s.saveProgress(true, 0)

	s.mu.Lock()
	entry := s.entryAtLocked(event.Position)
	if entry == nil {
		entry = s.entryAtLocked(s.current)
	}
	if entry == nil {
		s.mu.Unlock()
		return
	}
	isLastPlanEpisode := entry.EpisodeIndex == len(s.plan.Episodes)-1
	hasNextInPlaylist := false
	for index, candidate := range s.entries {
		if candidate.EpisodeIndex > entry.EpisodeIndex {
			hasNextInPlaylist = true
			_ = index
			break
		}
	}
	if !hasNextInPlaylist && !isLastPlanEpisode {
		s.waitingForNext = true
	}
	futureDone := s.futureDone
	mpv := s.mpv
	s.mu.Unlock()

	if event.Reason == "eof" && isLastPlanEpisode && !hasNextInPlaylist {
		if s.plan.CompleteOnFinish && s.plan.TitleID != 0 && s.service != nil && s.service.lib != nil {
			_ = s.service.lib.MarkCompleted(s.plan.TitleID)
		}
		if mpv != nil {
			_, _ = mpv.call("quit")
		}
		return
	}
	if event.Reason == "eof" && !hasNextInPlaylist && futureDone {
		if mpv != nil {
			_, _ = mpv.call("quit")
		}
	}
}

// failUnopened returns to the application when mpv could not open the first
// file. A failed open arrives as end-file with reason "error" and no preceding
// file-loaded event; without this check mpv would stay on its idle "drop files"
// screen while the application window remains hidden.
//
// The first failure is retried with a fresh resolve: a signed CDN URL can go
// stale between the resolve and mpv opening it (Filmlib especially), so a
// second attempt often brings the same episode up instead of ending playback.
func (s *Session) failUnopened(event mpvEvent) {
	s.mu.Lock()
	loaded := s.loadedOnce
	// Replacing a failed first file can make mpv report one more error for
	// the entry that has just been discarded. Its playlist entry id is still
	// the old one; the fresh entry gets a new id and keeps its own error.
	if event.EntryID != 0 && s.openRetried && event.EntryID == s.failedEntryID {
		s.mu.Unlock()
		logger.Debug("ignoring stale end-file after retry", "entry_id", event.EntryID)
		return
	}
	if !s.openRetried {
		s.failedEntryID = event.EntryID
	}
	entry := s.unopenedEntryLocked(event.Position)
	mpv := s.mpv
	s.mu.Unlock()
	if loaded {
		return
	}

	if s.retryOpenOnce() {
		// The fresh entry reloaded fine; the resumed open will either play or
		// end with its own end-file event.
		return
	}
	// The retry can leave a stale playlist position behind (mpv reports the
	// previous entry while the replacement is loading), so read the entry
	// again instead of reporting the one captured before the retry.
	s.mu.Lock()
	entry = s.unopenedEntryLocked(s.current)
	s.mu.Unlock()
	s.finish(unopenedError(entry, s.planSource()))
	if mpv != nil {
		mpv.close()
	}
}

// unopenedEntryLocked finds the episode that failed to open. The first file is
// always entries[0]; falling back to it keeps a stale playlist position from
// turning a clear player error into a generic playback failure.
func (s *Session) unopenedEntryLocked(position int) *playlistEntry {
	entry := s.entryAtLocked(position)
	if entry == nil {
		entry = s.entryAtLocked(s.current)
	}
	if entry == nil && len(s.entries) > 0 {
		entry = s.entries[0]
	}
	return entry
}

// retryOpenOnce resolves the first playlist entry again (skipping the Filmlib
// manifest cache) and loads it into mpv. It returns true when the load command
// was accepted; it reports false and lets failUnopened finish the session when
// the resolve also fails, or when the session changed underneath.
func (s *Session) retryOpenOnce() bool {
	s.mu.Lock()
	if s.openRetried || s.mpv == nil || len(s.entries) == 0 || s.loadedOnce {
		s.mu.Unlock()
		return false
	}
	s.openRetried = true
	s.freshResolve = true
	mpv := s.mpv
	index := s.entries[0].EpisodeIndex
	number := s.entries[0].Episode.Number
	resume := s.resumePending
	s.mu.Unlock()

	entry, err := s.resolveEntry(s.ctx, index)
	if err != nil {
		s.mu.Lock()
		s.freshResolve = false
		s.mu.Unlock()
		logger.Warn("cannot resolve the first video again", "episode", number, "error", err)
		return false
	}
	s.mu.Lock()
	if s.mpv != mpv || s.loadedOnce {
		s.freshResolve = false
		s.mu.Unlock()
		return false
	}
	err = mpv.load(entry.URL, headersFor(entry))
	s.freshResolve = false
	if err != nil {
		s.mu.Unlock()
		logger.Warn("cannot reload the first video", "episode", entry.Episode.Number, "error", err)
		return false
	}
	s.entries = []*playlistEntry{entry}
	s.resolved = map[int]*playlistEntry{index: entry}
	s.resumePending = resume
	s.current = 0
	s.currentEp = index
	s.currentNumber = entry.Episode.Number
	s.mu.Unlock()
	return true
}

func (s *Session) planSource() string {
	if s == nil || s.plan == nil {
		return ""
	}
	return s.plan.Source
}

func unopenedError(entry *playlistEntry, source string) error {
	if entry != nil {
		switch {
		case entry.Kind == "hls" && source == filmlib.Source:
			return ErrVideoOpen
		case entry.Kind == "hls":
			return ErrKodikUnavailable
		default:
			return ErrPlaybackFailed
		}
	}
	return ErrPlaybackFailed
}

func (s *Session) onFinishProgress() {
	s.saveProgress(true, 0)
}

func (s *Session) saveProgress(force bool, position float64) {
	s.mu.Lock()
	number := s.currentNumber
	if position <= 0 {
		position = s.lastPos
	}
	duration := s.duration
	lastWrite := s.lastWrite
	s.mu.Unlock()
	if s.plan.TitleID == 0 || number == "" {
		return
	}
	if !force && position < 5 {
		return
	}
	if !force && time.Since(lastWrite) < 5*time.Second {
		return
	}
	if err := s.service.lib.SaveProgress(s.plan.TitleID, number, position, duration); err == nil {
		s.mu.Lock()
		s.lastWrite = time.Now()
		s.mu.Unlock()
	}
}

func (s *Session) finish(err error) {
	s.finishOnce.Do(func() {
		if err != nil {
			logger.Warn("playback finished with an error", "error", err)
		} else {
			logger.Info("playback finished")
		}
		s.onFinishProgress()
		s.mu.Lock()
		s.finished = true
		suppress := s.suppressFinish
		s.mu.Unlock()
		s.cancel()
		s.service.clearSession(s)
		if !suppress && s.options.OnFinish != nil {
			go s.options.OnFinish(err)
		}
	})
}

func (s *Session) entryAt(position int) *playlistEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entryAtLocked(position)
}

func (s *Session) entryAtLocked(position int) *playlistEntry {
	if position < 0 || position >= len(s.entries) {
		return nil
	}
	return s.entries[position]
}

const (
	// preloadAhead bounds how many upcoming episodes are resolved in advance.
	// The window slides with playback: loadForward runs again whenever mpv
	// starts the next file, so a long title is never resolved all at once.
	preloadAhead = 30
	// preloadBehind is how many previous episodes stay available for seeking
	// back without a new request.
	preloadBehind = 5
	// Filmlib episodes are resolved through a headless theatre page, one at
	// a time, so the window is much smaller than the AnimeLIB one: a handful
	// ahead is enough to hide the resolve latency without keeping Chromium busy
	// for minutes.
	filmlibPreloadAhead  = 8
	filmlibPreloadBehind = 2
)

// ahead returns the forward preload window of the current plan.
func (s *Session) ahead() int {
	if s.plan != nil && s.plan.Source == filmlib.Source {
		return filmlibPreloadAhead
	}
	return preloadAhead
}

// behind returns the backward preload window of the current plan.
func (s *Session) behind() int {
	if s.plan != nil && s.plan.Source == filmlib.Source {
		return filmlibPreloadBehind
	}
	return preloadBehind
}

// backgroundLoader fills the episodes before the selected one. The forward
// window is started by onFileLoaded: resolving it earlier would put entries into
// mpv before the first file has opened, and with a warm Filmlib cache the
// whole window can resolve while the initial stream is still starting.
func (s *Session) backgroundLoader() {
	start := s.plan.EpisodeIndex
	behind := s.behind()
	for index := start - 1; index >= 0 && index >= start-behind; index-- {
		if s.ctx.Err() != nil {
			return
		}
		s.resolveAndAdd(index)
	}
	s.settleWaiting()
}

// loadForward resolves the next preloadAhead episodes after currentEp. When the
// window reaches the end of the plan futureDone is set, which tells onEndFile
// that no further episode can be appended.
func (s *Session) loadForward(currentEp int) {
	last := len(s.plan.Episodes) - 1
	if last < 0 {
		return
	}
	if currentEp < 0 {
		currentEp = 0
	}
	if currentEp > last {
		currentEp = last
	}
	limit := currentEp + s.ahead()
	if limit > last {
		limit = last
	}
	pending := make([]int, 0, limit-currentEp)
	s.mu.Lock()
	for index := currentEp + 1; index <= limit; index++ {
		if _, ok := s.resolved[index]; !ok {
			pending = append(pending, index)
		}
	}
	s.mu.Unlock()

	// AnimeLIB has no batch detail endpoint: the episode list never returns
	// players and the site itself loads one episode at a time. Fetch the whole
	// window in a bounded parallel batch instead, then resolve the entries from
	// the cached payloads without further requests.
	s.prefetchDetails(pending)
	for _, index := range pending {
		if s.ctx.Err() != nil {
			return
		}
		s.resolveAndAdd(index)
	}
	if limit < last {
		return
	}
	s.mu.Lock()
	s.futureDone = true
	s.mu.Unlock()
}

// prefetchDetails loads episode payloads for the given indices concurrently and
// stores them in the session cache. Failures are ignored here; resolveAndAdd
// retries them sequentially so one bad episode does not drop the window.
func (s *Session) prefetchDetails(indices []int) {
	const workers = 4
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, index := range indices {
		if s.ctx.Err() != nil {
			break
		}
		if index < 0 || index >= len(s.plan.Episodes) {
			continue
		}
		id := s.plan.Episodes[index].ID
		if id <= 0 {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-s.ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(s.ctx, 90*time.Second)
			defer cancel()
			_, _ = s.episodeDetail(ctx, id)
		}(id)
	}
	wg.Wait()
}

// episodeDetail returns the cached payload for an episode or fetches it once.
// Concurrent callers for the same id wait for the first fetch instead of
// issuing duplicate requests.
func (s *Session) episodeDetail(ctx context.Context, id int) (*animelib.EpisodeDetail, error) {
	for {
		s.detailMu.Lock()
		if cached := s.details[id]; cached != nil {
			s.detailMu.Unlock()
			return cached, nil
		}
		if flight := s.detailFlight[id]; flight != nil {
			s.detailMu.Unlock()
			select {
			case <-flight:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		flight := make(chan struct{})
		s.detailFlight[id] = flight
		s.detailMu.Unlock()

		detail, err := s.client.EpisodeDetail(ctx, id)

		s.detailMu.Lock()
		delete(s.detailFlight, id)
		if err == nil {
			s.details[id] = detail
		}
		close(flight)
		s.detailMu.Unlock()
		return detail, err
	}
}

// settleWaiting resumes playback when the file that just ended has a resolved
// successor. If the plan is exhausted and nothing follows, mpv is closed
// instead of being left open on a finished playlist.
func (s *Session) settleWaiting() {
	s.mu.Lock()
	waiting := s.waitingForNext
	futureDone := s.futureDone
	current := s.currentEp
	var lastEntry *playlistEntry
	if len(s.entries) > 0 {
		lastEntry = s.entries[len(s.entries)-1]
	}
	mpv := s.mpv
	s.mu.Unlock()
	if !waiting {
		return
	}
	if lastEntry != nil && lastEntry.EpisodeIndex > current {
		// The current file ended before the next item was appended. Start the
		// now-available next item explicitly; mpv does not resume an ended
		// playlist by itself.
		s.startWaitingNext()
		return
	}
	if futureDone && mpv != nil {
		// Every later episode failed to resolve. Do not leave mpv open at the
		// end of the playlist; finish the session and return to the app.
		_, _ = mpv.call("quit")
	}
}

func (s *Session) startWaitingNext() {
	s.mu.Lock()
	s.waitingForNext = false
	s.mu.Unlock()
	_, _ = s.mpv.call("set_property", "pause", false)
}

// ensurePrevious keeps at least five resolved episodes before the current one.
func (s *Session) ensurePrevious(currentEp int) {
	if currentEp <= 0 || s.ctx.Err() != nil {
		return
	}
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()
	s.mu.Lock()
	before := 0
	for _, entry := range s.entries {
		if entry.EpisodeIndex < currentEp {
			before++
		}
	}
	s.mu.Unlock()
	if before >= 5 {
		return
	}
	target := currentEp - 5
	if target < 0 {
		target = 0
	}
	for index := currentEp - 1; index >= target; index-- {
		if s.ctx.Err() != nil {
			return
		}
		s.resolveAndAdd(index)
	}
}

// resolveAndAdd resolves one episode and inserts it into the mpv playlist in
// episode order. Previous episodes are added on demand, so an insertion may be
// in the middle of the playlist rather than only at either end.
func (s *Session) resolveAndAdd(index int) {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	if _, ok := s.resolved[index]; ok {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	entry, err := s.resolveEntry(s.ctx, index)
	if err != nil {
		return
	}

	s.mu.Lock()
	if _, ok := s.resolved[index]; ok {
		s.mu.Unlock()
		return
	}
	insertAt := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].EpisodeIndex >= entry.EpisodeIndex
	})
	if insertAt < len(s.entries) && s.entries[insertAt].EpisodeIndex == entry.EpisodeIndex {
		s.mu.Unlock()
		return
	}
	current := s.current
	s.resolved[index] = entry
	s.entries = append(s.entries, nil)
	copy(s.entries[insertAt+1:], s.entries[insertAt:])
	s.entries[insertAt] = entry
	if insertAt <= s.current {
		s.current++
	}
	waiting := s.waitingForNext
	s.mu.Unlock()

	if err := s.mpv.insert(insertAt, entry.URL, headersFor(entry)); err != nil {
		// The entry never reached mpv. Roll back the in-memory playlist so
		// progress attribution and later insert positions stay consistent.
		s.mu.Lock()
		if insertAt < len(s.entries) && s.entries[insertAt] == entry {
			copy(s.entries[insertAt:], s.entries[insertAt+1:])
			s.entries[len(s.entries)-1] = nil
			s.entries = s.entries[:len(s.entries)-1]
		}
		delete(s.resolved, index)
		if insertAt <= current && s.current > current {
			s.current--
		}
		s.mu.Unlock()
		return
	}
	if waiting && insertAt > current {
		_, _ = s.mpv.call("set_property", "playlist-pos", insertAt)
		s.mu.Lock()
		s.waitingForNext = false
		s.mu.Unlock()
	}
	go s.prefetchChapters(entry)
}

func (s *Session) resolveEntry(ctx context.Context, index int) (*playlistEntry, error) {
	if index < 0 || index >= len(s.plan.Episodes) {
		return nil, ErrInvalidEpisode
	}
	if s.plan.Source == filmlib.Source {
		return s.resolveFilmlib(ctx, index)
	}
	episode := s.plan.Episodes[index]
	if episode.ID <= 0 {
		return nil, fmt.Errorf("episode %s has no id", episode.Number)
	}
	resolveCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	detail, err := s.episodeDetail(resolveCtx, episode.ID)
	if err != nil {
		return nil, fmt.Errorf("episode %s: %w", episode.Number, err)
	}

	animelibPlayer, kodikPlayer := selectVoicePlayers(detail.Players, s.plan.Voice)

	// AnimeLIB always wins when the selected voice has a direct file for this
	// episode. Kodik is the fallback only for episodes without one.
	if animelibPlayer != nil {
		link := pickAnimelibQuality(animelibPlayer.Qualities, s.quality)
		if link.Href != "" {
			mediaURL, joinErr := s.joinAnimelib(resolveCtx, link.Href, animelibPlayer.VideoDomain)
			if joinErr == nil && mediaURL != "" {
				// Subtitles belong to the player that is actually playing.
				// Never borrow tracks from another voice or from a Kodik
				// fallback, where the timings may differ.
				subtitleURL, subtitleLabel := s.subtitleForPlayer(resolveCtx, animelibPlayer)
				return &playlistEntry{
					EpisodeIndex:  index,
					Episode:       episode,
					URL:           mediaURL,
					Kind:          "mp4",
					Quality:       link.Quality,
					SubtitleURL:   subtitleURL,
					SubtitleLabel: subtitleLabel,
				}, nil
			}
		}
	}
	if kodikPlayer != nil {
		links, resolveErr := s.service.resolver.Resolve(resolveCtx, kodikPlayer.Src)
		if resolveErr == nil {
			mediaURL, quality := kodik.PickQuality(links, s.quality)
			if mediaURL != "" {
				return &playlistEntry{
					EpisodeIndex: index,
					Episode:      episode,
					URL:          mediaURL,
					Kind:         "hls",
					Quality:      quality,
				}, nil
			}
		} else if resolveCtx.Err() == nil {
			return nil, fmt.Errorf("voice %q for episode %s: %w", s.plan.Voice, episode.Number, resolveErr)
		}
	}
	return nil, fmt.Errorf("voice %q is unavailable for episode %s", s.plan.Voice, episode.Number)
}

func (s *Session) joinAnimelib(ctx context.Context, href, domain string) (string, error) {
	if isAbsoluteURL(href) {
		return href, nil
	}
	bases, err := s.animelibBases(ctx)
	if err != nil {
		return "", err
	}
	if joined := animelib.JoinVideoBase(pickVideoBase(bases, domain), href); joined != "" {
		return joined, nil
	}
	return "", errors.New("AnimeLIB returned no video servers")
}

func (s *Session) animelibBases(ctx context.Context) ([]string, error) {
	s.baseMu.Lock()
	defer s.baseMu.Unlock()
	if len(s.videoBases) > 0 {
		return append([]string(nil), s.videoBases...), nil
	}
	if s.baseErr != nil && len(s.videoBases) == 0 {
		// Retry once per session after a transient constants failure.
	}
	bases, err := s.client.VideoBases(ctx)
	if err != nil {
		s.baseErr = err
		return nil, err
	}
	s.videoBases = bases
	s.baseErr = nil
	return append([]string(nil), bases...), nil
}

func (s *Session) prefetchChapters(entry *playlistEntry) {
	select {
	case s.chapterSem <- struct{}{}:
		defer func() { <-s.chapterSem }()
	case <-s.ctx.Done():
		return
	}
	s.loadChapters(entry)
}

func (s *Session) applyChapters(entry *playlistEntry) {
	if entry == nil {
		return
	}
	go s.prefetchChapters(entry)
}

func (s *Session) applySubtitles(entry *playlistEntry) {
	if entry == nil {
		return
	}
	if len(entry.Subtitles) > 0 {
		// Every caption variant is added so the player's own track menu can
		// switch between them; the variant the source marks as default (or the
		// first one) becomes active.
		active := 0
		for index, subtitle := range entry.Subtitles {
			if subtitle.Default {
				active = index
				break
			}
		}
		for index, subtitle := range entry.Subtitles {
			flags := "auto"
			if index == active {
				flags = "select"
			}
			// Missing or broken subtitles must never stop playback.
			_ = s.mpv.addSubtitleFlags(subtitle.URL, subtitle.Label, flags)
		}
		return
	}
	if strings.TrimSpace(entry.SubtitleURL) == "" {
		return
	}
	_ = s.mpv.addSubtitle(entry.SubtitleURL, entry.SubtitleLabel)
}

func (s *Session) loadChapters(entry *playlistEntry) {
	if entry == nil {
		return
	}
	s.chaptersMu.Lock()
	if entry.chaptersReady {
		chapters := append([]metadata.Chapter(nil), entry.chapters...)
		s.chaptersMu.Unlock()
		s.setChaptersIfCurrent(entry, chapters)
		return
	}
	if entry.chaptersLoading {
		// A prefetch is already fetching this entry. It will apply the result
		// when the file becomes current.
		s.chaptersMu.Unlock()
		return
	}
	entry.chaptersLoading = true
	s.chaptersMu.Unlock()
	defer func() {
		s.chaptersMu.Lock()
		entry.chaptersLoading = false
		s.chaptersMu.Unlock()
	}()

	if s.plan.IMDbID == "" || s.plan.Season <= 0 {
		s.chaptersMu.Lock()
		entry.chaptersReady = true
		entry.chaptersNoData = true
		s.chaptersMu.Unlock()
		return
	}
	if s.plan.TitleID != 0 {
		if raw, noData, ok := s.service.lib.IntrodbCache(s.plan.TitleID, entry.Episode.Number); ok {
			if noData {
				s.chaptersMu.Lock()
				entry.chaptersReady = true
				entry.chaptersNoData = true
				entry.chapters = nil
				s.chaptersMu.Unlock()
				return
			}
			var cached []metadata.Chapter
			if json.Unmarshal(raw, &cached) == nil {
				s.chaptersMu.Lock()
				entry.chapters = cached
				entry.chaptersReady = true
				s.chaptersMu.Unlock()
				s.setChaptersIfCurrent(entry, cached)
				return
			}
		}
	}
	chapters, noData, err := metadata.Chapters(s.ctx, s.plan.IMDbID, s.plan.Season, entry.Episode.Number)
	if err != nil {
		// Treat a failed lookup as no data for this session, but do not cache
		// the failure permanently.
		s.chaptersMu.Lock()
		entry.chaptersReady = true
		entry.chaptersNoData = true
		s.chaptersMu.Unlock()
		return
	}
	s.chaptersMu.Lock()
	entry.chapters = chapters
	entry.chaptersReady = true
	entry.chaptersNoData = noData
	s.chaptersMu.Unlock()
	if s.plan.TitleID != 0 {
		if noData {
			_ = s.service.lib.SaveIntrodbCache(s.plan.TitleID, entry.Episode.Number, nil, true)
		} else if raw, marshalErr := json.Marshal(chapters); marshalErr == nil {
			_ = s.service.lib.SaveIntrodbCache(s.plan.TitleID, entry.Episode.Number, raw, false)
		}
	}
	s.setChaptersIfCurrent(entry, chapters)
}

func (s *Session) setChaptersIfCurrent(entry *playlistEntry, chapters []metadata.Chapter) {
	s.mu.Lock()
	current := s.currentEp == entry.EpisodeIndex
	s.mu.Unlock()
	if current {
		_ = s.mpv.setChapters(chapters)
	}
}

func pickAnimelibQuality(links []animelib.QualityLink, preferred int) animelib.QualityLink {
	if len(links) == 0 {
		return animelib.QualityLink{}
	}
	if preferred <= 0 {
		preferred = 1080
	}
	sorted := append([]animelib.QualityLink(nil), links...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Quality < sorted[j].Quality })
	for _, link := range sorted {
		if link.Quality == preferred {
			return link
		}
	}
	lower := animelib.QualityLink{}
	for _, link := range sorted {
		if link.Quality < preferred {
			lower = link
		}
	}
	if lower.Quality > 0 {
		return lower
	}
	return sorted[0]
}

func selectVoicePlayers(players []animelib.EpisodePlayer, voice string) (*animelib.EpisodePlayer, *animelib.EpisodePlayer) {
	var animelibPlayer *animelib.EpisodePlayer
	var kodikPlayer *animelib.EpisodePlayer
	for playerIndex := range players {
		player := &players[playerIndex]
		if !sameTeam(player.TeamName, voice) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(player.Player)) {
		case "animelib":
			if animelibPlayer == nil && len(player.Qualities) > 0 {
				animelibPlayer = player
			}
		case "kodik":
			if kodikPlayer == nil && player.Src != "" {
				kodikPlayer = player
			}
		}
	}
	return animelibPlayer, kodikPlayer
}

// pickSubtitleTrack enforces the playback preference: ASS first, VTT only when
// the selected player has no ASS track.
func pickSubtitleTrack(tracks []animelib.SubtitleTrack) (animelib.SubtitleTrack, bool) {
	var fallback animelib.SubtitleTrack
	hasFallback := false
	for _, track := range tracks {
		if strings.TrimSpace(track.Src) == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(track.Format)) {
		case "ass":
			return track, true
		case "vtt":
			if !hasFallback {
				fallback = track
				hasFallback = true
			}
		}
	}
	return fallback, hasFallback
}

func (s *Session) subtitleForPlayer(ctx context.Context, player *animelib.EpisodePlayer) (string, string) {
	if player == nil {
		return "", ""
	}
	track, ok := pickSubtitleTrack(player.Subtitles)
	if !ok {
		return "", ""
	}
	label := strings.ToUpper(strings.TrimSpace(track.Format))
	src := strings.TrimSpace(track.Src)
	if isAbsoluteURL(src) {
		return src, label
	}
	bases, err := s.animelibBases(ctx)
	if err != nil {
		return "", ""
	}
	return resolveRelativeSubtitle(src, player.VideoDomain, bases), label
}

// videoBaseHost maps the API's video_domain to the CDN host that actually
// stores the file. The constants endpoint lists several servers and a relative
// href belongs to exactly one of them: joining it to the first server yields a
// 404 for titles hosted on a secondary one.
func videoBaseHost(domain string) string {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "secondary_1":
		return "video2.cdnlibs.org"
	case "secondary_2":
		return "video1.imglib.info"
	default:
		return "video1.cdnlibs.org"
	}
}

// pickVideoBase returns the server matching the player's domain and falls back
// to the first known server when the list does not contain it.
func pickVideoBase(bases []string, domain string) string {
	wantedHost := videoBaseHost(domain)
	for _, base := range bases {
		parsed, err := url.Parse(strings.TrimSpace(base))
		if err == nil && strings.EqualFold(parsed.Hostname(), wantedHost) {
			return base
		}
	}
	if len(bases) > 0 {
		return bases[0]
	}
	return ""
}

// resolveRelativeSubtitle joins a relative subtitle path to the video server
// named by the player's video_domain. The API currently returns absolute
// subtitle URLs, but relative paths exist in older clients' data.
func resolveRelativeSubtitle(src, domain string, bases []string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	if isAbsoluteURL(src) {
		return src
	}
	return animelib.JoinVideoBase(pickVideoBase(bases, domain), src)
}

func sameTeam(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return strings.EqualFold(left, right)
}

func isAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.IsAbs()
}

var logger = log.For(log.SourcePlayback)
