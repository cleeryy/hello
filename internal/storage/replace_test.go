package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/storage"
)

// Given: one stored device
// When: ReplaceAll swaps in a new list
// Then: only the new list remains.
func TestStorage_replaceAll(t *testing.T) {
	s, err := storage.New(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)
	err = s.Create(&models.Device{ID: "a", Name: "A", MAC: "00:11:22:33:44:55"})
	require.NoError(t, err)
	got, err := s.ReplaceAll([]*models.Device{{ID: "b", Name: "B", MAC: "AA:BB:CC:DD:EE:01"}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	_, err = s.Get("b")
	require.NoError(t, err)
	_, err = s.Get("a")
	require.ErrorIs(t, err, storage.ErrNotFound)
}

// Given: one stored device
// When: ReplaceAll carries an invalid device
// Then: it fails and the old list is untouched.
func TestStorage_replaceAllRollsBack(t *testing.T) {
	s, err := storage.New(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)
	err = s.Create(&models.Device{ID: "a", Name: "A", MAC: "00:11:22:33:44:55"})
	require.NoError(t, err)
	_, err = s.ReplaceAll([]*models.Device{{ID: "bad", Name: "Bad", MAC: "not-a-mac"}})
	require.Error(t, err)
	_, err = s.Get("a")
	require.NoError(t, err)
}
