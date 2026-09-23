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
	// wakeCounts tallies recorded packets by trigger and result for /metrics.
	// First index: 0 = manual, 1 = schedule. Second index: 0 = ok, 1 = error.
	wakeCounts [2][2]uint64
	startedAt  time.Time
}

// New opens the history at filepath. Missing files start empty; unreadable or
// corrupt files fail startup.
func New(filepath string) (*History, error) {
	return newWithCapacity(filepath, defaultCapacity)
}

// NewWithCapacity opens the history with a custom retention bound.
func NewWithCapacity(filepath string, capacity int) (*History, error) {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return newWithCapacity(filepath, capacity)
}

func newWithCapacity(filepath string, capacity int) (*History, error) {
	h := &History{file: filepath, capacity: capacity, startedAt: time.Now()}
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
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return nil, fmt.Errorf("history: validate %s at index %d: %w", filepath, i, err)
		}
	}
	if len(entries) > capacity {
		entries = entries[:capacity]
	}
	h.entries = entries
	if err := os.Chmod(filepath, 0o600); err != nil {
		return nil, fmt.Errorf("history: chmod %s: %w", filepath, err)
	}
	return h, nil
}

// Record appends a wake event, assigns its id and timestamp, persists,
// and returns the stored copy. A save failure rolls the memory update back.
func (h *History) Record(e models.WakeEvent) (models.WakeEvent, error) {
	if err := e.Validate(); err != nil {
		return models.WakeEvent{}, fmt.Errorf("history: record: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	previous := append([]models.WakeEvent(nil), h.entries...)
	previousSeq := h.seq
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
		h.entries = previous
		h.seq = previousSeq
		return models.WakeEvent{}, err
	}
	h.wakeCounts[triggerIndex(e.Trigger)][resultIndex(e.Success)]++
	return e, nil
}

func triggerIndex(t models.Trigger) int {
	if t == models.TriggerSchedule {
		return 1
	}
	return 0
}

func resultIndex(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

// Stats snapshots wake counters, retention size, and start time.
type Stats struct {
	ManualOK, ManualErr, ScheduleOK, ScheduleErr uint64
	StartedAt                                    time.Time
	Size                                         int
}

// Stats returns a consistent snapshot of the counters and retention size.
func (h *History) Stats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return Stats{
		ManualOK:    h.wakeCounts[0][0],
		ManualErr:   h.wakeCounts[0][1],
		ScheduleOK:  h.wakeCounts[1][0],
		ScheduleErr: h.wakeCounts[1][1],
		StartedAt:   h.startedAt,
		Size:        len(h.entries),
	}
}

// List returns stored entries newest-first, optionally filtered by device.
// A non-positive limit selects the default page of 50.
func (h *History) List(deviceID string, limit int) []models.WakeEvent {
	return h.ListFiltered(Filter{DeviceID: deviceID}, limit)
}

// Filter selects history entries across device, trigger, result, and time.
// Zero values disable their dimension; Since/Before are unix seconds.
type Filter struct {
	DeviceID string
	Trigger  models.Trigger
	Success  *bool
	Since    int64
	Before   int64
}

// ListFiltered returns stored entries newest-first matching f.
// A non-positive limit selects the default page of 50.
func (h *History) ListFiltered(f Filter, limit int) []models.WakeEvent {
	if limit <= 0 {
		limit = 50
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]models.WakeEvent, 0, min(limit, len(h.entries)))
	for _, e := range h.entries {
		if f.DeviceID != "" && e.DeviceID != f.DeviceID {
			continue
		}
		if f.Trigger != "" && e.Trigger != f.Trigger {
			continue
		}
		if f.Success != nil && e.Success != *f.Success {
			continue
		}
		if f.Since != 0 && e.At < f.Since {
			continue
		}
		if f.Before != 0 && e.At >= f.Before {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// Purge drops entries recorded strictly before the unix timestamp,
// persists, and returns the removed count.
func (h *History) Purge(before int64) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	kept := h.entries[:0:0]
	purged := 0
	for _, e := range h.entries {
		if e.At < before {
			purged++
			continue
		}
		kept = append(kept, e)
	}
	if purged == 0 {
		return 0, nil
	}
	previous := h.entries
	h.entries = kept
	if err := h.save(); err != nil {
		h.entries = previous
		return 0, err
	}
	return purged, nil
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
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("history: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("history: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("history: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("history: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, h.file); err != nil {
		return fmt.Errorf("history: replace %s: %w", h.file, err)
	}
	return nil
}
