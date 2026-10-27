package storage

import (
	"database/sql"
	"errors"
)

// ListEpisodeWatches returns every title in history with its tracking row.
// The caller decides which statuses should be polled.
func (s *Storage) ListEpisodeWatches() ([]EpisodeWatch, error) {
	rows, err := s.db.Query(sqlQuery("tracking.list_watches"), SourceLibSocial)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EpisodeWatch
	for rows.Next() {
		var watch EpisodeWatch
		var tracked, anyNotified, preferredNotified int
		if err := rows.Scan(
			&watch.TitleID, &watch.Slug, &watch.EngName, &watch.RusName, &watch.Status, &watch.Voiceover,
			&tracked, &watch.LastEpisode, &anyNotified, &preferredNotified,
		); err != nil {
			return nil, err
		}
		watch.Tracked = tracked != 0
		watch.AnyVoiceNotified = anyNotified != 0
		watch.PreferredVoiceNotified = preferredNotified != 0
		out = append(out, watch)
	}
	return out, rows.Err()
}

// SeedEpisodeTracking starts tracking a title without notifying about episodes
// that were already available when it was saved. An existing row is kept, so
// pressing Save again never rewinds the notification state.
func (s *Storage) SeedEpisodeTracking(tid int64, lastEpisode string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("tracking.seed"), tid, lastEpisode)
	return err
}

// SaveEpisodeTracking updates the latest seen episode and its notification
// flags.
func (s *Storage) SaveEpisodeTracking(tid int64, tracking EpisodeTracking) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("tracking.upsert"), tid, tracking.LastEpisode, boolInt(tracking.AnyVoiceNotified), boolInt(tracking.PreferredVoiceNotified))
	return err
}

// DeleteEpisodeTracking stops tracking a title, for example when it completes
// or leaves history.
func (s *Storage) DeleteEpisodeTracking(tid int64) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("tracking.delete"), tid)
	return err
}

// GetIntrodbCache returns cached chapters for one episode. The boolean is true
// when introdb previously reported that it has no entry.
func (s *Storage) GetIntrodbCache(tid int64, episode string) ([]byte, bool, bool) {
	var chapters string
	var noData int
	err := s.db.QueryRow(sqlQuery("introdb.get"), tid, episode).Scan(&chapters, &noData)
	if err != nil {
		return nil, false, false
	}
	return []byte(chapters), noData != 0, true
}

// SaveIntrodbCache stores chapters (JSON) or a no-data marker.
func (s *Storage) SaveIntrodbCache(tid int64, episode string, chapters []byte, noData bool) error {
	if tid == 0 || episode == "" {
		return nil
	}
	value := 0
	if noData {
		value = 1
	}
	_, err := s.db.Exec(sqlQuery("introdb.upsert"), tid, episode, string(chapters), value)
	return err
}

// GetTitleQualities returns the quality ladder cached for a saved Filmlib
// title. The ladder is JSON keyed by voice name; an empty string means the
// title page has to probe it again.
func (s *Storage) GetTitleQualities(tid int64) (string, error) {
	if tid == 0 {
		return "", nil
	}
	var data string
	if err := s.db.QueryRow(sqlQuery("qualities.get"), tid).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return data, nil
}

// SaveTitleQualities caches the quality ladders of a saved Filmlib title. Only
// a non-empty payload is written: a failed probe must not erase the ladder the
// previous save managed to learn.
func (s *Storage) SaveTitleQualities(tid int64, data string) error {
	if tid == 0 || data == "" {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("qualities.upsert"), tid, data)
	return err
}
