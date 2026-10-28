package playback

import (
	"encoding/json"
	"strings"

	"github.com/arfush/yo/metadata"
)

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
