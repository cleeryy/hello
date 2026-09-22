package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// TestHistory_whenRecorded verifies entries are listed newest-first with ids.
func TestHistory_whenRecorded(t *testing.T) {
	// Given: an empty history.
	h, err := New(filepath.Join(t.TempDir(), "history.json"))
	require.NoError(t, err)

	// When: two wakes are recorded.
	first, err := h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
	require.NoError(t, err)
	second, err := h.Record(models.WakeEvent{DeviceID: "b", MAC: "00:11:22:33:44:56", Trigger: models.TriggerManual, Success: false})
	require.NoError(t, err)

	// Then: ids assigned and newest first.
	require.NotEmpty(t, first.ID)
	require.NotEmpty(t, second.ID)
	require.NotEqual(t, first.ID, second.ID)
	got := h.List("", 50)
	require.Len(t, got, 2)
	require.Equal(t, second.ID, got[0].ID)
	require.Equal(t, first.ID, got[1].ID)
}

// TestHistory_whenFiltered verifies device filter and limit.
func TestHistory_whenFiltered(t *testing.T) {
	// Given: three entries across two devices.
	h, err := New(filepath.Join(t.TempDir(), "history.json"))
	require.NoError(t, err)
	for _, id := range []string{"a", "b", "a"} {
		_, err := h.Record(models.WakeEvent{DeviceID: id, MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
		require.NoError(t, err)
	}

	// When: listing filtered to b with limit 1, then all with limit 1.
	// Then: filter applies, limit truncates newest-first.
	filtered := h.List("b", 50)
	require.Len(t, filtered, 1)
	require.Equal(t, "b", filtered[0].DeviceID)
	limited := h.List("", 1)
	require.Len(t, limited, 1)
}

// TestHistory_whenReloaded verifies persistence across restarts.
func TestHistory_whenReloaded(t *testing.T) {
	// Given: a history with one entry on disk.
	path := filepath.Join(t.TempDir(), "history.json")
	h, err := New(path)
	require.NoError(t, err)
	saved, err := h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerSchedule, Success: true})
	require.NoError(t, err)

	// When: reopened.
	reopened, err := New(path)

	// Then: the entry survives.
	require.NoError(t, err)
	got := reopened.List("", 50)
	require.Len(t, got, 1)
	require.Equal(t, saved.ID, got[0].ID)
	require.Equal(t, models.TriggerSchedule, got[0].Trigger)
}

func TestHistory_whenSaveFailsRollsBackMemory(t *testing.T) {
	dir := t.TempDir()
	h, err := New(filepath.Join(dir, "history.json"))
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err = h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})

	require.Error(t, err)
	require.Empty(t, h.List("", 50))
}

func TestHistory_persists_file_as_private(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	h, err := New(path)
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestHistory_whenOverCapacity verifies the ring bound.
func TestHistory_whenOverCapacity(t *testing.T) {
	// Given: a tiny history holding 3 entries.
	h, err := newWithCapacity(filepath.Join(t.TempDir(), "history.json"), 3)
	require.NoError(t, err)

	// When: 4 entries are recorded.
	for i := 0; i < 4; i++ {
		_, err := h.Record(models.WakeEvent{DeviceID: "a", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true})
		require.NoError(t, err)
	}

	// Then: only the newest 3 remain.
	require.Len(t, h.List("", 50), 3)
}
