package playback

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/metadata"
)

// animelibStreamHeaders keep AnimeLIB direct files and Kodik manifests working.
// They are set explicitly on every file because a Filmlib episode replaces the
// same mpv property with its own headers.
var animelibStreamHeaders = []string{
	"Referer: https://v5.animelib.org/",
	"Origin: https://v5.animelib.org",
}

// headersFor returns the headers of one playlist entry.
func headersFor(entry *playlistEntry) []string {
	if entry != nil && len(entry.Headers) > 0 {
		return entry.Headers
	}
	return animelibStreamHeaders
}

// resolveFilmlib resolves one Alloha episode through the headless theatre.
// The theatre also reports its own skip ranges, so intro/outro chapters are
// attached here and the introdb lookup is skipped for this entry.
func (s *Session) resolveFilmlib(ctx context.Context, index int) (*playlistEntry, error) {
	if s.plan.Filmlib == nil || strings.TrimSpace(s.plan.Filmlib.Iframe) == "" {
		return nil, errors.New("voiceover has no player URL")
	}
	episode := s.plan.Episodes[index]
	season, number, ok := filmlib.SplitEpisodeKey(episode.Number)
	if !ok {
		return nil, fmt.Errorf("invalid episode %q", episode.Number)
	}
	quality := s.quality
	if quality <= 0 {
		quality = 1080
	}
	s.mu.Lock()
	fresh := s.freshResolve
	s.mu.Unlock()
	request := filmlib.Request{
		Iframe:        s.plan.Filmlib.Iframe,
		Season:        season,
		Episode:       number,
		TranslationID: s.plan.Filmlib.TranslationID,
		Quality:       quality,
	}
	var media filmlib.Media
	var err error
	if fresh {
		// The first open failed; a signed URL may have expired, so do not
		// trust the 40-minute manifest cache for this retry.
		media, err = s.service.filmlib.ResolveFresh(ctx, request)
	} else {
		media, err = s.service.filmlib.Resolve(ctx, request)
	}
	if err != nil {
		return nil, fmt.Errorf("episode %s: %w", episode.Number, err)
	}
	entry := &playlistEntry{
		EpisodeIndex:   index,
		Episode:        episode,
		URL:            media.URL,
		Kind:           "hls",
		Quality:        media.Quality,
		Headers:        media.Headers,
		Subtitles:      media.Subtitles,
		chapters:       filmlibChapters(media.Skip),
		chaptersReady:  true,
		chaptersNoData: len(media.Skip) == 0,
	}
	logger.Info("filmlib episode resolved", "episode", episode.Number, "quality", media.Quality, "subtitles", len(media.Subtitles), "skip", len(media.Skip))
	return entry, nil
}

// filmlibChapters converts the theatre's skip ranges into the marker chapters
// the auto-skip loop already understands: a segment chapter at the start of the
// range and a plain chapter at its end.
func filmlibChapters(ranges []filmlib.SkipRange) []metadata.Chapter {
	if len(ranges) == 0 {
		return nil
	}
	chapters := make([]metadata.Chapter, 0, len(ranges)*2)
	for index, span := range ranges {
		if span.End <= span.Start {
			continue
		}
		segment := metadata.SegmentIntro
		if index == len(ranges)-1 && len(ranges) > 1 {
			segment = metadata.SegmentOutro
		}
		if len(chapters) > 0 && chapters[len(chapters)-1].Time >= span.Start {
			// The previous range ends where this one starts; its closing
			// chapter becomes this range's opening chapter.
			chapters[len(chapters)-1] = metadata.Chapter{Title: metadata.ChapterTitle(segment), Time: span.Start}
		} else {
			chapters = append(chapters, metadata.Chapter{Title: metadata.ChapterTitle(segment), Time: span.Start})
		}
		chapters = append(chapters, metadata.Chapter{Title: "Episode", Time: span.End})
	}
	return chapters
}
