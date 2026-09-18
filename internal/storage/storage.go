package storage

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
	// ErrNotFound is returned when no device matches the requested id.
	ErrNotFound = errors.New("storage: device not found")
	// ErrAlreadyExists is returned when creating a device with a duplicate id.
	ErrAlreadyExists = errors.New("storage: device already exists")
)

// Storage is a file-backed device registry safe for concurrent use.
type Storage struct {
	mu      sync.RWMutex
	devices map[string]*models.Device
	file    string
}

// New opens the registry at filepath, loading existing entries when present.
func New(filepath string) *Storage {
	s := &Storage{
		devices: make(map[string]*models.Device),
		file:    filepath,
	}
	_ = s.Load()
	return s
}

// Load replaces the in-memory registry with the file content.
// A missing file means an empty registry, not an error.
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("storage: read %s: %w", s.file, err)
	}

	var devices []*models.Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return fmt.Errorf("storage: decode %s: %w", s.file, err)
	}

	fresh := make(map[string]*models.Device, len(devices))
	for _, d := range devices {
		if d == nil {
			continue
		}
		fresh[d.ID] = d
	}
	s.devices = fresh
	return nil
}

// save writes the registry atomically (temp file + rename).
// Callers must hold the write lock.
func (s *Storage) save() error {
	devices := make([]*models.Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, d)
	}

	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: encode: %w", err)
	}

	dir := filepath.Dir(s.file)
	tmp, err := os.CreateTemp(dir, ".devices-*.tmp")
	if err != nil {
		return fmt.Errorf("storage: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("storage: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.file); err != nil {
		return fmt.Errorf("storage: replace %s: %w", s.file, err)
	}
	return nil
}

func clone(d *models.Device) *models.Device {
	cpy := *d
	return &cpy
}

// Save persists the current registry.
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// Create validates and inserts a device.
func (s *Storage) Create(device *models.Device) error {
	device.Normalize()
	if err := device.Validate(); err != nil {
		return fmt.Errorf("storage: create: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[device.ID]; exists {
		return fmt.Errorf("storage: create %s: %w", device.ID, ErrAlreadyExists)
	}

	s.devices[device.ID] = clone(device)
	return s.save()
}

// GetAll returns a snapshot copy of every device.
func (s *Storage) GetAll() []*models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*models.Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, clone(d))
	}
	return devices
}

// Get returns a copy of the device with the given id.
func (s *Storage) Get(id string) (*models.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.devices[id]
	if !ok {
		return nil, fmt.Errorf("storage: get %s: %w", id, ErrNotFound)
	}
	return clone(d), nil
}

// Update validates and replaces the device with the given id.
func (s *Storage) Update(id string, device *models.Device) error {
	device.ID = id
	device.Normalize()
	if err := device.Validate(); err != nil {
		return fmt.Errorf("storage: update: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.devices[id]; !ok {
		return fmt.Errorf("storage: update %s: %w", id, ErrNotFound)
	}

	s.devices[id] = clone(device)
	return s.save()
}

// Delete removes the device with the given id.
func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.devices[id]; !ok {
		return fmt.Errorf("storage: delete %s: %w", id, ErrNotFound)
	}

	delete(s.devices, id)
	return s.save()
}
