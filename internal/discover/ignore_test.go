package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Given: no file on disk
// When: loading the ignore list
// Then: it starts empty without error.
func TestLoadIgnoreList_missingFile(t *testing.T) {
	il, err := LoadIgnoreList(filepath.Join(t.TempDir(), "ignored.json"))
	require.NoError(t, err)
	require.Empty(t, il.List())
	require.False(t, il.Contains("aa:bb:cc:dd:ee:01", ""))
}

// Given: a corrupt file
// When: loading the ignore list
// Then: startup fails fast.
func TestLoadIgnoreList_corruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ignored.json")
	require.NoError(t, os.WriteFile(path, []byte("{nope"), 0o600))
	_, err := LoadIgnoreList(path)
	require.Error(t, err)
}

// Given: an empty list
// When: adding addresses
// Then: MACs canonicalize to lowercase, duplicates report created=false,
// and bad input fails with explicit errors.
func TestIgnoreList_addAndContains(t *testing.T) {
	il, err := LoadIgnoreList(filepath.Join(t.TempDir(), "ignored.json"))
	require.NoError(t, err)

	created, err := il.Add("AA:BB:CC:DD:EE:01", "")
	require.NoError(t, err)
	require.True(t, created)
	require.True(t, il.Contains("aa:bb:cc:dd:ee:01", ""))
	require.True(t, il.Contains("AA:BB:CC:DD:EE:01", ""))

	created, err = il.Add("aa:bb:cc:dd:ee:01", "")
	require.NoError(t, err)
	require.False(t, created)

	created, err = il.Add("", "192.168.1.50")
	require.NoError(t, err)
	require.True(t, created)
	require.True(t, il.Contains("", "192.168.1.50"))
	require.False(t, il.Contains("", "192.168.1.51"))

	for _, tc := range []struct{ mac, ip, msg string }{
		{"not-a-mac", "", "invalid mac"},
		{"", "999.1.1.1", "invalid ip"},
		{"", "", "mac or ip is required"},
	} {
		_, err = il.Add(tc.mac, tc.ip)
		require.ErrorContains(t, err, tc.msg, "mac=%q ip=%q", tc.mac, tc.ip)
	}
}

// Given: entries on disk
// When: reloading and removing
// Then: state persists across loads and removal reports honestly.
func TestIgnoreList_persistRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ignored.json")
	il, err := LoadIgnoreList(path)
	require.NoError(t, err)
	_, err = il.Add("AA:BB:CC:DD:EE:02", "192.168.1.52")
	require.NoError(t, err)

	reloaded, err := LoadIgnoreList(path)
	require.NoError(t, err)
	require.True(t, reloaded.Contains("aa:bb:cc:dd:ee:02", ""))
	require.True(t, reloaded.Contains("", "192.168.1.52"))

	entries := reloaded.List()
	require.Len(t, entries, 2)
	require.Equal(t, "aa:bb:cc:dd:ee:02", entries[0].MAC)
	require.Equal(t, "192.168.1.52", entries[1].IP)

	removed, err := reloaded.Remove("aa:bb:cc:dd:ee:02", "")
	require.NoError(t, err)
	require.True(t, removed)
	require.False(t, reloaded.Contains("aa:bb:cc:dd:ee:02", ""))

	removed, err = reloaded.Remove("aa:bb:cc:dd:ee:02", "")
	require.NoError(t, err)
	require.False(t, removed)
}
