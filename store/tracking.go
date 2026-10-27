package store

import (
	"database/sql"
	"errors"
)

// ListEpisodeWatches returns every title in history with its tracking row.
// The caller decides which statuses should be polled.
func (s *Store) ListEpisodeWatches() ([]EpisodeWatch, error) {
	rows, err := s.db.Query(`
        SELECT t.id, t.slug, t.eng_name, t.rus_name, t.status, h.voiceover,
               e.tid IS NOT NULL,
               COALESCE(e.last_episode, ''),
               COALESCE(e.any_voice_notified, 0),
               COALESCE(e.preferred_voice_notified, 0)
        FROM history h
        JOIN titles t ON t.id = h.tid
        LEFT JOIN episode_tracking e ON e.tid = t.id
        WHERE t.source = ?
        ORDER BY h.sort_order ASC`, SourceAnimeLIB)
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
func (s *Store) SeedEpisodeTracking(tid int64, lastEpisode string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO episode_tracking (tid, last_episode)
        VALUES (?, ?)
        ON CONFLICT(tid) DO NOTHING`, tid, lastEpisode)
	return err
}

// SaveEpisodeTracking updates the latest seen episode and its notification
// flags.
func (s *Store) SaveEpisodeTracking(tid int64, tracking EpisodeTracking) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO episode_tracking (tid, last_episode, any_voice_notified, preferred_voice_notified)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(tid) DO UPDATE SET
            last_episode             = excluded.last_episode,
            any_voice_notified       = excluded.any_voice_notified,
            preferred_voice_notified = excluded.preferred_voice_notified`,
		tid, tracking.LastEpisode, boolInt(tracking.AnyVoiceNotified), boolInt(tracking.PreferredVoiceNotified))
	return err
}

// DeleteEpisodeTracking stops tracking a title, for example when it completes
// or leaves history.
func (s *Store) DeleteEpisodeTracking(tid int64) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM episode_tracking WHERE tid = ?", tid)
	return err
}

// GetIntrodbCache returns cached chapters for one episode. The boolean is true
// when introdb previously reported that it has no entry.
func (s *Store) GetIntrodbCache(tid int64, episode string) ([]byte, bool, bool) {
	var chapters string
	var noData int
	err := s.db.QueryRow(`
        SELECT chapters, no_data FROM introdb_cache WHERE tid = ? AND episode = ?`,
		tid, episode).Scan(&chapters, &noData)
	if err != nil {
		return nil, false, false
	}
	return []byte(chapters), noData != 0, true
}

// SaveIntrodbCache stores chapters (JSON) or a no-data marker.
func (s *Store) SaveIntrodbCache(tid int64, episode string, chapters []byte, noData bool) error {
	if tid == 0 || episode == "" {
		return nil
	}
	value := 0
	if noData {
		value = 1
	}
	_, err := s.db.Exec(`
        INSERT INTO introdb_cache (tid, episode, chapters, no_data)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(tid, episode) DO UPDATE SET
            chapters = excluded.chapters,
            no_data  = excluded.no_data`,
		tid, episode, string(chapters), value)
	return err
}

// GetTitleQualities returns the quality ladder cached for a saved Kinopoisk
// title. The ladder is JSON keyed by voice name; an empty string means the
// title page has to probe it again.
func (s *Store) GetTitleQualities(tid int64) (string, error) {
	if tid == 0 {
		return "", nil
	}
	var data string
	if err := s.db.QueryRow("SELECT data FROM title_qualities WHERE tid = ?", tid).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return data, nil
}

// SaveTitleQualities caches the quality ladders of a saved Kinopoisk title. Only
// a non-empty payload is written: a failed probe must not erase the ladder the
// previous save managed to learn.
func (s *Store) SaveTitleQualities(tid int64, data string) error {
	if tid == 0 || data == "" {
		return nil
	}
	_, err := s.db.Exec(`
        INSERT INTO title_qualities (tid, data) VALUES (?, ?)
        ON CONFLICT(tid) DO UPDATE SET data = excluded.data`,
		tid, data)
	return err
}
