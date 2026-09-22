package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Given: an invalid DEFAULT_MAC
// When: run() boots
// Then: it fails fast before binding anything.
func TestRunRejectsInvalidConfig(t *testing.T) {
	t.Setenv("DEFAULT_MAC", "not-a-mac")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")

	require.Error(t, run())
}

// Given: no DEFAULT_MAC at all
// When: run() boots
// Then: it fails fast with a config error.
func TestRunRejectsMissingConfig(t *testing.T) {
	t.Setenv("DEFAULT_MAC", "")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")

	require.Error(t, run())
}
