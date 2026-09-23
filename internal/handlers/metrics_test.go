package handlers

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
)

// Given: one manual wake, one scheduled wake, two devices, two schedules
// When: GET /metrics
// Then: Prometheus exposition with exact counters and gauges.
func TestMetrics_whenWakesRecorded(t *testing.T) {
	s := newTestServer(t)
	h, err := history.New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)
	s.WithHistory(h)
	require.NoError(t, s.store.Create(&models.Device{
		ID: "pc1", Name: "PC 1", MAC: "AA:BB:CC:DD:EE:01", Status: models.StatusUnknown,
	}))
	require.NoError(t, s.store.Create(&models.Device{
		ID: "pc2", Name: "PC 2", MAC: "AA:BB:CC:DD:EE:02", Status: models.StatusUp,
	}))
	schedStore := newScheduleStore(t, filepath.Join(t.TempDir(), "sched.json"))
	s.WithSchedules(schedStore, nil)
	_, err = schedStore.Create(models.Schedule{ID: "morn", DeviceID: "pc1", Cron: "0 7 * * *", Enabled: true})
	require.NoError(t, err)
	_, err = schedStore.Create(models.Schedule{ID: "night", DeviceID: "pc1", Cron: "0 23 * * *", Enabled: false})
	require.NoError(t, err)
	// Creation activates; disable one schedule to cover both gauge branches.
	_, err = schedStore.Update("night", models.Schedule{ID: "night", DeviceID: "pc1", Cron: "0 23 * * *", Enabled: false})
	require.NoError(t, err)

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
	_, err = h.Record(models.WakeEvent{
		DeviceID: "pc1", MAC: "AA:BB:CC:DD:EE:01", Trigger: models.TriggerSchedule, Success: true,
	})
	require.NoError(t, err)

	w = doRequest(s, http.MethodGet, "/metrics", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	body := w.Body.String()
	require.Contains(t, body, "hello_wake_total{trigger=\"manual\",result=\"ok\"} 1")
	require.Contains(t, body, "hello_wake_total{trigger=\"manual\",result=\"error\"} 0")
	require.Contains(t, body, "hello_wake_total{trigger=\"schedule\",result=\"ok\"} 1")
	require.Contains(t, body, "hello_wake_total{trigger=\"schedule\",result=\"error\"} 0")
	require.Contains(t, body, "hello_devices{status=\"up\"} 1")
	require.Contains(t, body, "hello_devices{status=\"down\"} 0")
	require.Contains(t, body, "hello_devices{status=\"unknown\"} 1")
	require.Contains(t, body, "hello_schedules{enabled=\"true\"} 1")
	require.Contains(t, body, "hello_schedules{enabled=\"false\"} 1")
	require.Contains(t, body, "hello_uptime_seconds")
	require.Contains(t, body, "hello_history_size 2")
}

// Given: a bare server without history or schedules
// When: GET /metrics
// Then: 200 with zero-filled series, no crash.
func TestMetrics_whenEmpty(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/metrics", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "hello_wake_total{trigger=\"manual\",result=\"ok\"} 0")
	require.Contains(t, w.Body.String(), "hello_devices{status=\"unknown\"} 0")
}
