package scheduler_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

func TestStore_whenMissingFile(t *testing.T) {
	// Given: a store on a nonexistent file
	st := scheduler.NewStore(filepath.Join(t.TempDir(), "schedules.json"))
	// When: listed
	// Then: empty, no error
	require.Empty(t, st.All())
}

func TestStore_whenCreateAndReload(t *testing.T) {
	// Given: a stored enabled schedule
	path := filepath.Join(t.TempDir(), "schedules.json")
	st := scheduler.NewStore(path)
	created, err := st.Create(models.Schedule{ID: "morning", DeviceID: "pc", Cron: "0 7 * * *"})
	// When: created then reloaded from disk
	// Then: persisted with enabled defaulting to true
	require.NoError(t, err)
	require.True(t, created.Enabled)
	reloaded := scheduler.NewStore(path)
	got, err := reloaded.Get("morning")
	require.NoError(t, err)
	require.Equal(t, "0 7 * * *", got.Cron)
	require.True(t, got.Enabled)
}

func TestStore_whenDuplicate(t *testing.T) {
	// Given: a store holding one schedule
	st := scheduler.NewStore(filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "s", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	// When: created again under the same id
	// Then: already-exists
	_, err = st.Create(models.Schedule{ID: "s", DeviceID: "pc", Cron: "@daily"})
	require.ErrorIs(t, err, scheduler.ErrAlreadyExists)
}

func TestStore_whenUnknownDevice(t *testing.T) {
	// Given: a create request pointing at no known device
	st := scheduler.NewStore(filepath.Join(t.TempDir(), "schedules.json"))
	// When: created with an empty device id
	// Then: missing device
	_, err := st.Create(models.Schedule{ID: "s", Cron: "@daily"})
	require.ErrorIs(t, err, models.ErrMissingDevice)
}

func TestStore_whenUpdateDelete(t *testing.T) {
	// Given: a stored schedule
	st := scheduler.NewStore(filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "s", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	// When: updated then deleted
	// Then: cron replaced, then gone with not-found on second delete
	updated, err := st.Update("s", models.Schedule{ID: "other", DeviceID: "pc", Cron: "0 8 * * *"})
	require.NoError(t, err)
	require.Equal(t, "s", updated.ID)
	require.Equal(t, "0 8 * * *", updated.Cron)
	require.NoError(t, st.Delete("s"))
	require.ErrorIs(t, st.Delete("s"), scheduler.ErrNotFound)
	_, err = st.Get("s")
	require.ErrorIs(t, err, scheduler.ErrNotFound)
}
