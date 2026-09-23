package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: one device per reachability state
// When: GETting /health
// Then: each counter reads 1 (counting branches).
func TestHealthDeviceCounts(t *testing.T) {
	s := newTestServer(t)
	seed := []struct {
		id     string
		mac    string
		status models.Status
	}{
		{"up", "AA:BB:CC:DD:EE:A1", models.StatusUp},
		{"down", "AA:BB:CC:DD:EE:A2", models.StatusDown},
		{"unk", "AA:BB:CC:DD:EE:A3", models.StatusUnknown},
	}
	for _, d := range seed {
		require.NoError(t, s.store.Create(&models.Device{
			ID: d.id, Name: d.id, MAC: d.mac, Status: d.status,
		}))
	}

	w := doRequest(s, http.MethodGet, "/health", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, `"up":1`)
	require.Contains(t, body, `"down":1`)
	require.Contains(t, body, `"unknown":1`)
}

// Given: a schedule router
// When: updating a schedule toward a ghost device
// Then: 422 before touching the store.
func TestUpdateScheduleUnknownDevice(t *testing.T) {
	r, _ := scheduleRouter(t)

	req, _ := http.NewRequest(http.MethodPut, "/schedules/x",
		bytes.NewReader([]byte(`{"id":"x","device_id":"ghost","cron":"* * * * *"}`)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// Given: a bad default MAC
// When: POSTing the default wake
// Then: 422 (validation branch of wakeDefault).
func TestWakeDefaultBadMAC(t *testing.T) {
	s := newTestServer(t)
	s.cfg.DefaultMAC = "not-a-mac"

	w := doRequest(s, http.MethodPost, "/wake", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a MAC outside the registry
// When: POSTing a raw wake for it
// Then: 200 through the configured broadcast (LookupMAC-miss branch).
func TestWakeMACUnknown(t *testing.T) {
	s := newTestServer(t)
	s.cfg.BroadcastIP = "255.255.255.255"
	var gotBroadcast string
	s.sendWOL = func(mac, broadcast string) error { gotBroadcast = broadcast; return nil }

	w := doRequest(s, http.MethodPost, "/wake/00:11:22:33:44:99", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "255.255.255.255", gotBroadcast)
}

// Given: any server
// When: GETting /history with a bad limit
// Then: 400 (limit parsing branch).
func TestListHistoryBadLimit(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/history?limit=abc", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// Given: a schedule whose device was deleted
// When: firing it manually
// Then: 422 (unknown-target branch of fireScheduleNow).
func TestFireScheduleDeletedDevice(t *testing.T) {
	r, _ := scheduleRouter(t)

	w := doPOST(t, r, "/schedules", `{"id":"f1","device_id":"pc","cron":"* * * * *"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	req, _ := http.NewRequest(http.MethodDelete, "/devices/pc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	w = doPOST(t, r, "/schedules/f1/fire", "")
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
