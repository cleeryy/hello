package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: one registered device
// When: PATCHing monitor_secs with a valid value
// Then: 200 and the value persists.
func TestPatchMonitorSecs(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodPatch, "/devices/pc1", map[string]any{"monitor_secs": 60})
	require.Equal(t, http.StatusOK, w.Code)
	var updated models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, 60, updated.MonitorSecs)

	w = doRequest(s, http.MethodGet, "/devices/pc1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 60, got.MonitorSecs)
}

// Given: one registered device
// When: PATCHing monitor_secs out of range
// Then: 422, nothing changes.
func TestPatchMonitorSecsInvalid(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	for _, raw := range []string{
		`{"monitor_secs":3}`,
		`{"monitor_secs":-1}`,
		`{"monitor_secs":1.5}`,
		`{"monitor_secs":90000}`,
	} {
		w = doRequestRaw(s, http.MethodPatch, "/devices/pc1", raw)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, raw)
	}
}

// Given: one registered device
// When: PATCHing an overlong note
// Then: 422 (not 500): model errors map to client errors.
func TestPatchNotesTooLong(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodPatch, "/devices/pc1",
		map[string]any{"notes": strings.Repeat("n", 501)})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}
