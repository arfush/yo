package playback

import (
	"context"
	"sync"
	"time"

	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/library"
	"github.com/arfush/yo/libsocial"
	"github.com/arfush/yo/metadata"
)

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

	client *libsocial.Client
	mpv    *mpvProcess

	// details caches episode payloads fetched by the parallel batch prefetch so
	// resolveEntry does not request the same episode twice. detailFlight
	// deduplicates concurrent fetches of the same episode.
	detailMu     sync.Mutex
	details      map[int]*libsocial.EpisodeDetail
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
