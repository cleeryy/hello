package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/cleeryy/hello/internal/models"
)

type Storage struct {
	devices map[string]*models.Device
	mu      sync.RWMutex
	file    string
}

func New(filepath string) *Storage {
	s := &Storage{
		devices: make(map[string]*models.Device),
		file:    filepath,
	}
	s.Load()
	return s
}

func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.file)
	if err != nil {
		return nil
	}

	var devices []*models.Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return err
	}

	for _, device := range devices {
		s.devices[device.ID] = device
	}

	return nil
}

// saveInternal - Version sans lock pour usage interne
func (s *Storage) saveInternal() error {
	devices := make([]*models.Device, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}

	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.file, data, 0644)
}

// Save - Version publique avec lock
func (s *Storage) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveInternal()
}

func (s *Storage) Create(device *models.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[device.ID]; exists {
		return fmt.Errorf("device with id %s already exists", device.ID)
	}

	s.devices[device.ID] = device
	return s.saveInternal()  // ✅ Appelle la version interne sans lock
}

func (s *Storage) GetAll() []*models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*models.Device, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}

	return devices
}

func (s *Storage) Get(id string) (*models.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[id]
	if !exists {
		return nil, fmt.Errorf("device with id %s not found", id)
	}

	return device, nil
}

func (s *Storage) Update(id string, device *models.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[id]; !exists {
		return fmt.Errorf("device with id %s not found", id)
	}

	device.ID = id
	s.devices[id] = device
	return s.saveInternal()  // ✅ Version interne
}

func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[id]; !exists {
		return fmt.Errorf("device with id %s not found", id)
	}

	delete(s.devices, id)
	return s.saveInternal()  // ✅ Version interne
}

