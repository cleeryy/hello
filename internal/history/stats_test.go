package history

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Recorded wakes feed the /metrics counters and the retention size.
func TestStats_whenWakesRecorded(t *testing.T) {
	h, err := New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)

	record := func(trigger models.Trigger, ok bool) {
		t.Helper()
		_, err := h.Record(models.WakeEvent{
			DeviceID: "pc1", MAC: "AA:BB:CC:DD:EE:01", Trigger: trigger, Success: ok,
		})
		require.NoError(t, err)
	}
	record(models.TriggerManual, true)
	record(models.TriggerManual, false)
	record(models.TriggerSchedule, true)
	record(models.TriggerSchedule, false)
	record(models.TriggerSchedule, true)

	st := h.Stats()
	require.Equal(t, uint64(1), st.ManualOK)
	require.Equal(t, uint64(1), st.ManualErr)
	require.Equal(t, uint64(2), st.ScheduleOK)
	require.Equal(t, uint64(1), st.ScheduleErr)
	require.Equal(t, 5, st.Size)
	require.False(t, st.StartedAt.IsZero())
	require.WithinDuration(t, time.Now(), st.StartedAt, time.Minute)
}
