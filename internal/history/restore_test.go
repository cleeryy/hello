package history

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: a history with two entries
// When: Restore swaps in one entry with a kept id
// Then: only the new entry remains, id kept, counters untouched.
func TestHistory_restore(t *testing.T) {
	h, err := New(filepath.Join(t.TempDir(), "h.json"))
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "b", MAC: "00:11:22:33:44:66", Trigger: models.TriggerManual, Success: true})
	require.NoError(t, err)
	err = h.Restore([]models.WakeEvent{
		{ID: "keep-1", DeviceID: "c", MAC: "AA:BB:CC:DD:EE:01", Trigger: models.TriggerSchedule, Success: false, At: 123},
	})
	require.NoError(t, err)
	got := h.List("", 50)
	require.Len(t, got, 1)
	require.Equal(t, "keep-1", got[0].ID)
	require.Equal(t, int64(123), got[0].At)
	require.Equal(t, uint64(0), h.Stats().ScheduleErr)
}

// Given: a history with one entry
// When: Restore carries an invalid event
// Then: it fails and the old entry is untouched.
func TestHistory_restoreRejectsBad(t *testing.T) {
	h, err := New(filepath.Join(t.TempDir(), "h.json"))
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
	require.NoError(t, err)
	err = h.Restore([]models.WakeEvent{{DeviceID: "", MAC: "nope", Trigger: "bogus"}})
	require.Error(t, err)
	require.Len(t, h.List("", 50), 1)
}
