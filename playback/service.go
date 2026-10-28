package playback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arfush/yo/browser"
	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/kodik"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/libsocial"
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
	var client *libsocial.Client
	if plan.Source != filmlib.Source {
		client = libsocial.New(token)
	}
	session := &Session{
		service:         s,
		plan:            plan,
		options:         options,
		client:          client,
		ctx:             sessionCtx,
		cancel:          cancel,
		resolved:        make(map[int]*playlistEntry),
		details:         make(map[int]*libsocial.EpisodeDetail),
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

var logger = log.For(log.SourcePlayback)
