package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

// New opens the registry and fails fast when an existing file is unreadable or
// corrupt. A missing file starts an empty registry.
func New(path string) (*Storage, error) {
	s := &Storage{devices: make(map[string]*models.Device), file: path}
	if err := s.Load(); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("storage: chmod %s: %w", path, err)
	}
	return s, nil
}

// Load replaces the in-memory registry with validated file content.
// A missing file means an empty registry, not an error.
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			s.devices = make(map[string]*models.Device)
			return nil
		}
		return fmt.Errorf("storage: read %s: %w", s.file, err)
	}

	var loaded []*models.Device
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("storage: decode %s: %w", s.file, err)
	}

	fresh := make(map[string]*models.Device, len(loaded))
	for _, d := range loaded {
		if d == nil {
			return fmt.Errorf("storage: decode %s: null device entry", s.file)
		}
		if err := d.Validate(); err != nil {
			return fmt.Errorf("storage: validate %s: %w", s.file, err)
		}
		if _, exists := fresh[d.ID]; exists {
			return fmt.Errorf("storage: decode %s: duplicate device id %q", s.file, d.ID)
		}
		fresh[d.ID] = clone(d)
	}
	s.devices = fresh
	return nil
}

// save writes the registry atomically (temp file + rename).
// Callers must hold the write lock.
func (s *Storage) save() error {
	devices := make([]*models.Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, clone(d))
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
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("storage: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("storage: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("storage: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.file); err != nil {
		return fmt.Errorf("storage: replace %s: %w", s.file, err)
	}
	return nil
}

func clone(d *models.Device) *models.Device {
	if d == nil {
		return nil
	}
	cpy := *d
	return &cpy
}

func cloneDevices(in map[string]*models.Device) map[string]*models.Device {
	out := make(map[string]*models.Device, len(in))
	for id, d := range in {
		out[id] = clone(d)
	}
	return out
}

// Save persists the current registry.
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// ValidateDevices normalizes and checks a full replacement batch without
// persisting anything, so restore dry-runs enforce the exact same rules.
func ValidateDevices(devices []*models.Device) ([]*models.Device, error) {
	prepared := make([]*models.Device, 0, len(devices))
	seen := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		if device == nil {
			return nil, fmt.Errorf("storage: replace: nil device")
		}
		device.Normalize()
		if err := device.Validate(); err != nil {
			return nil, fmt.Errorf("storage: replace %q: %w", device.ID, err)
		}
		if _, dup := seen[device.ID]; dup {
			return nil, fmt.Errorf("storage: replace: duplicate device id %q", device.ID)
		}
		seen[device.ID] = struct{}{}
		prepared = append(prepared, clone(device))
	}
	return prepared, nil
}

// ReplaceAll validates a full registry replacement, then performs one locked
// write. On any persistence failure the in-memory registry is restored to
// its prior state, so callers can treat restore as all-or-nothing.
func (s *Storage) ReplaceAll(devices []*models.Device) ([]*models.Device, error) {
	prepared, err := ValidateDevices(devices)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	previous := cloneDevices(s.devices)
	fresh := make(map[string]*models.Device, len(prepared))
	for _, device := range prepared {
		fresh[device.ID] = clone(device)
	}
	s.devices = fresh
	if err := s.save(); err != nil {
		s.devices = previous
		return nil, err
	}
	return prepared, nil
}

// Create validates and atomically inserts one device.
func (s *Storage) Create(device *models.Device) error {
	_, err := s.CreateMany([]*models.Device{device})
	return err
}

// CreateMany pre-validates a batch, then performs one locked write. On any
// persistence failure the in-memory registry is restored to its prior state.
func (s *Storage) CreateMany(devices []*models.Device) ([]*models.Device, error) {
	prepared := make([]*models.Device, 0, len(devices))
	seen := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		if device == nil {
			return nil, fmt.Errorf("storage: create: nil device")
		}
		device.Normalize()
		if err := device.Validate(); err != nil {
			return nil, fmt.Errorf("storage: create: %w", err)
		}
		if _, duplicate := seen[device.ID]; duplicate {
			return nil, fmt.Errorf("storage: create: duplicate device id %q", device.ID)
		}
		seen[device.ID] = struct{}{}
		prepared = append(prepared, clone(device))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, device := range prepared {
		if _, exists := s.devices[device.ID]; exists {
			return nil, fmt.Errorf("storage: create %s: %w", device.ID, ErrAlreadyExists)
		}
	}
	previous := cloneDevices(s.devices)
	for _, device := range prepared {
		s.devices[device.ID] = clone(device)
	}
	if err := s.save(); err != nil {
		s.devices = previous
		return nil, err
	}

	created := make([]*models.Device, 0, len(prepared))
	for _, device := range prepared {
		created = append(created, clone(device))
	}
	return created, nil
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

// LookupMAC returns the id of the device holding mac, comparing canonical
// forms so colon- and hyphen-separated spellings match. It reports false
// when no registered device holds the address.
func (s *Storage) LookupMAC(mac string) (string, bool) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return "", false
	}
	want := hw.String()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, d := range s.devices {
		stored, err := net.ParseMAC(d.MAC)
		if err != nil {
			continue
		}
		if stored.String() == want {
			return id, true
		}
	}
	return "", false
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

// Update validates and atomically replaces the device with the given id.
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
	previous := cloneDevices(s.devices)
	s.devices[id] = clone(device)
	if err := s.save(); err != nil {
		s.devices = previous
		return err
	}
	return nil
}

// Delete atomically removes the device with the given id.
func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.devices[id]; !ok {
		return fmt.Errorf("storage: delete %s: %w", id, ErrNotFound)
	}
	previous := cloneDevices(s.devices)
	delete(s.devices, id)
	if err := s.save(); err != nil {
		s.devices = previous
		return err
	}
	return nil
}

// RecordWake bumps the wake counters of a device in one locked write, so a
// concurrent monitor status update cannot silently drop the increment.
func (s *Storage) RecordWake(id string, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.devices[id]
	if !ok {
		return fmt.Errorf("storage: record wake %s: %w", id, ErrNotFound)
	}
	previous := cloneDevices(s.devices)
	d.WakeCount++
	d.LastWakeAt = at
	if err := s.save(); err != nil {
		s.devices = previous
		return err
	}
	return nil
}
