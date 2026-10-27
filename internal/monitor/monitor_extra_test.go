package monitor_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/monitor"
)

// Given: one unknown device on an unroutable address
// When: running Check twice
// Then: (1,1) then (1,0), and uptime tallies 0 up over 2 polls.
func Test_Monitor_checkCountsAndUptime(t *testing.T) {
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: true,
	}))
	mon := monitor.NewWithTimeout(s, time.Hour, 100*time.Millisecond)

	checked, changed := mon.Check()
	require.Equal(t, 1, checked)
	require.Equal(t, 1, changed)

	checked, changed = mon.Check()
	require.Equal(t, 1, checked)
	require.Equal(t, 0, changed)

	up, total, ok := mon.Uptime("pc1")
	require.True(t, ok)
	require.Equal(t, 0, up)
	require.Equal(t, 2, total)

	_, _, ok = mon.Uptime("ghost")
	require.False(t, ok)
}

// Given: a device flipped back up between passes
// When: running 25 checks
// Then: the transition ring caps at 20 newest-first, and pc1 flaps.
func Test_Monitor_transitionsCapAndFlapping(t *testing.T) {
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: true,
	}))
	mon := monitor.NewWithTimeout(s, time.Hour, 50*time.Millisecond)

	for i := 0; i < 21; i++ {
		dev, err := s.Get("pc1")
		require.NoError(t, err)
		dev.Status = models.StatusUp
		require.NoError(t, s.Update("pc1", dev))
		mon.Check()
	}

	got := mon.Transitions()
	require.Len(t, got, 20)
	for _, tr := range got {
		require.Equal(t, "pc1", tr.DeviceID)
		require.Equal(t, "up", tr.From)
		require.Equal(t, "down", tr.To)
		require.Greater(t, tr.At, int64(0))
	}
	require.Contains(t, mon.Flapping(), "pc1")
}

// Given: a device with a one-hour per-device interval
// When: running two immediate checks
// Then: the second pass skips it (checked 0).
func Test_Monitor_perDeviceIntervalSkips(t *testing.T) {
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: true,
		MonitorSecs: 3600,
	}))
	mon := monitor.NewWithTimeout(s, time.Hour, 50*time.Millisecond)

	checked, _ := mon.Check()
	require.Equal(t, 1, checked)
	checked, _ = mon.Check()
	require.Equal(t, 0, checked)
}

// Given: one check pass
// When: reading Status
// Then: tuning and latest pass counters are reported.
func Test_Monitor_status(t *testing.T) {
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: true,
	}))
	mon := monitor.NewWithTimeout(s, time.Hour, 50*time.Millisecond)
	mon.Check()

	interval, timeout, lastRun, lastChecked, lastChanged := mon.Status()
	require.Equal(t, time.Hour, interval)
	require.Equal(t, 50*time.Millisecond, timeout)
	require.Greater(t, lastRun, int64(0))
	require.Equal(t, 1, lastChecked)
	require.Equal(t, 1, lastChanged)
}
