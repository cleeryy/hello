package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setDoctorEnv(t *testing.T, token string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("API_TOKEN", token)
	t.Setenv("API_TOKEN_FILE", "")
	t.Setenv("DEFAULT_MAC", "00:11:22:33:44:55")
	t.Setenv("DEVICES_FILE", filepath.Join(dir, "devices.json"))
	t.Setenv("HISTORY_FILE", filepath.Join(dir, "hist.json"))
	t.Setenv("SCHEDULES_FILE", filepath.Join(dir, "sched.json"))
	t.Setenv("PORT", "18099")
	t.Setenv("STATUS_WEBHOOK_URL", "")
	t.Setenv("WAKE_WEBHOOK_URL", "")
}

// Given: a valid environment
// When: doctor runs with --json
// Then: it reports success.
func TestDoctor_ok(t *testing.T) {
	setDoctorEnv(t, "doctor-token-abcdef-1234")
	require.NoError(t, runDoctor([]string{"--json"}))
}

// Given: a missing API token
// When: doctor runs
// Then: it fails.
func TestDoctor_missingToken(t *testing.T) {
	setDoctorEnv(t, "")
	require.Error(t, runDoctor([]string{}))
}

// Given: an unknown flag
// When: doctor runs
// Then: it fails fast.
func TestDoctor_unknownFlag(t *testing.T) {
	setDoctorEnv(t, "doctor-token-abcdef-1234")
	require.Error(t, runDoctor([]string{"--frobnicate"}))
}
