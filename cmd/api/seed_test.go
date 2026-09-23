package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/storage"
)

// Given: a JSON array source file
// When: seedDevices runs against an empty registry
// Then: all devices are imported, reseeding aborts on existing ids.
func TestSeedDevices(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	require.NoError(t, os.WriteFile(src, []byte(`[
{"id":"pc1","name":"PC 1","mac":"00:11:22:33:44:01","tags":["lab"],"notes":"n"},
{"id":"pc2","name":"PC 2","mac":"00:11:22:33:44:02"}
]`), 0o600))
	registry := filepath.Join(dir, "devices.json")

	count, err := seedDevices(registry, src)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	store, err := storage.New(registry)
	require.NoError(t, err)
	require.Len(t, store.GetAll(), 2)
	got, err := store.Get("pc1")
	require.NoError(t, err)
	require.Equal(t, []string{"lab"}, got.Tags)
	require.Equal(t, "n", got.Notes)

	// Reseeding aborts: existing ids are never overwritten.
	_, err = seedDevices(registry, src)
	require.Error(t, err)
	require.Len(t, store.GetAll(), 2)

	// Missing source and invalid JSON fail loudly.
	_, err = seedDevices(registry, filepath.Join(dir, "nope.json"))
	require.Error(t, err)
	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte(`[{"id":"x"}]`), 0o600))
	_, err = seedDevices(registry, bad)
	require.Error(t, err)
}
