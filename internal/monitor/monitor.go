package monitor

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/storage"
)

// pingHost is a seam for deterministic tests.
var pingHost = ping.PingHost

const (
	// maxTransitions bounds the in-memory status-change ring.
	maxTransitions = 20
	// flapWindow and flapThreshold define flapping: at least flapThreshold
	// transitions for one device inside flapWindow.
	flapWindow    = 5 * time.Minute
	flapThreshold = 4
)

// Transition is one observed reachability change.
type Transition struct {
	DeviceID string `json:"device_id"`
	From     string `json:"from"`
	To       string `json:"to"`
	At       int64  `json:"at"`
}

type uptimeStat struct {
	up    int
	total int
}

// Monitor polls ping-enabled devices and persists status changes.
type Monitor struct {
	store    *storage.Storage
	interval time.Duration
	timeout  time.Duration

	// OnStatusChange is invoked synchronously after a persisted status change.
	OnStatusChange func(models.Device)

	mu          sync.Mutex
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	transitions []Transition
	uptime      map[string]*uptimeStat
	lastCheck   map[string]time.Time
	lastRun     time.Time
	lastChecked int
	lastChanged int
	startedAt   time.Time
}

// New returns a Monitor polling every interval with a 2s per-host timeout.
func New(store *storage.Storage, interval time.Duration) *Monitor {
	return NewWithTimeout(store, interval, 2*time.Second)
}

// NewWithTimeout returns a Monitor with an explicit per-host ping timeout.
func NewWithTimeout(store *storage.Storage, interval, timeout time.Duration) *Monitor {
	return &Monitor{
		store:     store,
		interval:  interval,
		timeout:   timeout,
		stopCh:    make(chan struct{}),
		uptime:    map[string]*uptimeStat{},
		lastCheck: map[string]time.Time{},
		startedAt: time.Now(),
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
	m.poll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

func (m *Monitor) poll() {
	_, _, fired := m.checkDevices("")
	for _, dev := range fired {
		if m.OnStatusChange != nil {
			m.OnStatusChange(dev)
		}
	}
}

// Check runs one polling pass on demand and reports checked/changed counts.
func (m *Monitor) Check() (checked, changed int) {
	return m.check("")
}

// CheckOne probes a single device on demand. An unknown id reports zero
// checks; the periodic loop keeps polling independently.
func (m *Monitor) CheckOne(id string) (checked, changed int) {
	return m.check(id)
}

func (m *Monitor) check(onlyID string) (checked, changed int) {
	checked, changed, fired := m.checkDevices(onlyID)
	for _, dev := range fired {
		if m.OnStatusChange != nil {
			m.OnStatusChange(dev)
		}
	}
	return checked, changed
}

func (m *Monitor) checkDevices(onlyID string) (checked, changed int, fired []models.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, d := range m.store.GetAll() {
		if onlyID != "" && d.ID != onlyID {
			continue
		}
		if d.IP == "" || !d.PingEnabled {
			continue
		}
		// Per-device interval (F-61): skip devices polled too recently.
		if d.MonitorSecs > 0 {
			if last, ok := m.lastCheck[d.ID]; ok && now.Sub(last) < time.Duration(d.MonitorSecs)*time.Second {
				continue
			}
		}
		m.lastCheck[d.ID] = now
		next := models.StatusDown
		if pingHost(d.IP, m.timeout) {
			next = models.StatusUp
		}
		checked++
		st := m.uptime[d.ID]
		if st == nil {
			st = &uptimeStat{}
			m.uptime[d.ID] = st
		}
		st.total++
		if next == models.StatusUp {
			st.up++
		}
		if d.Status == next {
			continue
		}
		updated := *d
		updated.Status = next
		if next == models.StatusUp {
			updated.LastSeen = now.Unix()
		}
		if err := m.store.Update(d.ID, &updated); err != nil {
			slog.Error("device status persist failed",
				slog.String("id", d.ID), slog.Any("err", err))
			continue
		}
		changed++
		m.pushTransition(Transition{
			DeviceID: d.ID,
			From:     string(d.Status),
			To:       string(next),
			At:       now.Unix(),
		})
		slog.Info("device status changed",
			slog.String("id", d.ID), slog.String("status", string(next)))
		fired = append(fired, updated)
	}
	m.lastRun = now
	m.lastChecked = checked
	m.lastChanged = changed
	return checked, changed, fired
}

func (m *Monitor) pushTransition(t Transition) {
	m.transitions = append([]Transition{t}, m.transitions...)
	if len(m.transitions) > maxTransitions {
		m.transitions = m.transitions[:maxTransitions]
	}
}

// Transitions returns recent status changes, newest first.
func (m *Monitor) Transitions() []Transition {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Transition, len(m.transitions))
	copy(out, m.transitions)
	return out
}

// DeviceUptime is the honest per-device poll summary backing
// GET /devices/:id/uptime (F-57): raw poll counters plus the latest
// recorded transition and the monitor start time. ok is false when the
// device was never polled.
type DeviceUptime struct {
	Up         int
	Total      int
	LastChange int64
	Since      int64
}

// Detail reports the per-device poll summary; ok is false when the device
// was never polled.
func (m *Monitor) Detail(deviceID string) (DeviceUptime, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, found := m.uptime[deviceID]
	if !found || st.total == 0 {
		return DeviceUptime{}, false
	}
	out := DeviceUptime{Up: st.up, Total: st.total, Since: m.startedAt.Unix()}
	for _, t := range m.transitions {
		if t.DeviceID == deviceID {
			out.LastChange = t.At
			break
		}
	}
	return out, true
}

// Uptime reports polls answered up over polls performed since start.
// ok is false when the device was never polled.
func (m *Monitor) Uptime(deviceID string) (up, total int, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, found := m.uptime[deviceID]
	if !found || st.total == 0 {
		return 0, 0, false
	}
	return st.up, st.total, true
}

// Flapping lists devices with at least flapThreshold transitions inside
// the trailing flapWindow, sorted by id.
func (m *Monitor) Flapping() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-flapWindow).Unix()
	counts := map[string]int{}
	for _, t := range m.transitions {
		if t.At >= cutoff {
			counts[t.DeviceID]++
		}
	}
	var out []string
	for id, n := range counts {
		if n >= flapThreshold {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Since reports the monitor start time as unix seconds, backing the
// per-device uptime summary for devices never polled yet.
func (m *Monitor) Since() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startedAt.Unix()
}

// Status snapshots the monitor loop figures for /monitor and /health.
func (m *Monitor) Status() (interval, timeout time.Duration, lastRun int64, lastChecked, lastChanged int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var at int64
	if !m.lastRun.IsZero() {
		at = m.lastRun.Unix()
	}
	return m.interval, m.timeout, at, m.lastChecked, m.lastChanged
}
