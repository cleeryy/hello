package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/monitor"
)

// monitorRouter wires one unroutable ping-enabled device to a monitor with a
// fast per-host timeout, returning the mounted engine.
func monitorRouter(t *testing.T, timeout time.Duration) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := newStorage(t, t.TempDir()+"/d.json")
	require.NoError(t, store.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		IP: "192.0.2.1", Status: models.StatusUnknown, PingEnabled: true,
	}))
	srv := New(&config.Config{}, store, nil).
		WithMonitor(monitor.NewWithTimeout(store, time.Hour, timeout))
	r := gin.New()
	srv.Mount(r)
	return r
}

// Given: one unknown device on an unroutable address
// When: POSTing /monitor/check twice
// Then: first pass checks 1 and changes 1, second changes nothing.
func TestMonitorCheck(t *testing.T) {
	r := monitorRouter(t, 200*time.Millisecond)

	w := doPOST(t, r, "/monitor/check", "")
	require.Equal(t, http.StatusOK, w.Code)
	var first struct {
		Checked int `json:"checked"`
		Changed int `json:"changed"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	require.Equal(t, 1, first.Checked)
	require.Equal(t, 1, first.Changed)

	w = doPOST(t, r, "/monitor/check", "")
	require.Equal(t, http.StatusOK, w.Code)
	var second struct {
		Checked int `json:"checked"`
		Changed int `json:"changed"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	require.Equal(t, 1, second.Checked)
	require.Equal(t, 0, second.Changed)
}

// Given: one check pass over a fresh device
// When: GETting /monitor/transitions
// Then: one unknown-to-down transition for pc1.
func TestMonitorTransitions(t *testing.T) {
	r := monitorRouter(t, 200*time.Millisecond)
	require.Equal(t, http.StatusOK, doPOST(t, r, "/monitor/check", "").Code)

	w := doGET(t, r, "/monitor/transitions")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Transitions []monitor.Transition `json:"transitions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Transitions, 1)
	require.Equal(t, "pc1", body.Transitions[0].DeviceID)
	require.Equal(t, "unknown", body.Transitions[0].From)
	require.Equal(t, "down", body.Transitions[0].To)
	require.Greater(t, body.Transitions[0].At, int64(0))
}

// Given: one check pass
// When: GETting /monitor/uptime
// Then: pc1 shows 0 up over 1 total poll.
func TestMonitorUptime(t *testing.T) {
	r := monitorRouter(t, 200*time.Millisecond)
	require.Equal(t, http.StatusOK, doPOST(t, r, "/monitor/check", "").Code)

	w := doGET(t, r, "/monitor/uptime")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Devices []struct {
			DeviceID string  `json:"device_id"`
			Up       int     `json:"up"`
			Total    int     `json:"total"`
			Percent  float64 `json:"percent"`
		} `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Devices, 1)
	require.Equal(t, "pc1", body.Devices[0].DeviceID)
	require.Equal(t, 0, body.Devices[0].Up)
	require.Equal(t, 1, body.Devices[0].Total)
	require.Equal(t, float64(0), body.Devices[0].Percent)
}

// Given: a fresh monitor
// When: GETting /monitor/flapping and /monitor/status
// Then: no flapping, and the tuning plus latest pass are reported.
func TestMonitorFlappingAndStatus(t *testing.T) {
	r := monitorRouter(t, 200*time.Millisecond)

	w := doGET(t, r, "/monitor/flapping")
	require.Equal(t, http.StatusOK, w.Code)
	var flap struct {
		Devices []string `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &flap))
	require.Empty(t, flap.Devices)

	require.Equal(t, http.StatusOK, doPOST(t, r, "/monitor/check", "").Code)

	w = doGET(t, r, "/monitor/status")
	require.Equal(t, http.StatusOK, w.Code)
	var st struct {
		Interval int64 `json:"interval_seconds"`
		Timeout  int64 `json:"timeout_seconds"`
		LastRun  int64 `json:"last_run"`
		Checked  int   `json:"last_checked"`
		Changed  int   `json:"last_changed"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &st))
	require.Equal(t, int64(3600), st.Interval)
	require.Greater(t, st.LastRun, int64(0))
	require.Equal(t, 1, st.Checked)
	require.Equal(t, 1, st.Changed)
}

// Given: a server without a monitor
// When: hitting the monitor routes
// Then: 404, the routes stay unregistered.
func TestMonitorRoutesMissing(t *testing.T) {
	s := newTestServer(t)

	require.Equal(t, http.StatusNotFound, doRequest(s, http.MethodGet, "/monitor/status", nil).Code)
	require.Equal(t, http.StatusNotFound, doRequest(s, http.MethodPost, "/monitor/check", nil).Code)
}
