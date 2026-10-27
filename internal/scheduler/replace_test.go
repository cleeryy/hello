package scheduler_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

// Given: one stored schedule
// When: ReplaceAll swaps in a new list
// Then: only the new list remains.
func TestStore_replaceAll(t *testing.T) {
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "s.json"))
	_, err := st.Create(models.Schedule{ID: "a", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	got, err := st.ReplaceAll([]models.Schedule{{ID: "b", DeviceID: "pc", Cron: "@hourly"}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	_, err = st.Get("b")
	require.NoError(t, err)
	_, err = st.Get("a")
	require.ErrorIs(t, err, scheduler.ErrNotFound)
}

// Given: a store capped at one schedule
// When: a second schedule arrives via Create or ReplaceAll
// Then: both fail with ErrTooMany and the store is untouched.
func TestStore_capEnforced(t *testing.T) {
	st, err := scheduler.NewStoreWithCap(filepath.Join(t.TempDir(), "s.json"), 1)
	require.NoError(t, err)
	require.Equal(t, 1, st.Cap())
	_, err = st.Create(models.Schedule{ID: "a", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	_, err = st.Create(models.Schedule{ID: "b", DeviceID: "pc", Cron: "@daily"})
	require.ErrorIs(t, err, scheduler.ErrTooMany)
	_, err = st.ReplaceAll([]models.Schedule{
		{ID: "a", DeviceID: "pc", Cron: "@daily"},
		{ID: "b", DeviceID: "pc", Cron: "@daily"},
	})
	require.ErrorIs(t, err, scheduler.ErrTooMany)
	_, err = st.Get("a")
	require.NoError(t, err)
}
