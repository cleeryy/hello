package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cleeryy/hello/internal/models"
)

// defaultCapacity bounds the ring buffer and the persisted file.
const defaultCapacity = 200

// History is an append-only, size-bounded log of sent magic packets,
// persisted to disk and safe for concurrent use. Newest entries come first.
type History struct {
	mu       sync.RWMutex
	entries  []models.WakeEvent
	file     string
	capacity int
	seq      uint64
}

// New opens the history at filepath, loading existing entries when present.
// A missing file means an empty history; a corrupt one is an error.
func New(filepath string) (*History, error) {
	return newWithCapacity(filepath, defaultCapacity)
}

func newWithCapacity(filepath string, capacity int) (*History, error) {
	h := &History{file: filepath, capacity: capacity}
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return h, nil
		}
		return nil, fmt.Errorf("history: read %s: %w", filepath, err)
	}
	var entries []models.WakeEvent
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("history: decode %s: %w", filepath, err)
	}
	if len(entries) > capacity {
		entries = entries[:capacity]
	}
	h.entries = entries
	return h, nil
}

// Record appends a wake event, assigns its id and timestamp, persists,
// and returns the stored copy.
func (h *History) Record(e models.WakeEvent) (models.WakeEvent, error) {
	if err := e.Validate(); err != nil {
		return models.WakeEvent{}, fmt.Errorf("history: record: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.seq++
	e.ID = fmt.Sprintf("h-%d-%d", time.Now().UnixNano(), h.seq)
	if e.At == 0 {
		e.At = time.Now().Unix()
	}
	h.entries = append([]models.WakeEvent{e}, h.entries...)
	if len(h.entries) > h.capacity {
		h.entries = h.entries[:h.capacity]
	}
	if err := h.save(); err != nil {
		return models.WakeEvent{}, err
	}
	return e, nil
}

// List returns stored entries newest-first, optionally filtered by device.
// A non-positive limit selects the default page of 50.
func (h *History) List(deviceID string, limit int) []models.WakeEvent {
	if limit <= 0 {
		limit = 50
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]models.WakeEvent, 0, min(limit, len(h.entries)))
	for _, e := range h.entries {
		if deviceID != "" && e.DeviceID != deviceID {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// save writes the log atomically (temp file + rename).
// Callers must hold the write lock.
func (h *History) save() error {
	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("history: encode: %w", err)
	}

	dir := filepath.Dir(h.file)
	tmp, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return fmt.Errorf("history: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("history: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("history: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("history: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, h.file); err != nil {
		return fmt.Errorf("history: replace %s: %w", h.file, err)
	}
	return nil
}
