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

// NewStore loads path, starting empty when the file is absent.
func NewStore(path string) *Store {
	s := &Store{path: path, items: map[string]models.Schedule{}}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return s
	}
	var list []models.Schedule
	if err := json.Unmarshal(raw, &list); err != nil {
		return s
	}
	for _, item := range list {
		if item.ID != "" {
			s.items[item.ID] = item
		}
	}
	return s
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
	in.Enabled = true
	s.items[in.ID] = in
	return in, s.saveLocked()
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
	s.items[id] = in
	return in, s.saveLocked()
}

// Delete removes a schedule by id.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	delete(s.items, id)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	list := make([]models.Schedule, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".schedules-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
