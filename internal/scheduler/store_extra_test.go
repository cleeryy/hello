package scheduler_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

// Given: a stored schedule
// When: marked fired with success
// Then: run time and ok result are recorded, the row stays enabled.
func TestStore_whenMarkFiredSuccess(t *testing.T) {
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "a", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)

	st.MarkFired("a", true, "")
	got, err := st.Get("a")
	require.NoError(t, err)
	require.Greater(t, got.LastRunAt, int64(0))
	require.Equal(t, "ok", got.LastResult)
	require.True(t, got.Enabled)
}

// Given: a one-shot schedule
// When: marked fired
// Then: it is consumed (disabled and detached) after recording the run.
func TestStore_whenMarkFiredConsumesOnce(t *testing.T) {
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{
		ID: "o", DeviceID: "pc", Cron: "@daily",
		Once: true, OnceAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	st.MarkFired("o", true, "")
	got, err := st.Get("o")
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.False(t, got.Once)
	require.Equal(t, "ok", got.LastResult)
}

// Given: a failed fire with a long detail
// When: marked fired
// Then: the error result is stored truncated to 200 characters.
func TestStore_whenMarkFiredError(t *testing.T) {
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "a", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)

	st.MarkFired("a", false, strings.Repeat("z", 300))
	got, err := st.Get("a")
	require.NoError(t, err)
	require.Equal(t, "error: "+strings.Repeat("z", 200), got.LastResult)

	// Unknown ids are ignored, never fatal.
	st.MarkFired("ghost", true, "")
}

// Given: one-shot and classic schedules
// When: stale one-shots are disabled
// Then: only past-due enabled one-shots flip.
func TestStore_whenDisableStaleOnce(t *testing.T) {
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	now := time.Now().Unix()
	_, err := st.Create(models.Schedule{
		ID: "o", DeviceID: "pc", Cron: "@daily",
		Once: true, OnceAt: now + 3600,
	})
	require.NoError(t, err)
	_, err = st.Create(models.Schedule{ID: "c", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)

	require.Equal(t, 0, st.DisableStaleOnce(now))
	require.Equal(t, 1, st.DisableStaleOnce(now+7200))

	got, err := st.Get("o")
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.False(t, got.Once)
	classic, err := st.Get("c")
	require.NoError(t, err)
	require.True(t, classic.Enabled)
}

// Given: a schedule quota of one
// When: a second schedule is created
// Then: ErrTooMany wraps the quota message.
func TestStore_whenQuotaReached(t *testing.T) {
	st, err := scheduler.NewStoreWithCap(filepath.Join(t.TempDir(), "schedules.json"), 1)
	require.NoError(t, err)
	_, err = st.Create(models.Schedule{ID: "s1", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	_, err = st.Create(models.Schedule{ID: "s2", DeviceID: "pc", Cron: "@daily"})
	require.ErrorIs(t, err, scheduler.ErrTooMany)
}
