package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

func scheduleRouter(t *testing.T) (*gin.Engine, *scheduler.Scheduler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{BroadcastIP: "255.255.255.255"}
	devStore := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	err := devStore.Create(&models.Device{ID: "pc", Name: "PC", MAC: "00:11:22:33:44:55"})
	require.NoError(t, err)
	schedStore := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	sch := scheduler.New(schedStore,
		func(id string) (models.Device, error) {
			dev, err := devStore.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *dev, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	srv := New(cfg, devStore, wshub.NewHub()).WithSchedules(schedStore, sch)
	r := gin.New()
	srv.Mount(r)
	return r, sch
}

func TestSchedules_whenListEmpty(t *testing.T) {
	// Given: no schedules
	r, _ := scheduleRouter(t)
	// When: listed
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/schedules", nil)
	r.ServeHTTP(w, req)
	// Then: empty envelope
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Empty(t, body.Schedules)
}

func TestSchedules_whenCreateValid(t *testing.T) {
	// Given: a create request for a known device
	r, sch := scheduleRouter(t)
	// When: posted
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/schedules",
		bytes.NewBufferString(`{"id":"morning","device_id":"pc","cron":"0 7 * * *"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	// Then: 201 enabled and the runner picked it up
	require.Equal(t, http.StatusCreated, w.Code)
	var created models.Schedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.True(t, created.Enabled)
	require.Equal(t, 1, sch.Entries())
}

func TestSchedules_whenCreateInvalid(t *testing.T) {
	// Given: create requests with unknown device, bad cron, missing fields
	r, sch := scheduleRouter(t)
	cases := []struct {
		name string
		body string
	}{
		{"unknown device", `{"id":"s","device_id":"ghost","cron":"@daily"}`},
		{"bad cron", `{"id":"s","device_id":"pc","cron":"nope"}`},
		{"missing id", `{"device_id":"pc","cron":"@daily"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/schedules",
				bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		})
	}
	require.Equal(t, 0, sch.Entries())
}

func TestSchedules_whenDuplicate(t *testing.T) {
	// Given: an existing schedule
	r, _ := scheduleRouter(t)
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/schedules",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusCreated,
		post(`{"id":"s","device_id":"pc","cron":"@daily"}`).Code)
	// When: posted again
	// Then: 409
	require.Equal(t, http.StatusConflict,
		post(`{"id":"s","device_id":"pc","cron":"@daily"}`).Code)
}

func TestSchedules_whenPauseResumeDelete(t *testing.T) {
	// Given: a running schedule
	r, sch := scheduleRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/schedules",
		bytes.NewBufferString(`{"id":"s","device_id":"pc","cron":"@daily"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	// When: paused via PUT
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPut, "/schedules/s",
		bytes.NewBufferString(`{"id":"s","device_id":"pc","cron":"@daily","enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	// Then: runner emptied
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 0, sch.Entries())
	// When: resumed via PUT
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPut, "/schedules/s",
		bytes.NewBufferString(`{"id":"s","device_id":"pc","cron":"@daily","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, sch.Entries())
	// When: deleted
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/schedules/s", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, 0, sch.Entries())
	// When: deleted again
	// Then: 404
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/schedules/s", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}
