package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cleeryy/hello/internal/models"
)

var (
	ErrNotFound      = errors.New("schedule not found")
	ErrAlreadyExists = errors.New("schedule already exists")
	ErrTooMany       = errors.New("schedule quota reached")
)

// defaultCap bounds the number of stored schedules.
const defaultCap = 100

// Store persists schedules in one JSON file with atomic saves.
type Store struct {
	mu    sync.RWMutex
	path  string
	items map[string]models.Schedule
	cap   int
}

// NewStore loads path and fails fast when an existing file is corrupt.
func NewStore(path string) (*Store, error) {
	return NewStoreWithCap(path, defaultCap)
}

// NewStoreWithCap loads path with a custom schedule quota.
func NewStoreWithCap(path string, cap int) (*Store, error) {
	if cap <= 0 {
		cap = defaultCap
	}
	s := &Store{path: path, items: map[string]models.Schedule{}, cap: cap}
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
	// NextRun is serve-time only and must never leak into the file.
	in.NextRun = 0
	if err := in.Validate(); err != nil {
		return models.Schedule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[in.ID]; ok {
		return models.Schedule{}, fmt.Errorf("%w: %s", ErrAlreadyExists, in.ID)
	}
	if len(s.items) >= s.cap {
		return models.Schedule{}, fmt.Errorf("%w: at most %d schedules", ErrTooMany, s.cap)
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
	in.NextRun = 0
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

// SetAllEnabled flips every schedule at once, persists once, and returns
// the number of schedules that actually changed state.
func (s *Store) SetAllEnabled(enabled bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := 0
	previous := make(map[string]models.Schedule, len(s.items))
	for id, item := range s.items {
		previous[id] = item
		if item.Enabled != enabled {
			item.Enabled = enabled
			s.items[id] = item
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}
	if err := s.saveLocked(); err != nil {
		s.items = previous
		return 0, err
	}
	return changed, nil
}

// MarkFired records a fire on a schedule. A consumed one-shot schedule is
// disabled and detached from its single fire time, keeping the cron row as
// history. Unknown ids are ignored: the runner may race a delete.
func (s *Store) MarkFired(id string, ok bool, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, found := s.items[id]
	if !found {
		return
	}
	item.LastRunAt = time.Now().Unix()
	if ok {
		item.LastResult = "ok"
	} else {
		item.LastResult = "error: " + truncateDetail(detail)
	}
	if item.Once {
		item.Enabled = false
		item.Once = false
	}
	previous := s.items[id]
	s.items[id] = item
	if err := s.saveLocked(); err != nil {
		s.items[id] = previous
	}
}

func truncateDetail(detail string) string {
	const maxDetailRunes = 200
	runes := []rune(strings.TrimSpace(detail))
	if len(runes) <= maxDetailRunes {
		return string(runes)
	}
	return string(runes[:maxDetailRunes])
}

// DisableStaleOnce switches off enabled one-shot schedules whose fire time
// already passed (for example missed during downtime). It returns the count
// of disabled schedules and persists once when nonzero.
func (s *Store) DisableStaleOnce(now int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	disabled := 0
	previous := make(map[string]models.Schedule, len(s.items))
	for id, item := range s.items {
		previous[id] = item
		if item.Enabled && item.Once && item.OnceAt <= now {
			item.Enabled = false
			item.Once = false
			s.items[id] = item
			disabled++
		}
	}
	if disabled == 0 {
		return 0
	}
	if err := s.saveLocked(); err != nil {
		s.items = previous
		return 0
	}
	return disabled
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
