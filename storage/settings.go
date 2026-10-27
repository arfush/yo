package storage

import "strconv"

// Settings returns the raw value of a settings row. The boolean is false when
// the key was never stored.
func (s *Storage) Settings(key string) (string, bool) {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), key).Scan(&value); err != nil {
		return "", false
	}
	return value, true
}

// SetSettings stores a raw settings row.
func (s *Storage) SetSettings(key, value string) error {
	if key == "" {
		return nil
	}
	_, err := s.db.Exec(sqlQuery("settings.upsert"), key, value)
	return err
}

// boolSetting reads a boolean setting and falls back to defaultValue when the
// row is missing or unreadable.
func (s *Storage) boolSetting(key string, defaultValue bool) bool {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), key).Scan(&value); err != nil {
		return defaultValue
	}
	return value != "0" && value != "false"
}

// setBoolSetting stores a boolean setting.
func (s *Storage) setBoolSetting(key string, enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	return s.SetSettings(key, value)
}

// EpisodeNotificationsEnabled reports whether the user wants new-episode
// notifications. The setting is enabled by default for a fresh database.
func (s *Storage) EpisodeNotificationsEnabled() bool {
	return s.boolSetting("episode_notifications", true)
}

// SetEpisodeNotificationsEnabled stores the new-episode notification choice.
func (s *Storage) SetEpisodeNotificationsEnabled(enabled bool) error {
	return s.setBoolSetting("episode_notifications", enabled)
}

// AutoUpdateEnabled reports whether the application may install updates
// without asking. It is on by default so a fresh install picks up the latest
// release; an explicit "0" row still turns it off for good.
func (s *Storage) AutoUpdateEnabled() bool {
	return s.boolSetting("auto_update", true)
}

// SetAutoUpdateEnabled stores the automatic-update choice.
func (s *Storage) SetAutoUpdateEnabled(enabled bool) error {
	return s.setBoolSetting("auto_update", enabled)
}

// BetaUpdatesEnabled reports whether patch releases (major.minor.patch) may be
// installed. It is off by default because patch builds may be less polished.
func (s *Storage) BetaUpdatesEnabled() bool {
	return s.boolSetting("beta_updates", false)
}

// SetBetaUpdatesEnabled stores the beta-testing choice.
func (s *Storage) SetBetaUpdatesEnabled(enabled bool) error {
	return s.setBoolSetting("beta_updates", enabled)
}

// PreferredQuality returns the globally remembered quality, defaulting to 1080.
func (s *Storage) PreferredQuality() int {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), "quality").Scan(&value); err == nil {
		if quality, err := strconv.Atoi(value); err == nil && quality > 0 {
			return quality
		}
	}
	return 1080
}

// SetPreferredQuality remembers the quality globally.
func (s *Storage) SetPreferredQuality(quality int) error {
	if quality <= 0 {
		return nil
	}
	return s.SetSettings("quality", strconv.Itoa(quality))
}

// SharedControlEnabled reports whether watch-party guests may pause, resume and
// seek the host's player. It is enabled by default so a room behaves the way
// people expect without opening the settings first.
func (s *Storage) SharedControlEnabled() bool {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), "party_control").Scan(&value); err != nil {
		return true
	}
	return value != "0" && value != "false"
}

// SetSharedControlEnabled stores the shared-control choice.
func (s *Storage) SetSharedControlEnabled(enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	return s.SetSettings("party_control", value)
}

// AutoSkipSegment reports whether chapters of this introdb segment type are
// skipped automatically. Auto-skip is opt-in, so a missing row means off.
func (s *Storage) AutoSkipSegment(segment string) bool {
	return s.boolSetting("autoskip_"+segment, false)
}

// SetAutoSkipSegment stores the per-type auto-skip choice.
func (s *Storage) SetAutoSkipSegment(segment string, enabled bool) error {
	return s.setBoolSetting("autoskip_"+segment, enabled)
}

// PartyProvider returns the relay the user picked for new rooms. An empty
// string means the user never picked one and the first configured relay wins.
func (s *Storage) PartyProvider() string {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), "party_provider").Scan(&value); err != nil {
		return ""
	}
	return value
}

// SetPartyProvider remembers the relay chosen in the settings.
func (s *Storage) SetPartyProvider(provider string) error {
	return s.SetSettings("party_provider", provider)
}

// PartyName returns the nickname shown in watch-party rooms. An empty
// string means the user never picked one and the UI generates a default.
func (s *Storage) PartyName() string {
	var value string
	if err := s.db.QueryRow(sqlQuery("settings.get"), "party_name").Scan(&value); err != nil {
		return ""
	}
	return value
}

// SetPartyName remembers the nickname used in watch-party rooms.
func (s *Storage) SetPartyName(name string) error {
	return s.SetSettings("party_name", name)
}
