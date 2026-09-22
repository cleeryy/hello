package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cleeryy/hello/internal/models"
)

var (
	ErrNotFound      = errors.New("schedule not found")
	ErrAlreadyExists = errors.New("schedule already exists")
)

// Store persists schedules in one JSON file with atomic saves.
type Store struct {
	mu    sync.RWMutex
	path  string
	items map[string]models.Schedule
}

// NewStore loads path and fails fast when an existing file is corrupt.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, items: map[string]models.Schedule{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("scheduler: read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return s, nil
	}
	var list []models.Schedule
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("scheduler: decode %s: %w", path, err)
	}
	for i := range list {
		if err := list[i].Validate(); err != nil {
			return nil, fmt.Errorf("scheduler: validate %s at index %d: %w", path, i, err)
		}
		if _, exists := s.items[list[i].ID]; exists {
			return nil, fmt.Errorf("scheduler: decode %s: duplicate schedule id %q", path, list[i].ID)
		}
		s.items[list[i].ID] = list[i]
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("scheduler: chmod %s: %w", path, err)
	}
	return s, nil
}

// All returns every schedule in no guaranteed order.
func (s *Store) All() []models.Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Schedule, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out
}

// Get returns a copy by id.
func (s *Store) Get(id string) (models.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return models.Schedule{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return item, nil
}

// Create validates, enables and persists a new schedule.
func (s *Store) Create(in models.Schedule) (models.Schedule, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return models.Schedule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[in.ID]; ok {
		return models.Schedule{}, fmt.Errorf("%w: %s", ErrAlreadyExists, in.ID)
	}
	previous := make(map[string]models.Schedule, len(s.items)+1)
	for id, item := range s.items {
		previous[id] = item
	}
	in.Enabled = true
	s.items[in.ID] = in
	if err := s.saveLocked(); err != nil {
		s.items = previous
		return models.Schedule{}, err
	}
	return in, nil
}

// Update replaces the schedule kept under id, keeping the path id.
func (s *Store) Update(id string, in models.Schedule) (models.Schedule, error) {
	in.ID = id
	in.Normalize()
	if err := in.Validate(); err != nil {
		return models.Schedule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return models.Schedule{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	previous := make(map[string]models.Schedule, len(s.items))
	for itemID, item := range s.items {
		previous[itemID] = item
	}
	s.items[id] = in
	if err := s.saveLocked(); err != nil {
		s.items = previous
		return models.Schedule{}, err
	}
	return in, nil
}

// Delete removes a schedule by id.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	previous := make(map[string]models.Schedule, len(s.items))
	for itemID, item := range s.items {
		previous[itemID] = item
	}
	delete(s.items, id)
	if err := s.saveLocked(); err != nil {
		s.items = previous
		return err
	}
	return nil
}

func (s *Store) saveLocked() error {
	list := make([]models.Schedule, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("scheduler: encode: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".schedules-*")
	if err != nil {
		return fmt.Errorf("scheduler: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("scheduler: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("scheduler: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("scheduler: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("scheduler: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("scheduler: replace %s: %w", s.path, err)
	}
	return nil
}
