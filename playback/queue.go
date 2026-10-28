package playback

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/kodik"
	"github.com/arfush/yo/libsocial"
)

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
func (s *Session) episodeDetail(ctx context.Context, id int) (*libsocial.EpisodeDetail, error) {
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

	libsocialPlayer, kodikPlayer := selectVoicePlayers(detail.Players, s.plan.Voice)

	// LibSocial always wins when the selected voice has a direct file for this
	// episode. Kodik is the fallback only for episodes without one.
	if libsocialPlayer != nil {
		link := pickLibSocialQuality(libsocialPlayer.Qualities, s.quality)
		if link.Href != "" {
			mediaURL, joinErr := s.joinLibSocial(resolveCtx, link.Href, libsocialPlayer.VideoDomain)
			if joinErr == nil && mediaURL != "" {
				// Subtitles belong to the player that is actually playing.
				// Never borrow tracks from another voice or from a Kodik
				// fallback, where the timings may differ.
				subtitleURL, subtitleLabel := s.subtitleForPlayer(resolveCtx, libsocialPlayer)
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

func (s *Session) joinLibSocial(ctx context.Context, href, domain string) (string, error) {
	if isAbsoluteURL(href) {
		return href, nil
	}
	bases, err := s.libsocialBases(ctx)
	if err != nil {
		return "", err
	}
	if joined := libsocial.JoinVideoBase(pickVideoBase(bases, domain), href); joined != "" {
		return joined, nil
	}
	return "", errors.New("AnimeLIB returned no video servers")
}

func (s *Session) libsocialBases(ctx context.Context) ([]string, error) {
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
