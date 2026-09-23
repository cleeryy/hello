package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: one registered device
// When: GETting /health
// Then: status ok plus device counts, version, uptime and store sizes.
func TestHealthEnriched(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.1.10",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodGet, "/health", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Uptime  int64  `json:"uptime_seconds"`
		Devices struct {
			Total   int `json:"total"`
			Up      int `json:"up"`
			Down    int `json:"down"`
			Unknown int `json:"unknown"`
		} `json:"devices"`
		Schedules struct {
			Total   int `json:"total"`
			Enabled int `json:"enabled"`
		} `json:"schedules"`
		HistorySize int `json:"history_size"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &health))
	require.Equal(t, "ok", health.Status)
	require.NotEmpty(t, health.Version)
	require.GreaterOrEqual(t, health.Uptime, int64(0))
	require.Equal(t, 1, health.Devices.Total)
	require.Equal(t, 1, health.Devices.Unknown)
	require.Equal(t, 0, health.Schedules.Total)
	require.Equal(t, 0, health.HistorySize)
}

// Given: a fresh server
// When: GETting /version
// Then: 200 with the build version (dev by default).
func TestVersionEndpoint(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/version", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "dev", body.Version)
}
