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

func testDevice(id string) *models.Device {
	return &models.Device{
		ID: id, Name: "PC " + id, MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown,
	}
}

func Test_Storage_Create_and_Get_roundtrip(t *testing.T) {
	// Given
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))

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
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	err := s.Create(testDevice("pc1"))

	// Then
	require.ErrorIs(t, err, storage.ErrAlreadyExists)
}

func Test_Storage_Create_rejects_invalid_device(t *testing.T) {
	// Given
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
	bad := testDevice("pc1")
	bad.MAC = "bogus"

	// When
	err := s.Create(bad)

	// Then
	require.ErrorIs(t, err, models.ErrInvalidMAC)
}

func Test_Storage_Get_returns_copy(t *testing.T) {
	// Given
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
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
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))

	// When
	_, err := s.Get("nope")

	// Then
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func Test_Storage_Update_and_Delete(t *testing.T) {
	// Given
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
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
	s := storage.New(path)
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	reloaded := storage.New(path)
	got, err := reloaded.Get("pc1")

	// Then
	require.NoError(t, err)
	assert.Equal(t, "PC pc1", got.Name)
}

func Test_Storage_Load_rejects_corrupt_file(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "devices.json")
	require.NoError(t, os.WriteFile(path, []byte("{oops"), 0o644))
	s := storage.New(path)

	// When
	err := s.Load()

	// Then
	require.Error(t, err)
}

func Test_Storage_concurrent_access(t *testing.T) {
	// Given
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
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
	s := storage.New(filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(testDevice("pc1")))

	// When
	id, ok := s.LookupMAC("00-11-22-33-44-55")
	_, missing := s.LookupMAC("AA:BB:CC:DD:EE:FF")

	// Then
	assert.True(t, ok)
	assert.Equal(t, "pc1", id)
	assert.False(t, missing)
}
