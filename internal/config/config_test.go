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
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")
	t.Setenv("PORT", "")
	t.Setenv("BROADCAST_IP", "")
	t.Setenv("DEVICES_FILE", "")
	t.Setenv("MONITOR_INTERVAL_SEC", "")
	t.Setenv("HISTORY_FILE", "")
	t.Setenv("SCHEDULES_FILE", "")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, "00:11:22:33:44:55", cfg.DefaultMAC)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "255.255.255.255", cfg.BroadcastIP)
	assert.Equal(t, "devices.json", cfg.DevicesFile)
	assert.Equal(t, 30*time.Second, cfg.MonitorInterval)
	assert.Equal(t, "wake-history.json", cfg.HistoryFile)
	assert.Equal(t, "schedules.json", cfg.SchedulesFile)
	assert.Equal(t, "test-token-16-chars-ok", cfg.APIToken)
	assert.Equal(t, "127.0.0.1,::1", cfg.TrustedProxies)
	assert.Empty(t, cfg.CORSOrigins)
}

func Test_LoadConfig_honors_env_overrides(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "AA:BB:CC:DD:EE:FF")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")
	t.Setenv("PORT", "9090")
	t.Setenv("BROADCAST_IP", "192.168.1.255")
	t.Setenv("DEVICES_FILE", "/tmp/d.json")
	t.Setenv("MONITOR_INTERVAL_SEC", "5")
	t.Setenv("HISTORY_FILE", "/tmp/h.json")
	t.Setenv("SCHEDULES_FILE", "/tmp/s.json")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "192.168.1.255", cfg.BroadcastIP)
	assert.Equal(t, "/tmp/d.json", cfg.DevicesFile)
	assert.Equal(t, 5*time.Second, cfg.MonitorInterval)
	assert.Equal(t, "/tmp/h.json", cfg.HistoryFile)
	assert.Equal(t, "/tmp/s.json", cfg.SchedulesFile)
}

func Test_LoadConfig_rejects_missing_default_mac(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")

	// When
	_, err := config.LoadConfig()

	// Then
	require.Error(t, err)
}

func Test_LoadConfig_falls_back_on_bad_interval(t *testing.T) {
	// Given
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")
	t.Setenv("MONITOR_INTERVAL_SEC", "nope")

	// When
	cfg, err := config.LoadConfig()

	// Then
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.MonitorInterval)
}

func Test_LoadConfig_rejects_unsafe_or_invalid_security_config(t *testing.T) {
	tests := map[string]map[string]string{
		"missing token": {
			"DEFAULT_MAC": "00:11:22:33:44:55",
		},
		"weak default token": {
			"DEFAULT_MAC": "00:11:22:33:44:55",
			"API_TOKEN":   "change-me",
		},
		"short token": {
			"DEFAULT_MAC": "00:11:22:33:44:55",
			"API_TOKEN":   "short",
		},
		"invalid trusted proxy": {
			"DEFAULT_MAC":     "00:11:22:33:44:55",
			"API_TOKEN":       "test-token-16-chars-ok",
			"TRUSTED_PROXIES": "not-a-proxy",
		},
		"wildcard CORS origin": {
			"DEFAULT_MAC":  "00:11:22:33:44:55",
			"API_TOKEN":    "test-token-16-chars-ok",
			"CORS_ORIGINS": "*",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			for key, value := range env {
				t.Setenv(key, value)
			}
			_, err := config.LoadConfig()
			require.Error(t, err)
		})
	}
}

func Test_LoadConfig_parses_explicit_origins(t *testing.T) {
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("API_TOKEN", "test-token-16-chars-ok")
	t.Setenv("CORS_ORIGINS", "https://wake.example.com, https://wake.example.com, http://localhost:8080")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	origins, err := cfg.ParseCORSOrigins()
	require.NoError(t, err)
	assert.Equal(t, []string{"https://wake.example.com", "http://localhost:8080"}, origins)
}
