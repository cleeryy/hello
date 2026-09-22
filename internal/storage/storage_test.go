package storage_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/storage"
)

func newStorage(t *testing.T, path string) *storage.Storage {
	t.Helper()
	store, err := storage.New(path)
	require.NoError(t, err)
	return store
}

func testDevice(id string) *models.Device {
	return &models.Device{
		ID: id, Name: "PC " + id, MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown,
	}
}

func Test_Storage_Create_and_Get_roundtrip(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))

	// When
	require.NoError(t, s.Create(testDevice("pc1")))
	got, err := s.Get("pc1")

	// Then
	require.NoError(t, err)
	assert.Equal(t, "pc1", got.ID)
	assert.Equal(t, models.StatusUnknown, got.Status)
}

func Test_Storage_Create_rejects_duplicate(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	err := s.Create(testDevice("pc1"))

	// Then
	require.ErrorIs(t, err, storage.ErrAlreadyExists)
}

func Test_Storage_Create_rejects_invalid_device(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	bad := testDevice("pc1")
	bad.MAC = "bogus"

	// When
	err := s.Create(bad)

	// Then
	require.ErrorIs(t, err, models.ErrInvalidMAC)
}

func Test_Storage_Get_returns_copy(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	got, err := s.Get("pc1")
	require.NoError(t, err)
	got.Name = "mutated"
	fresh, err := s.Get("pc1")

	// Then
	require.NoError(t, err)
	assert.Equal(t, "PC pc1", fresh.Name)
}

func Test_Storage_Get_missing_returns_not_found(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))

	// When
	_, err := s.Get("nope")

	// Then
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func Test_Storage_Update_and_Delete(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	upd := testDevice("other")
	upd.Name = "Renamed"
	require.NoError(t, s.Update("pc1", upd))

	// Then
	got, err := s.Get("pc1")
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.Equal(t, "pc1", got.ID)

	// When
	require.NoError(t, s.Delete("pc1"))

	// Then
	_, err = s.Get("pc1")
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.ErrorIs(t, s.Delete("pc1"), storage.ErrNotFound)
	require.ErrorIs(t, s.Update("pc1", testDevice("pc1")), storage.ErrNotFound)
}

func Test_Storage_persists_across_reload(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "devices.json")
	s := newStorage(t, path)
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	reloaded := newStorage(t, path)
	got, err := reloaded.Get("pc1")

	// Then
	require.NoError(t, err)
	assert.Equal(t, "PC pc1", got.Name)
}

func Test_Storage_Load_rejects_corrupt_file(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "devices.json")
	require.NoError(t, os.WriteFile(path, []byte("{oops"), 0o644))

	// When
	_, err := storage.New(path)

	// Then
	require.Error(t, err)
}

func Test_Storage_CreateMany_is_atomic_on_save_failure(t *testing.T) {
	// Given
	dir := t.TempDir()
	s := newStorage(t, filepath.Join(dir, "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	// When
	created, err := s.CreateMany([]*models.Device{testDevice("pc2")})

	// Then: memory and disk remain unchanged.
	require.Error(t, err)
	require.Nil(t, created)
	require.Len(t, s.GetAll(), 1)
	_, err = s.Get("pc2")
	require.ErrorIs(t, err, storage.ErrNotFound)
	reloaded := newStorage(t, filepath.Join(dir, "devices.json"))
	require.Len(t, reloaded.GetAll(), 1)
}

func Test_Storage_persists_files_as_private(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	s := newStorage(t, path)
	require.NoError(t, s.Create(testDevice("pc1")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func Test_Storage_concurrent_access(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Get("pc1")
			_ = s.GetAll()
		}()
	}
	wg.Wait()

	// Then
	assert.Len(t, s.GetAll(), 1)
}

func Test_Storage_LookupMAC_resolves_id(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	id, ok := s.LookupMAC("00-11-22-33-44-55")
	_, missing := s.LookupMAC("AA:BB:CC:DD:EE:FF")

	// Then
	assert.True(t, ok)
	assert.Equal(t, "pc1", id)
	assert.False(t, missing)
}
