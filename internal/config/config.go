package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	// APIToken locks the API behind case-sensitive Bearer auth.
	APIToken string
	// HistoryFile persists the wake log.
	HistoryFile string
	// SchedulesFile persists wake schedules.
	SchedulesFile string
	// HistoryCap bounds the retained wake log (F-29).
	HistoryCap int
	// SchedulesCap bounds the number of stored schedules (F-37).
	SchedulesCap int
	// WakeCooldownSec throttles manual wakes per device, 0 disables (F-21).
	WakeCooldownSec int
	TrustedProxies  string
	CORSOrigins     string
}

const (
	defaultPort            = "8080"
	defaultBroadcastIP     = "255.255.255.255"
	defaultDevicesFile     = "devices.json"
	defaultHistoryFile     = "wake-history.json"
	defaultSchedulesFile   = "schedules.json"
	defaultMonitorInterval = 30 * time.Second
	defaultTrustedProxies  = "127.0.0.1,::1"
	minTokenLength         = 16
	defaultHistoryCap      = 200
	maxHistoryCap          = 2000
	defaultSchedulesCap    = 100
	maxSchedulesCap        = 1000
	maxWakeCooldownSec     = 3600
)

var apiTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// LoadConfig reads the configuration. It never exits the process:
// invalid or unsafe values are returned as errors for main to handle.
func LoadConfig() (*Config, error) {
	// .env is optional; real environment always wins.
	_ = godotenv.Load()

	cfg := &Config{
		DefaultMAC:      os.Getenv("DEFAULT_MAC"),
		Port:            envOr("PORT", defaultPort),
		BroadcastIP:     envOr("BROADCAST_IP", defaultBroadcastIP),
		DevicesFile:     envOr("DEVICES_FILE", defaultDevicesFile),
		MonitorInterval: monitorInterval(),
		APIToken:        os.Getenv("API_TOKEN"),
		HistoryFile:     envOr("HISTORY_FILE", defaultHistoryFile),
		SchedulesFile:   envOr("SCHEDULES_FILE", defaultSchedulesFile),
		HistoryCap:      boundedInt("HISTORY_CAP", defaultHistoryCap, 10, maxHistoryCap),
		SchedulesCap:    boundedInt("SCHEDULES_CAP", defaultSchedulesCap, 1, maxSchedulesCap),
		WakeCooldownSec: boundedInt("WAKE_COOLDOWN_SEC", 0, 0, maxWakeCooldownSec),
		TrustedProxies:  envOr("TRUSTED_PROXIES", defaultTrustedProxies),
		CORSOrigins:     os.Getenv("CORS_ORIGINS"),
	}

	if cfg.DefaultMAC == "" {
		return nil, fmt.Errorf("config: DEFAULT_MAC is required")
	}
	if _, err := net.ParseMAC(cfg.DefaultMAC); err != nil {
		return nil, fmt.Errorf("config: invalid DEFAULT_MAC %q", cfg.DefaultMAC)
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("config: API_TOKEN is required")
	}
	if cfg.APIToken == "change-me" || cfg.APIToken != strings.TrimSpace(cfg.APIToken) {
		return nil, fmt.Errorf("config: API_TOKEN is unsafe")
	}
	if len(cfg.APIToken) < minTokenLength || !apiTokenPattern.MatchString(cfg.APIToken) {
		return nil, fmt.Errorf("config: API_TOKEN must contain at least %d letters, numbers, underscores, or hyphens", minTokenLength)
	}
	if _, err := cfg.ParseTrustedProxies(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if _, err := cfg.ParseCORSOrigins(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// boundedInt reads an integer env var, falling back to fallback when
// missing, unparsable, or outside [min, max].
func boundedInt(key string, fallback, min, max int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return fallback
	}
	return n
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

// ParseTrustedProxies validates the reverse-proxy allowlist used by Gin when
// interpreting ClientIP and Forwarded headers.
func (c *Config) ParseTrustedProxies() ([]string, error) {
	parts := splitList(c.TrustedProxies)
	if len(parts) == 0 {
		return nil, fmt.Errorf("TRUSTED_PROXIES must not be empty")
	}
	for _, proxy := range parts {
		if ip := net.ParseIP(proxy); ip != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES contains invalid address or CIDR %q", proxy)
		}
	}
	return parts, nil
}

// ParseCORSOrigins validates an exact, comma-separated browser origin allowlist.
// Wildcards and non-HTTP origins are intentionally rejected.
func (c *Config) ParseCORSOrigins() ([]string, error) {
	parts := splitList(c.CORSOrigins)
	seen := make(map[string]struct{}, len(parts))
	origins := make([]string, 0, len(parts))
	for _, raw := range parts {
		if raw == "*" {
			return nil, fmt.Errorf("CORS_ORIGINS must be an explicit origin whitelist")
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("CORS_ORIGINS contains invalid origin %q", raw)
		}
		origin := u.Scheme + "://" + u.Host
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
