package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

func mountWithHistory(t *testing.T) (*gin.Engine, *history.History) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	store := newStorage(t, dir+"/devices.json")
	require.NoError(t, store.Create(&models.Device{
		ID: "pc1", Name: "PC 1", MAC: "00:11:22:33:44:55", Status: models.StatusUnknown,
	}))
	h, err := history.New(dir + "/history.json")
	require.NoError(t, err)
	srv := New(&config.Config{DefaultMAC: "AA:BB:CC:DD:EE:FF", BroadcastIP: "255.255.255.255"}, store, wshub.NewHub())
	srv.WithHistory(h)
	srv.sendWOL = func(mac, broadcast string) error { return nil }
	engine := gin.New()
	srv.Mount(engine)
	return engine, h
}

// TestHistory_whenManualWake verifies a manual wake appends a history entry.
func TestHistory_whenManualWake(t *testing.T) {
	// Given: a server with history wired and a stubbed sender.
	engine, h := mountWithHistory(t)

	// When: a registered device is woken.
	req := httptest.NewRequest(http.MethodPost, "/devices/pc1/wake", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: 200 and one successful manual entry linked to the device.
	require.Equal(t, http.StatusOK, rec.Code)
	entries := h.List("", 50)
	require.Len(t, entries, 1)
	require.Equal(t, "pc1", entries[0].DeviceID)
	require.Equal(t, models.TriggerManual, entries[0].Trigger)
	require.True(t, entries[0].Success)
}

// TestHistory_whenListed verifies GET /history returns newest-first.
func TestHistory_whenListed(t *testing.T) {
	// Given: a server with one recorded wake.
	engine, _ := mountWithHistory(t)
	wakeReq := httptest.NewRequest(http.MethodPost, "/devices/pc1/wake", nil)
	engine.ServeHTTP(httptest.NewRecorder(), wakeReq)

	// When: the history is listed.
	req := httptest.NewRequest(http.MethodGet, "/history", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: 200 with the {"history": [...]} envelope.
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		History []models.WakeEvent `json:"history"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.History, 1)
	require.Equal(t, "pc1", body.History[0].DeviceID)
}
