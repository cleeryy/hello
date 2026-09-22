package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

func newScheduleStore(t *testing.T, path string) *scheduler.Store {
	t.Helper()
	store, err := scheduler.NewStore(path)
	require.NoError(t, err)
	return store
}

func TestStore_whenMissingFile(t *testing.T) {
	// Given: a store on a nonexistent file
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	// When: listed
	// Then: empty, no error
	require.Empty(t, st.All())
}

func TestStore_whenCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedules.json")
	require.NoError(t, os.WriteFile(path, []byte("{oops"), 0o644))

	_, err := scheduler.NewStore(path)

	require.Error(t, err)
}

func TestStore_whenSaveFailsRollsBackMemory(t *testing.T) {
	dir := t.TempDir()
	st := newScheduleStore(t, filepath.Join(dir, "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "one", DeviceID: "pc", Cron: "0 7 * * *"})
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err = st.Create(models.Schedule{ID: "two", DeviceID: "pc", Cron: "0 8 * * *"})

	require.Error(t, err)
	_, err = st.Get("two")
	require.ErrorIs(t, err, scheduler.ErrNotFound)
}

func TestStore_persists_file_as_private(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedules.json")
	st := newScheduleStore(t, path)
	_, err := st.Create(models.Schedule{ID: "one", DeviceID: "pc", Cron: "0 7 * * *"})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStore_whenCreateAndReload(t *testing.T) {
	// Given: a stored enabled schedule
	path := filepath.Join(t.TempDir(), "schedules.json")
	st := newScheduleStore(t, path)
	created, err := st.Create(models.Schedule{ID: "morning", DeviceID: "pc", Cron: "0 7 * * *"})
	// When: created then reloaded from disk
	// Then: persisted with enabled defaulting to true
	require.NoError(t, err)
	require.True(t, created.Enabled)
	reloaded := newScheduleStore(t, path)
	got, err := reloaded.Get("morning")
	require.NoError(t, err)
	require.Equal(t, "0 7 * * *", got.Cron)
	require.True(t, got.Enabled)
}

func TestStore_whenDuplicate(t *testing.T) {
	// Given: a store holding one schedule
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "s", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	// When: created again under the same id
	// Then: already-exists
	_, err = st.Create(models.Schedule{ID: "s", DeviceID: "pc", Cron: "@daily"})
	require.ErrorIs(t, err, scheduler.ErrAlreadyExists)
}

func TestStore_whenUnknownDevice(t *testing.T) {
	// Given: a create request pointing at no known device
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	// When: created with an empty device id
	// Then: missing device
	_, err := st.Create(models.Schedule{ID: "s", Cron: "@daily"})
	require.ErrorIs(t, err, models.ErrMissingDevice)
}

func TestStore_whenUpdateDelete(t *testing.T) {
	// Given: a stored schedule
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
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
