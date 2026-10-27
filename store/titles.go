package store

import (
	"database/sql"
	"errors"
	"time"
)

// UpsertTitle stores fresh metadata and returns the local title id.
func (s *Store) UpsertTitle(title Title) (int64, error) {
	if title.UpdatedAt == 0 {
		title.UpdatedAt = time.Now().Unix()
	}
	userStatus := title.UserStatus
	if userStatus <= 0 {
		userStatus = 2
	}
	_, err := s.db.Exec(`
        INSERT INTO titles (slug, source, external_id, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(slug) DO UPDATE SET
            source         = excluded.source,
            external_id    = excluded.external_id,
            eng_name       = CASE WHEN excluded.eng_name != '' THEN excluded.eng_name ELSE titles.eng_name END,
            rus_name       = CASE WHEN excluded.rus_name != '' THEN excluded.rus_name ELSE titles.rus_name END,
            imdb_id        = CASE WHEN excluded.imdb_id != '' THEN excluded.imdb_id ELSE titles.imdb_id END,
            season         = CASE WHEN excluded.season != 0 THEN excluded.season ELSE titles.season END,
            status         = excluded.status,
            user_status    = titles.user_status,
            episodes_total = CASE WHEN excluded.episodes_total > 0 THEN excluded.episodes_total ELSE titles.episodes_total END,
            is_movie       = excluded.is_movie,
            updated_at     = excluded.updated_at
    `, title.Slug, titleSource(title), title.ExternalID, title.EngName, title.RusName, title.IMDbID, title.Season, title.Status, userStatus, title.EpisodesTotal, boolInt(title.IsMovie), title.UpdatedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRow("SELECT id FROM titles WHERE slug = ?", title.Slug).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// GetTitle returns a cached title by slug.
func (s *Store) GetTitle(slug string) (*Title, error) {
	row := s.db.QueryRow(`
        SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
        FROM titles WHERE slug = ?`, slug)
	return scanTitle(row)
}

// GetTitleByID returns a cached title by local id.
func (s *Store) GetTitleByID(tid int64) (*Title, error) {
	row := s.db.QueryRow(`
        SELECT id, source, external_id, slug, eng_name, rus_name, imdb_id, season, status, user_status, episodes_total, is_movie, updated_at
        FROM titles WHERE id = ?`, tid)
	return scanTitle(row)
}

// UpdateEpisodesTotal refreshes the cached number of episodes after a
// background check found a newly released episode.
func (s *Store) UpdateEpisodesTotal(tid int64, total int) error {
	if tid == 0 || total <= 0 {
		return nil
	}
	_, err := s.db.Exec("UPDATE titles SET episodes_total = ?, updated_at = ? WHERE id = ?", total, time.Now().Unix(), tid)
	return err
}

// UpdateTitleStatus refreshes the cached status after a background check.
func (s *Store) UpdateTitleStatus(tid int64, status int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec("UPDATE titles SET status = ?, updated_at = ? WHERE id = ?", status, time.Now().Unix(), tid)
	return err
}

// SetUserStatus stores the local library status for a title. A changed status
// also moves the title to the top of its new status list, while the all-titles
// order stays as the user arranged it.
func (s *Store) SetUserStatus(tid int64, status int) error {
	changed, err := s.setUserStatus(tid, status, 0, false)
	if err != nil {
		logger.Error("cannot save user status", "title", tid, "status", status, "error", err)
		return err
	}
	if changed {
		logger.Info("user status saved", "title", tid, "status", status)
	}
	return nil
}

// SetUserStatusFrom changes a status only when it still has the expected old
// value. It is used to promote a planned title to "Смотрю" when playback starts
// without overwriting a choice the user made in the meantime.
func (s *Store) SetUserStatusFrom(tid int64, from, to int) error {
	_, err := s.setUserStatus(tid, to, from, true)
	return err
}

// setUserStatus applies a status change in one transaction. requireFrom makes
// the write conditional on the status the caller last saw.
func (s *Store) setUserStatus(tid int64, status, from int, requireFrom bool) (bool, error) {
	if tid == 0 {
		return false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var current int
	err = tx.QueryRow("SELECT user_status FROM titles WHERE id = ?", tid).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	if requireFrom && current != from {
		return false, tx.Commit()
	}
	if current == status {
		return false, tx.Commit()
	}
	statusOrder, err := nextStatusSortOrder(tx, status)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec("UPDATE titles SET user_status = ? WHERE id = ?", status, tid); err != nil {
		return false, err
	}
	if _, err := tx.Exec("UPDATE history SET status_order = ? WHERE tid = ?", statusOrder, tid); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// UpdateNames refreshes the localized/English names discovered during
// playback. Empty values do not overwrite existing names.
func (s *Store) UpdateNames(tid int64, english, russian string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        UPDATE titles SET
            eng_name = CASE WHEN ? != '' THEN ? ELSE eng_name END,
            rus_name = CASE WHEN ? != '' THEN ? ELSE rus_name END
        WHERE id = ?`,
		english, english, russian, russian, tid)
	return err
}

// UpdateMetadata stores the IMDb id and franchise season discovered during
// playback. Empty values do not overwrite existing metadata.
func (s *Store) UpdateMetadata(tid int64, imdbID string, season int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(`
        UPDATE titles SET
            imdb_id = CASE WHEN ? != '' THEN ? ELSE imdb_id END,
            season  = CASE WHEN ? > 0 THEN ? ELSE season END
        WHERE id = ?`,
		imdbID, imdbID, season, season, tid)
	return err
}
