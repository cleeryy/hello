package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
)

func seedHistoryExtra(t *testing.T, h *history.History) {
	t.Helper()
	now := time.Now().Unix()
	entries := []models.WakeEvent{
		{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, At: now - 3600},
		{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerSchedule, Success: false, Error: "boom", At: now - 1800},
		{DeviceID: "box", MAC: "AA:BB:CC:DD:EE:FF", Trigger: models.TriggerManual, Success: false, At: now - 60},
	}
	for _, e := range entries {
		_, err := h.Record(e)
		require.NoError(t, err)
	}
}

func getHistory(t *testing.T, engine *gin.Engine, target string) []models.WakeEvent {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		History []models.WakeEvent `json:"history"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.History
}

// Given: three recorded wakes across triggers, results, and time
// When: listed with each filter dimension
// Then: only matching entries come back newest-first; bad values are 422.
func TestHistory_whenFiltered(t *testing.T) {
	engine, h := mountWithHistory(t)
	seedHistoryExtra(t, h)
	now := time.Now()

	got := getHistory(t, engine, "/history?trigger=manual")
	require.Len(t, got, 2)
	require.Equal(t, "box", got[0].DeviceID)

	got = getHistory(t, engine, "/history?trigger=schedule&success=false")
	require.Len(t, got, 1)
	require.Equal(t, "boom", got[0].Error)

	got = getHistory(t, engine, "/history?success=false")
	require.Len(t, got, 2)

	got = getHistory(t, engine, "/history?device_id=pc1")
	require.Len(t, got, 2)

	since := now.Add(-4000 * time.Second).UTC().Format(time.RFC3339)
	require.Len(t, getHistory(t, engine, "/history?since="+since), 3)
	before := now.Add(-1000 * time.Second).UTC().Format(time.RFC3339)
	require.Len(t, getHistory(t, engine, "/history?before="+before), 2)

	for _, target := range []string{
		"/history?trigger=bogus",
		"/history?success=maybe",
		"/history?since=tomorrow",
		"/history?before=soon",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, target)
	}
}

// Given: the same three wakes
// When: stats are requested
// Then: totals, split, and rate match the retained log.
func TestHistory_whenStats(t *testing.T) {
	engine, h := mountWithHistory(t)
	seedHistoryExtra(t, h)

	req := httptest.NewRequest(http.MethodGet, "/history/stats", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Total     int                       `json:"total"`
		Ok        int                       `json:"ok"`
		Error     int                       `json:"error"`
		Rate      float64                   `json:"success_rate"`
		ByTrigger map[string]map[string]int `json:"by_trigger"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Total)
	require.Equal(t, 1, body.Ok)
	require.Equal(t, 2, body.Error)
	require.InDelta(t, 0.3333, body.Rate, 0.0001)
	require.Equal(t, 1, body.ByTrigger["manual"]["ok"])
	require.Equal(t, 1, body.ByTrigger["manual"]["error"])
	require.Equal(t, 1, body.ByTrigger["schedule"]["error"])
}

// Given: retained wakes
// When: purged with and without a valid bound
// Then: 400 without before, 422 on bad timestamp, count + empty log on success.
func TestHistory_whenPurged(t *testing.T) {
	engine, h := mountWithHistory(t)
	seedHistoryExtra(t, h)

	serve := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, target, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, http.StatusBadRequest, serve("/history").Code)
	require.Equal(t, http.StatusUnprocessableEntity, serve("/history?before=soon").Code)

	before := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	rec := serve("/history?before=" + before)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Purged int `json:"purged"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 3, body.Purged)
	require.Empty(t, getHistory(t, engine, "/history"))
}

// Given: one wake with a note
// When: exported as CSV
// Then: header plus the row download as an attachment.
func TestHistory_whenExportedCSV(t *testing.T) {
	engine, h := mountWithHistory(t)
	_, err := h.Record(models.WakeEvent{
		DeviceID: "pc1", MAC: "00:11:22:33:44:55",
		Trigger: models.TriggerManual, Success: true, Note: "soir",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/history/export", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "history.csv")
	require.Contains(t, rec.Body.String(), "id,at,device_id,mac,trigger,success,attempts,note,error")
	require.Contains(t, rec.Body.String(), "soir")
}
