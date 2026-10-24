package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/arfush/yonime/internal/animelib"
	"github.com/arfush/yonime/internal/store"
)

// EpisodeVoiceover is one translation team that published a new episode.
type EpisodeVoiceover struct {
	Name string
	// KodikOnly reports that AnimeLIB's own player does not carry this episode,
	// so it will be watched through Kodik.
	KodikOnly bool
}

// EpisodeNotification describes one notification the background checker wants
// to show. Preferred marks the second notification, sent when the user's own
// saved voiceover catches up with an episode that was already announced for
// another team.
type EpisodeNotification struct {
	Title         string
	Episode       string
	Voiceovers    []EpisodeVoiceover
	TitleReleased bool
	Preferred     bool
}

// episodeFetcher is replaced by tests so the checker can run without a network.
// Production leaves it nil and uses the paced background AnimeLIB client.
type episodeFetcher func(context.Context, string) (*animelib.EpisodeRelease, error)

// EpisodeNotificationsEnabled reports whether the user wants new-episode
// notifications.
func (l *Library) EpisodeNotificationsEnabled() bool {
	if l == nil {
		return false
	}
	return l.store.EpisodeNotificationsEnabled()
}

// SetEpisodeNotificationsEnabled stores the notification choice.
func (l *Library) SetEpisodeNotificationsEnabled(enabled bool) error {
	if l == nil {
		return errors.New("база данных недоступна")
	}
	return l.store.SetEpisodeNotificationsEnabled(enabled)
}

// CheckEpisodeNotifications polls every saved ongoing or announced title and
// returns notifications through notify. The tracking table is updated only
// after a notification has been delivered, so a failed delivery is retried on
// the next pass instead of being silently lost.
func (l *Library) CheckEpisodeNotifications(ctx context.Context, notify func(EpisodeNotification) error) error {
	if l == nil || !l.store.EpisodeNotificationsEnabled() {
		return nil
	}
	watches, err := l.store.ListEpisodeWatches()
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
		client := animelib.NewBackground(token)
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
			if err := l.store.DeleteEpisodeTracking(watch.TitleID); err != nil {
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

func (l *Library) processEpisodeRelease(watch store.EpisodeWatch, release *animelib.EpisodeRelease, notify func(EpisodeNotification) error) error {
	if release == nil {
		return nil
	}
	remoteStatus := statusFrom(release.StatusID, release.StatusLabel)
	if remoteStatus == StatusCompleted {
		if err := l.store.UpdateTitleStatus(watch.TitleID, int(remoteStatus)); err != nil {
			return err
		}
		return l.store.DeleteEpisodeTracking(watch.TitleID)
	}
	if remoteStatus != Status(watch.Status) {
		if err := l.store.UpdateTitleStatus(watch.TitleID, int(remoteStatus)); err != nil {
			return err
		}
	}

	if !watch.Tracked {
		// Existing history predates the tracking table. Record what is already
		// available without firing notifications for old episodes.
		return l.store.SeedEpisodeTracking(watch.TitleID, release.Episode)
	}
	if release.Episode == "" {
		return nil
	}

	if release.Episode != watch.LastEpisode {
		if !animelib.EpisodeNewer(release.Episode, watch.LastEpisode) {
			return nil
		}
		return l.notifyNewEpisode(watch, release, notify)
	}
	return l.notifyPreferredVoice(watch, release, notify)
}

func (l *Library) notifyNewEpisode(watch store.EpisodeWatch, release *animelib.EpisodeRelease, notify func(EpisodeNotification) error) error {
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

	tracking := store.EpisodeTracking{
		LastEpisode:      release.Episode,
		AnyVoiceNotified: true,
		// The first notification already covered the saved voice when that
		// voice was among the teams that published the episode.
		PreferredVoiceNotified: preferred,
	}
	return l.store.SaveEpisodeTracking(watch.TitleID, tracking)
}

func (l *Library) notifyPreferredVoice(watch store.EpisodeWatch, release *animelib.EpisodeRelease, notify func(EpisodeNotification) error) error {
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
	return l.store.SaveEpisodeTracking(watch.TitleID, store.EpisodeTracking{
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

func preferredVoice(voices []animelib.ReleaseVoiceover, name string) bool {
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

func filterVoiceovers(voices []animelib.ReleaseVoiceover, name string) []animelib.ReleaseVoiceover {
	out := make([]animelib.ReleaseVoiceover, 0, 1)
	for _, voice := range voices {
		if voice.Name == name {
			out = append(out, voice)
		}
	}
	return out
}

func voiceoversForNotification(voices []animelib.ReleaseVoiceover) []EpisodeVoiceover {
	out := make([]EpisodeVoiceover, 0, len(voices))
	for _, voice := range voices {
		out = append(out, EpisodeVoiceover{Name: voice.Name, KodikOnly: voice.KodikOnly})
	}
	return out
}

func displayTitle(watch store.EpisodeWatch) string {
	if watch.RusName != "" {
		return watch.RusName
	}
	if watch.EngName != "" {
		return watch.EngName
	}
	return watch.Slug
}

// SendTestEpisodeNotifications emits one fake notification for every history
// title in the ongoing or announced state. The titles, statuses and saved
// voiceovers are real; only the episode number is invented. It exists solely to
// exercise notification formatting and Windows delivery and will be removed
// once the feature has been checked by hand.
func (l *Library) SendTestEpisodeNotifications(notify func(EpisodeNotification) error) (int, error) {
	if l == nil {
		return 0, errors.New("база данных недоступна")
	}
	entries, err := l.store.ListHistory()
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, entry := range entries {
		status := Status(entry.Status)
		if status != StatusOngoing && status != StatusAnnounced {
			continue
		}
		episode := entry.Episode
		if episode == "" {
			episode = "1"
		}
		voice := entry.Voiceover
		if voice == "" {
			voice = "AnimeLIB"
		}
		event := EpisodeNotification{
			Title:         historyDisplayTitle(entry),
			Episode:       episode,
			Voiceovers:    []EpisodeVoiceover{{Name: voice}},
			TitleReleased: status == StatusAnnounced,
		}
		if err := sendNotification(notify, event); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func historyDisplayTitle(entry store.HistoryEntry) string {
	if entry.RusName != "" {
		return entry.RusName
	}
	if entry.EngName != "" {
		return entry.EngName
	}
	return entry.Slug
}

// seedEpisodeTracking records the latest episode already known when a title is
// saved, so enabling notifications later does not report the whole back
// catalogue as new.
func (l *Library) seedEpisodeTracking(title Title) error {
	if title.ID == 0 {
		return nil
	}
	if title.Status == StatusCompleted {
		return l.store.DeleteEpisodeTracking(title.ID)
	}
	return l.store.SeedEpisodeTracking(title.ID, latestEpisodeNumber(title.Episodes))
}

func latestEpisodeNumber(episodes []string) string {
	if len(episodes) == 0 {
		return ""
	}
	numbers := append([]string(nil), episodes...)
	animelib.SortEpisodes(numbers)
	return numbers[len(numbers)-1]
}
