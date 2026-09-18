package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
)

func Test_LoadConfig_returns_defaults(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("PORT", "")
	t.Setenv("BROADCAST_IP", "")
	t.Setenv("DEVICES_FILE", "")
	t.Setenv("MONITOR_INTERVAL_SEC", "")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, "00:11:22:33:44:55", cfg.DefaultMAC)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "255.255.255.255", cfg.BroadcastIP)
	assert.Equal(t, "devices.json", cfg.DevicesFile)
	assert.Equal(t, 30*time.Second, cfg.MonitorInterval)
}

func Test_LoadConfig_honors_env_overrides(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "AA:BB:CC:DD:EE:FF")
	t.Setenv("PORT", "9090")
	t.Setenv("BROADCAST_IP", "192.168.1.255")
	t.Setenv("DEVICES_FILE", "/tmp/d.json")
	t.Setenv("MONITOR_INTERVAL_SEC", "5")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "192.168.1.255", cfg.BroadcastIP)
	assert.Equal(t, "/tmp/d.json", cfg.DevicesFile)
	assert.Equal(t, 5*time.Second, cfg.MonitorInterval)
}

func Test_LoadConfig_rejects_missing_default_mac(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "")

	// When
	_, err := config.LoadConfig()

	// Then
	require.Error(t, err)
}

func Test_LoadConfig_falls_back_on_bad_interval(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("MONITOR_INTERVAL_SEC", "nope")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.MonitorInterval)
}
