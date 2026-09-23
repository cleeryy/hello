package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")
}

// Given: valid capacity and cooldown knobs
// When: loaded
// Then: the values are honored.
func Test_LoadConfig_honors_capacity_knobs(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("HISTORY_CAP", "500")
	t.Setenv("SCHEDULES_CAP", "50")
	t.Setenv("WAKE_COOLDOWN_SEC", "60")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 500, cfg.HistoryCap)
	require.Equal(t, 50, cfg.SchedulesCap)
	require.Equal(t, 60, cfg.WakeCooldownSec)
}

// Given: missing, unparsable, or out-of-range knobs
// When: loaded
// Then: safe defaults apply.
func Test_LoadConfig_falls_back_on_bad_knobs(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("HISTORY_CAP", "5")
	t.Setenv("SCHEDULES_CAP", "0")
	t.Setenv("WAKE_COOLDOWN_SEC", "9999")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 200, cfg.HistoryCap)
	require.Equal(t, 100, cfg.SchedulesCap)
	require.Equal(t, 0, cfg.WakeCooldownSec)

	t.Setenv("HISTORY_CAP", "abc")
	t.Setenv("WAKE_COOLDOWN_SEC", "-1")
	cfg, err = config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 200, cfg.HistoryCap)
	require.Equal(t, 0, cfg.WakeCooldownSec)
}
