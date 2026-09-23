package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/cleeryy/hello/internal/models"
)

// Scheduler fires enabled schedules through fire, resolving each target
// with lookup. Unknown devices are skipped, never fatal. fire reports
// whether the packet left and a short detail for the run record.
type Scheduler struct {
	mu      sync.Mutex
	store   *Store
	lookup  func(deviceID string) (models.Device, error)
	fire    func(sched models.Schedule, dev models.Device) (bool, string)
	cron    *cron.Cron
	running bool
}

// New returns a Scheduler; call Reload or Start before expecting fires.
func New(store *Store, lookup func(string) (models.Device, error), fire func(models.Schedule, models.Device) (bool, string)) *Scheduler {
	return &Scheduler{store: store, lookup: lookup, fire: fire, cron: cron.New()}
}

// Reload rebuilds cron entries from the store, keeping enabled ones only.
// Stale one-shot schedules (fire time passed while down) are switched off
// first so they never fire a year late. The clock keeps its running state:
// hot reload, no tick lost.
func (s *Scheduler) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if disabled := s.store.DisableStaleOnce(time.Now().Unix()); disabled > 0 {
		slog.Info("disabled stale one-shot schedules", slog.Int("count", disabled))
	}
	s.cron.Stop()
	s.cron = cron.New()
	for _, item := range s.store.All() {
		if !item.Enabled {
			continue
		}
		sched := item
		if _, err := s.cron.AddFunc(sched.Cron, func() { s.run(sched) }); err != nil {
			slog.Warn("schedule skipped", slog.String("id", sched.ID), slog.Any("err", err))
		}
	}
	if s.running {
		s.cron.Start()
	}
}

// Entries counts active cron entries.
func (s *Scheduler) Entries() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cron.Entries())
}

// Start loads schedules and blocks until ctx ends, then stops the clock.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	s.Reload()
	<-ctx.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.cron.Stop()
}

func (s *Scheduler) run(sched models.Schedule) {
	dev, err := s.lookup(sched.DeviceID)
	if err != nil {
		slog.Warn("schedule target gone", slog.String("id", sched.ID), slog.Any("err", err))
		return
	}
	ok, detail := s.fire(sched, dev)
	s.store.MarkFired(sched.ID, ok, detail)
}
