package storage

import (
	"database/sql"
	"errors"
	"time"
)

// UpsertTitle stores fresh metadata and returns the local title id.
func (s *Storage) UpsertTitle(title Title) (int64, error) {
	if title.UpdatedAt == 0 {
		title.UpdatedAt = time.Now().Unix()
	}
	userStatus := title.UserStatus
	if userStatus <= 0 {
		userStatus = 2
	}
	_, err := s.db.Exec(sqlQuery("title.upsert"), title.Slug, titleSource(title), title.ExternalID, title.EngName, title.RusName, title.IMDbID, title.Season, title.Status, userStatus, title.EpisodesTotal, boolInt(title.IsMovie), title.UpdatedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRow(sqlQuery("title.id_by_slug"), title.Slug).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// GetTitle returns a cached title by slug.
func (s *Storage) GetTitle(slug string) (*Title, error) {
	row := s.db.QueryRow(sqlQuery("title.by_slug"), slug)
	return scanTitle(row)
}

// GetTitleByID returns a cached title by local id.
func (s *Storage) GetTitleByID(tid int64) (*Title, error) {
	row := s.db.QueryRow(sqlQuery("title.by_id"), tid)
	return scanTitle(row)
}

// UpdateEpisodesTotal refreshes the cached number of episodes after a
// background check found a newly released episode.
func (s *Storage) UpdateEpisodesTotal(tid int64, total int) error {
	if tid == 0 || total <= 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("title.update_episodes_total"), total, time.Now().Unix(), tid)
	return err
}

// UpdateTitleStatus refreshes the cached status after a background check.
func (s *Storage) UpdateTitleStatus(tid int64, status int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("title.update_status"), status, time.Now().Unix(), tid)
	return err
}

// SetUserStatus stores the local library status for a title. A changed status
// also moves the title to the top of its new status list, while the all-titles
// order stays as the user arranged it.
func (s *Storage) SetUserStatus(tid int64, status int) error {
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
// value. It is used to promote a planned title to "Watching" when playback starts
// without overwriting a choice the user made in the meantime.
func (s *Storage) SetUserStatusFrom(tid int64, from, to int) error {
	_, err := s.setUserStatus(tid, to, from, true)
	return err
}

// setUserStatus applies a status change in one transaction. requireFrom makes
// the write conditional on the status the caller last saw.
func (s *Storage) setUserStatus(tid int64, status, from int, requireFrom bool) (bool, error) {
	if tid == 0 {
		return false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var current int
	err = tx.QueryRow(sqlQuery("title.user_status"), tid).Scan(&current)
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
	if _, err := tx.Exec(sqlQuery("title.update_user_status"), status, tid); err != nil {
		return false, err
	}
	if _, err := tx.Exec(sqlQuery("title.update_history_status_order"), statusOrder, tid); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// UpdateNames refreshes the localized/English names discovered during
// playback. Empty values do not overwrite existing names.
func (s *Storage) UpdateNames(tid int64, english, russian string) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("title.update_names"), english, english, russian, russian, tid)
	return err
}

// UpdateMetadata stores the IMDb id and franchise season discovered during
// playback. Empty values do not overwrite existing metadata.
func (s *Storage) UpdateMetadata(tid int64, imdbID string, season int) error {
	if tid == 0 {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("title.update_metadata"), imdbID, imdbID, season, season, tid)
	return err
}
