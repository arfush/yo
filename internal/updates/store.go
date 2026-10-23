package updates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State is persisted in %APPDATA%\yonime\updates.json. It holds only the last
// check time and the release cached for the footer button; credentials never
// enter this file.
type State struct {
	LastCheck time.Time `json:"last_check,omitempty"`
	Available *Release  `json:"available,omitempty"`
}

// Store owns the update state file. All methods are safe for concurrent use.
type Store struct {
	path string

	mu    sync.Mutex
	state State
}

// Open loads the update state. A missing file is an empty state. A corrupt file
// is moved aside instead of blocking the application; it will be replaced on
// the next save.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("updates: empty state path")
	}
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("read update state: %w", err)
	}
	if len(data) == 0 {
		return store, nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		_ = os.Rename(path, path+".broken")
		return store, nil
	}
	if state.Available != nil && !state.Available.Valid() {
		state.Available = nil
	}
	store.state = state
	return store, nil
}

// State returns a copy of the current state.
func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() State {
	state := s.state
	if state.Available != nil {
		release := *state.Available
		state.Available = &release
	}
	return state
}

// RecordCheck stores the result of a successful release request. A nil release
// means GitHub currently publishes no usable release.
func (s *Store) RecordCheck(release *Release, checked time.Time) error {
	var copyRelease *Release
	if release != nil {
		if !release.Valid() {
			return fmt.Errorf("updates: invalid release")
		}
		value := *release
		copyRelease = &value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LastCheck = checked
	s.state.Available = copyRelease
	return s.saveLocked()
}

// SaveLastCheck updates the check timestamp without touching the cached
// release. It is used when the feed request failed but the cached release is
// still valid.
func (s *Store) SaveLastCheck(checked time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.LastCheck = checked
	return s.saveLocked()
}

// ClearAvailable drops the cached release.
func (s *Store) ClearAvailable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Available = nil
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update state: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create update state directory: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write update state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		// Windows can fail to replace the file when another process has it
		// open. Removing the target first keeps the update atomic enough for
		// this small cache file.
		_ = os.Remove(s.path)
		if retry := os.Rename(temporary, s.path); retry != nil {
			_ = os.Remove(temporary)
			return fmt.Errorf("replace update state: %w", retry)
		}
	}
	return nil
}
