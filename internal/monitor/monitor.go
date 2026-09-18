package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/storage"
)

// pingHost is a seam for deterministic tests.
var pingHost = ping.PingHost

// Monitor polls ping-enabled devices and persists status changes.
type Monitor struct {
	store    *storage.Storage
	interval time.Duration
	timeout  time.Duration

	// OnStatusChange is invoked synchronously after a persisted status change.
	OnStatusChange func(models.Device)

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// New returns a Monitor polling every interval with a 2s per-host timeout.
func New(store *storage.Storage, interval time.Duration) *Monitor {
	return &Monitor{
		store:    store,
		interval: interval,
		timeout:  2 * time.Second,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the polling loop until ctx is done or Stop is called.
func (m *Monitor) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.run(ctx)
	slog.Info("device monitor started")
}

// Stop halts the loop. It is idempotent.
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.wg.Wait()
	slog.Info("device monitor stopped")
}

func (m *Monitor) run(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	m.checkDevices()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkDevices()
		}
	}
}

func (m *Monitor) checkDevices() {
	for _, d := range m.store.GetAll() {
		if d.IP == "" || !d.PingEnabled {
			continue
		}
		next := models.StatusDown
		if pingHost(d.IP, m.timeout) {
			next = models.StatusUp
		}
		if d.Status == next {
			continue
		}
		updated := *d
		updated.Status = next
		if next == models.StatusUp {
			updated.LastSeen = time.Now().Unix()
		}
		if err := m.store.Update(d.ID, &updated); err != nil {
			slog.Error("device status persist failed",
				slog.String("id", d.ID), slog.Any("err", err))
			continue
		}
		slog.Info("device status changed",
			slog.String("id", d.ID), slog.String("status", string(next)))
		if m.OnStatusChange != nil {
			m.OnStatusChange(updated)
		}
	}
}
