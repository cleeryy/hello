package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/handlers"
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	require.NoError(t, w.Close())
	os.Stdout = old
	raw, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(raw)
}

// Given: a valid environment
// When: doctor runs with --json
// Then: stdout is a versioned report with one entry per check.
func TestDoctor_jsonShape(t *testing.T) {
	setDoctorEnv(t, "doctor-token-abcdef-1234")
	var docErr error
	out := captureStdout(t, func() {
		docErr = runDoctor([]string{"--json"})
	})
	var report struct {
		Version string `json:"version"`
		OK      bool   `json:"ok"`
		Checks  []struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	require.Equal(t, handlers.Version, report.Version)
	require.NotEmpty(t, report.Checks)
	names := make([]string, 0, len(report.Checks))
	allOK := true
	for _, c := range report.Checks {
		require.NotEmpty(t, c.Name)
		require.NotEmpty(t, c.Detail)
		if !c.OK {
			allOK = false
		}
		names = append(names, c.Name)
	}
	// The aggregate flag mirrors the checks: environment-dependent entries
	// (missing arp binary, occupied port) may fail outside CI.
	require.Equal(t, allOK, report.OK)
	require.Equal(t, allOK, docErr == nil)
	for _, want := range []string{"config", "token", "devices-file", "history-file", "schedules-file", "webhooks"} {
		require.Contains(t, names, want)
	}
}
