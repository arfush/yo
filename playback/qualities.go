package playback

import (
	"context"
	"errors"

	"github.com/arfush/yo/kinopoisk"
	"github.com/arfush/yo/library"
)

// Qualities reports which stream qualities a Kinopoisk voice actually offers.
// The title page calls it in the background right after a film or series is
// opened: the mirror's ladder differs per title (a BDRip film may stop at 1080,
// a remaster reaches 2160), and the quality buttons must disable what the
// selected voice cannot deliver. No mpv process is started, and the resolver's
// manifest cache makes a repeated probe of the same episode cheap.
//
// The preferred quality is passed through only so the probe and a later
// playback of that quality share one cached manifest.
func (s *Service) Qualities(ctx context.Context, plan *library.PlaybackPlan, preferred int) ([]int, error) {
	if s == nil || plan == nil {
		return nil, errors.New("playback plan is nil")
	}
	if plan.Source != kinopoisk.Source || plan.Kinopoisk == nil {
		return nil, errors.New("voiceover has no player URL")
	}
	if s.kinopoisk == nil {
		return nil, errors.New("Kinopoisk browser is unavailable")
	}
	if s.Active() {
		// The probe must never queue behind a playing session's resolves: mpv
		// is opening and the title page is about to disappear anyway.
		return nil, ErrAlreadyPlaying
	}
	if len(plan.Episodes) == 0 {
		return nil, errors.New("the selected voiceover has no available episodes")
	}
	index := plan.EpisodeIndex
	if index < 0 || index >= len(plan.Episodes) {
		index = 0
	}
	season, episode, ok := kinopoisk.SplitEpisodeKey(plan.Episodes[index].Number)
	if !ok {
		return nil, ErrInvalidEpisode
	}
	if preferred <= 0 {
		preferred = 1080
	}
	logger.Info("probing Kinopoisk qualities", "title", plan.Slug, "voice", plan.Voice, "episode", plan.Episodes[index].Number)
	media, err := s.kinopoisk.Resolve(ctx, kinopoisk.Request{
		Iframe:        plan.Kinopoisk.Iframe,
		Season:        season,
		Episode:       episode,
		TranslationID: plan.Kinopoisk.TranslationID,
		Quality:       preferred,
	})
	if err != nil {
		logger.Warn("cannot probe Kinopoisk qualities", "title", plan.Slug, "voice", plan.Voice, "error", err)
		return nil, err
	}
	logger.Info("Kinopoisk qualities resolved", "title", plan.Slug, "voice", plan.Voice, "qualities", media.Qualities)
	return media.Qualities, nil
}
