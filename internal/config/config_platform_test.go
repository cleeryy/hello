package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
)

func setValidBase(t *testing.T) {
	t.Helper()
	t.Setenv("API_TOKEN", "platform-token-abcdef-1234")
	t.Setenv("API_TOKEN_FILE", "")
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("STATUS_WEBHOOK_URL", "")
	t.Setenv("WAKE_WEBHOOK_URL", "")
}

// Given: API_TOKEN empty and API_TOKEN_FILE pointing at a token file
// When: loading config
// Then: the token comes from the file.
func TestLoadConfig_tokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.txt")
	require.NoError(t, os.WriteFile(path, []byte("file-token-abcdef-1234567890"), 0o600))
	t.Setenv("API_TOKEN", "")
	t.Setenv("API_TOKEN_FILE", path)
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "file-token-abcdef-1234567890", cfg.APIToken)
}

// Given: API_TOKEN_FILE pointing nowhere
// When: loading config
// Then: startup fails with a read error.
func TestLoadConfig_tokenFileMissing(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	t.Setenv("API_TOKEN_FILE", filepath.Join(t.TempDir(), "nope.txt"))
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	_, err := config.LoadConfig()
	require.Error(t, err)
}

// Given: a non-http webhook URL
// When: loading config
// Then: startup fails.
func TestLoadConfig_badWebhook(t *testing.T) {
	setValidBase(t)
	t.Setenv("STATUS_WEBHOOK_URL", "ftp://hooks.example.com/x")
	_, err := config.LoadConfig()
	require.Error(t, err)
}

// Given: an https webhook URL
// When: loading config
// Then: it is accepted and stored.
func TestLoadConfig_goodWebhook(t *testing.T) {
	setValidBase(t)
	t.Setenv("WAKE_WEBHOOK_URL", "https://hooks.example.com/wake")
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "https://hooks.example.com/wake", cfg.WakeWebhookURL)
}
