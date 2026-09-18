package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the runtime configuration. Values come from the
// environment, optionally supplemented by a local .env file.
type Config struct {
	DefaultMAC      string
	Port            string
	BroadcastIP     string
	DevicesFile     string
	MonitorInterval time.Duration
}

const (
	defaultPort            = "8080"
	defaultBroadcastIP     = "255.255.255.255"
	defaultDevicesFile     = "devices.json"
	defaultMonitorInterval = 30 * time.Second
)

// LoadConfig reads the configuration. It never exits the process:
// a missing DEFAULT_MAC is returned as an error for main to handle.
func LoadConfig() (*Config, error) {
	// .env is optional; real environment always wins.
	_ = godotenv.Load()

	cfg := &Config{
		DefaultMAC:      os.Getenv("DEFAULT_MAC"),
		Port:            envOr("PORT", defaultPort),
		BroadcastIP:     envOr("BROADCAST_IP", defaultBroadcastIP),
		DevicesFile:     envOr("DEVICES_FILE", defaultDevicesFile),
		MonitorInterval: monitorInterval(),
	}

	if cfg.DefaultMAC == "" {
		return nil, fmt.Errorf("config: DEFAULT_MAC is required")
	}
	if _, err := net.ParseMAC(cfg.DefaultMAC); err != nil {
		return nil, fmt.Errorf("config: invalid DEFAULT_MAC %q", cfg.DefaultMAC)
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func monitorInterval() time.Duration {
	raw := os.Getenv("MONITOR_INTERVAL_SEC")
	if raw == "" {
		return defaultMonitorInterval
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return defaultMonitorInterval
	}
	return time.Duration(secs) * time.Second
}
