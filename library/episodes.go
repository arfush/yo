package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/filmlib"
	"github.com/arfush/yo/libsocial"
	"github.com/arfush/yo/storage"
)

// Notification types are shared with the UI through catalog.
type (
	EpisodeVoiceover    = catalog.EpisodeVoiceover
	EpisodeNotification = catalog.EpisodeNotification
)

// episodeFetcher is replaced by tests so the checker can run without a network.
// Production leaves it nil and uses the paced background AnimeLIB client.
type episodeFetcher func(context.Context, string) (*libsocial.EpisodeRelease, error)

// episodeCountFetcher is the test seam for the episode-total backfill.
type episodeCountFetcher func(context.Context, string) (int, error)

// EpisodeNotificationsEnabled reports whether the user wants new-episode
// notifications.
func (l *Library) EpisodeNotificationsEnabled() bool {
	if l == nil {
		return false
	}
	return l.storage.EpisodeNotificationsEnabled()
}

// SetEpisodeNotificationsEnabled stores the notification choice.
func (l *Library) SetEpisodeNotificationsEnabled(enabled bool) error {
	if l == nil {
		return ErrDatabaseUnavailable
	}
	return l.storage.SetEpisodeNotificationsEnabled(enabled)
}

// CheckEpisodeNotifications polls every saved ongoing or announced title and
// returns notifications through notify. The tracking table is updated only
// after a notification has been delivered, so a failed delivery is retried on
// the next pass instead of being silently lost.
func (l *Library) CheckEpisodeNotifications(ctx context.Context, notify func(EpisodeNotification) error) error {
	if l == nil || !l.storage.EpisodeNotificationsEnabled() {
		return nil
	}
	watches, err := l.storage.ListEpisodeWatches()
	if err != nil {
		return err
	}
	if len(watches) == 0 {
		return nil
	}

	fetch := l.episodeFetcher
	if fetch == nil {
		token, err := l.readToken()
		if err != nil {
			return err
		}
		client := libsocial.NewBackground(token)
		fetch = client.LatestEpisode
	}

	var failures []error
	for _, watch := range watches {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		status := Status(watch.Status)
		if status == StatusCompleted ||
			(status != StatusOngoing && status != StatusAnnounced) {
			if err := l.storage.DeleteEpisodeTracking(watch.TitleID); err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", watch.Slug, err))
			}
			continue
		}

		release, err := fetch(ctx, watch.Slug)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", watch.Slug, err))
			continue
		}
		if err := l.processEpisodeRelease(watch, release, notify); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", watch.Slug, err))
		}
	}
	return errors.Join(failures...)
}

func (l *Library) processEpisodeRelease(watch storage.EpisodeWatch, release *libsocial.EpisodeRelease, notify func(EpisodeNotification) error) error {
	if release == nil {
		return nil
	}
	// The episode list length is the total shown by the history progress bar.
	// It grows when an ongoing title gets a new episode.
	if release.EpisodeCount > 0 {
		if err := l.storage.UpdateEpisodesTotal(watch.TitleID, release.EpisodeCount); err != nil {
			return err
		}
	}
	remoteStatus := statusFrom(release.StatusID, release.StatusLabel)
	if remoteStatus == StatusCompleted {
		if err := l.storage.UpdateTitleStatus(watch.TitleID, int(remoteStatus)); err != nil {
			return err
		}
		return l.storage.DeleteEpisodeTracking(watch.TitleID)
	}
	if remoteStatus != Status(watch.Status) {
		if err := l.storage.UpdateTitleStatus(watch.TitleID, int(remoteStatus)); err != nil {
			return err
		}
	}

	if !watch.Tracked {
		// Existing history predates the tracking table. Record what is already
		// available without firing notifications for old episodes.
		return l.storage.SeedEpisodeTracking(watch.TitleID, release.Episode)
	}
	if release.Episode == "" {
		return nil
	}

	if release.Episode != watch.LastEpisode {
		if !libsocial.EpisodeNewer(release.Episode, watch.LastEpisode) {
			return nil
		}
		return l.notifyNewEpisode(watch, release, notify)
	}
	return l.notifyPreferredVoice(watch, release, notify)
}

func (l *Library) notifyNewEpisode(watch storage.EpisodeWatch, release *libsocial.EpisodeRelease, notify func(EpisodeNotification) error) error {
	if len(release.Voiceovers) == 0 {
		// The episode exists in the flat list but no team has published it yet.
		// Keep the previous marker so the first real release is still noticed.
		return nil
	}

	preferred := preferredVoice(release.Voiceovers, watch.Voiceover)
	event := EpisodeNotification{
		Title:         displayTitle(watch),
		Episode:       release.Episode,
		Voiceovers:    voiceoversForNotification(release.Voiceovers),
		TitleReleased: watch.Status == int(StatusAnnounced),
	}
	if err := sendNotification(notify, event); err != nil {
		return err
	}

	tracking := storage.EpisodeTracking{
		LastEpisode:      release.Episode,
		AnyVoiceNotified: true,
		// The first notification already covered the saved voice when that
		// voice was among the teams that published the episode.
		PreferredVoiceNotified: preferred,
	}
	return l.storage.SaveEpisodeTracking(watch.TitleID, tracking)
}

func (l *Library) notifyPreferredVoice(watch storage.EpisodeWatch, release *libsocial.EpisodeRelease, notify func(EpisodeNotification) error) error {
	if !watch.AnyVoiceNotified || watch.PreferredVoiceNotified || watch.Voiceover == "" {
		return nil
	}
	preferred := preferredVoice(release.Voiceovers, watch.Voiceover)
	if !preferred {
		return nil
	}

	event := EpisodeNotification{
		Title:      displayTitle(watch),
		Episode:    release.Episode,
		Voiceovers: voiceoversForNotification(filterVoiceovers(release.Voiceovers, watch.Voiceover)),
		Preferred:  true,
	}
	if err := sendNotification(notify, event); err != nil {
		return err
	}
	return l.storage.SaveEpisodeTracking(watch.TitleID, storage.EpisodeTracking{
		LastEpisode:            watch.LastEpisode,
		AnyVoiceNotified:       true,
		PreferredVoiceNotified: true,
	})
}

func sendNotification(notify func(EpisodeNotification) error, event EpisodeNotification) error {
	if notify == nil {
		return nil
	}
	return notify(event)
}

func preferredVoice(voices []libsocial.ReleaseVoiceover, name string) bool {
	if name == "" {
		return false
	}
	for _, voice := range voices {
		if voice.Name == name {
			return true
		}
	}
	return false
}

func filterVoiceovers(voices []libsocial.ReleaseVoiceover, name string) []libsocial.ReleaseVoiceover {
	out := make([]libsocial.ReleaseVoiceover, 0, 1)
	for _, voice := range voices {
		if voice.Name == name {
			out = append(out, voice)
		}
	}
	return out
}

func voiceoversForNotification(voices []libsocial.ReleaseVoiceover) []EpisodeVoiceover {
	out := make([]EpisodeVoiceover, 0, len(voices))
	for _, voice := range voices {
		out = append(out, EpisodeVoiceover{Name: voice.Name, KodikOnly: voice.KodikOnly})
	}
	return out
}

func displayTitle(watch storage.EpisodeWatch) string {
	if watch.RusName != "" {
		return watch.RusName
	}
	if watch.EngName != "" {
		return watch.EngName
	}
	return watch.Slug
}

// RefreshMissingEpisodeTotals fills the cached episode count for history rows
// saved before totals were stored. It downloads only what is missing and calls
// onUpdate after every title so the history list can redraw its progress bars
// without waiting for the whole library. Ongoing titles are kept current by the
// notification checker; this backfill mostly covers completed titles.
func (l *Library) RefreshMissingEpisodeTotals(ctx context.Context, onUpdate func()) error {
	if l == nil {
		return nil
	}
	titles, err := l.History()
	if err != nil {
		return err
	}
	missing := make([]Title, 0, len(titles))
	for _, title := range titles {
		if title.IsReading() {
			continue
		}
		if title.EpisodesTotal == 0 && title.Slug != "" {
			missing = append(missing, title)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	fetch := l.episodeCountFetcher
	if fetch == nil {
		token, err := l.readToken()
		if err != nil {
			return err
		}
		client := libsocial.NewBackground(token)
		fetch = client.EpisodeCount
	}

	var failures []error
	for _, title := range missing {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		total, err := fetch(ctx, title.Slug)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", title.Slug, err))
			continue
		}
		if total <= 0 {
			continue
		}
		if err := l.storage.UpdateEpisodesTotal(title.ID, total); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", title.Slug, err))
			continue
		}
		if onUpdate != nil {
			onUpdate()
		}
	}
	return errors.Join(failures...)
}

// seedEpisodeTracking records the latest episode already known when a title is
// saved, so enabling notifications later does not report the whole back
// catalogue as new.
func (l *Library) seedEpisodeTracking(title Title) error {
	if title.ID == 0 {
		return nil
	}
	// The episode checker polls AnimeLIB only; a Filmlib title must not seed
	// a row it will never update.
	if title.Source == filmlib.Source {
		return nil
	}
	if title.Status == StatusCompleted {
		return l.storage.DeleteEpisodeTracking(title.ID)
	}
	return l.storage.SeedEpisodeTracking(title.ID, latestEpisodeNumber(title.Episodes))
}

func latestEpisodeNumber(episodes []string) string {
	if len(episodes) == 0 {
		return ""
	}
	numbers := append([]string(nil), episodes...)
	libsocial.SortEpisodes(numbers)
	return numbers[len(numbers)-1]
}
