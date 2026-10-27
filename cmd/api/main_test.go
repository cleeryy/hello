package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/handlers"
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

// Given: the --version flag
// When: the CLI dispatch runs
// Then: it prints the wired build version and claims the invocation.
func TestVersionFlagPrintsBuildVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		out := captureStdout(t, func() {
			handled, err := runCommand(args)
			require.True(t, handled)
			require.NoError(t, err)
		})
		require.Equal(t, handlers.Version, strings.TrimSpace(out))
	}
}

// Given: no subcommand or an unknown one
// When: the CLI dispatch runs
// Then: it declines so the server boots instead.
func TestRunCommandPassesThroughToServe(t *testing.T) {
	handled, err := runCommand([]string{})
	require.False(t, handled)
	require.NoError(t, err)
	handled, err = runCommand([]string{"serve-now"})
	require.False(t, handled)
	require.NoError(t, err)
}
