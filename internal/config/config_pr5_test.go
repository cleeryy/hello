package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
)

// Given: no discovery or ping tuning in the environment
// When: loaded
// Then: safe defaults apply.
func Test_LoadConfig_discoverPingDefaults(t *testing.T) {
	setBaseEnv(t)

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "ignored.json", cfg.IgnoredFile)
	require.Equal(t, 0, cfg.DiscoverIntervalSec)
	require.Equal(t, 30, cfg.DiscoverCooldownSec)
	require.Equal(t, 128, cfg.DiscoverConcurrency)
	require.Equal(t, "", cfg.DiscoverPorts)
	require.True(t, cfg.DiscoverResolve)
	require.Equal(t, 2, cfg.PingTimeoutSec)
	require.Equal(t, "", cfg.PingTCPPorts)
}

// Given: valid discovery and ping knobs
// When: loaded
// Then: the values are honored.
func Test_LoadConfig_honorsDiscoverPingKnobs(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DISCOVER_INTERVAL_SEC", "300")
	t.Setenv("DISCOVER_COOLDOWN_SEC", "60")
	t.Setenv("DISCOVER_CONCURRENCY", "64")
	t.Setenv("DISCOVER_PORTS", "22,80")
	t.Setenv("DISCOVER_RESOLVE", "off")
	t.Setenv("PING_TIMEOUT_SEC", "5")
	t.Setenv("PING_TCP_PORTS", "22,443")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 300, cfg.DiscoverIntervalSec)
	require.Equal(t, 60, cfg.DiscoverCooldownSec)
	require.Equal(t, 64, cfg.DiscoverConcurrency)
	require.Equal(t, "22,80", cfg.DiscoverPorts)
	require.False(t, cfg.DiscoverResolve)
	require.Equal(t, 5, cfg.PingTimeoutSec)
	require.Equal(t, "22,443", cfg.PingTCPPorts)
}

// Given: missing, unparsable, or out-of-range knobs
// When: loaded
// Then: safe defaults apply.
func Test_LoadConfig_fallsBackOnBadDiscoverKnobs(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DISCOVER_INTERVAL_SEC", "30")
	t.Setenv("DISCOVER_COOLDOWN_SEC", "1")
	t.Setenv("DISCOVER_CONCURRENCY", "0")
	t.Setenv("DISCOVER_RESOLVE", "maybe")
	t.Setenv("PING_TIMEOUT_SEC", "99")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 0, cfg.DiscoverIntervalSec)
	require.Equal(t, 30, cfg.DiscoverCooldownSec)
	require.Equal(t, 128, cfg.DiscoverConcurrency)
	require.True(t, cfg.DiscoverResolve)
	require.Equal(t, 2, cfg.PingTimeoutSec)

	t.Setenv("DISCOVER_INTERVAL_SEC", "90000")
	cfg, err = config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 0, cfg.DiscoverIntervalSec)
}

// Given: port CSV values
// When: parsed
// Then: duplicates collapse, empties stay nil, bad ports error.
func Test_ParsePorts(t *testing.T) {
	cfg := &config.Config{DiscoverPorts: "22,80,22"}
	ports, err := cfg.ParseProbePorts()
	require.NoError(t, err)
	require.Equal(t, []int{22, 80}, ports)

	cfg = &config.Config{}
	ports, err = cfg.ParseProbePorts()
	require.NoError(t, err)
	require.Nil(t, ports)

	for _, raw := range []string{"0", "99999", "abc", "22,abc"} {
		cfg = &config.Config{DiscoverPorts: raw}
		_, err = cfg.ParseProbePorts()
		require.Error(t, err, raw)
	}

	cfg = &config.Config{PingTCPPorts: "22,443"}
	names, err := cfg.ParsePingTCPPorts()
	require.NoError(t, err)
	require.Equal(t, []string{"22", "443"}, names)

	cfg = &config.Config{}
	names, err = cfg.ParsePingTCPPorts()
	require.NoError(t, err)
	require.Nil(t, names)

	cfg = &config.Config{PingTCPPorts: "99999"}
	_, err = cfg.ParsePingTCPPorts()
	require.Error(t, err)
}
