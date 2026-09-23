package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

var errFakeSend = errors.New("fake send failure")

func postSched(t *testing.T, r *gin.Engine, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getSched(t *testing.T, r *gin.Engine, target string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeSchedules(t *testing.T, w *httptest.ResponseRecorder) []models.Schedule {
	t.Helper()
	var body struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Schedules
}

// Given: a cron schedule
// When: created
// Then: the response carries the computed next run.
func TestSchedules_whenNextRunEnriched(t *testing.T) {
	r, _ := scheduleRouter(t)

	w := postSched(t, r, "/schedules", `{"id":"n","device_id":"pc","cron":"* * * * *"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var created models.Schedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Greater(t, created.NextRun, time.Now().Unix())
}

// Given: a future RFC3339 timestamp
// When: posted as once_at
// Then: a one-shot schedule is created; past or bad values are 422.
func TestSchedules_whenOneShotCreated(t *testing.T) {
	r, _ := scheduleRouter(t)
	future := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)

	w := postSched(t, r, "/schedules", `{"id":"once1","device_id":"pc","once_at":"`+future+`"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var created models.Schedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.True(t, created.Once)
	at, err := time.Parse(time.RFC3339, future)
	require.NoError(t, err)
	require.Equal(t, at.Unix(), created.OnceAt)
	require.NotEmpty(t, created.Cron)

	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	w = postSched(t, r, "/schedules", `{"id":"bad","device_id":"pc","once_at":"`+past+`"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	w = postSched(t, r, "/schedules", `{"id":"bad2","device_id":"pc","once_at":"soon"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a valid schedule payload with dry_run
// When: posted
// Then: 200 with the validated schedule, nothing persisted.
func TestSchedules_whenCreateDryRun(t *testing.T) {
	r, _ := scheduleRouter(t)

	w := postSched(t, r, "/schedules?dry_run=1", `{"id":"x","device_id":"pc","cron":"* * * * *"}`)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		DryRun   bool            `json:"dry_run"`
		Schedule models.Schedule `json:"schedule"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.DryRun)
	require.Equal(t, "x", body.Schedule.ID)
	require.Empty(t, decodeSchedules(t, getSched(t, r, "/schedules")))
}

// Given: two schedules
// When: pause-all then resume-all
// Then: counts match and every row flips.
func TestSchedules_whenPauseResumeAll(t *testing.T) {
	r, _ := scheduleRouter(t)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"a","device_id":"pc","cron":"@daily"}`).Code)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"b","device_id":"pc","cron":"@daily"}`).Code)

	w := postSched(t, r, "/schedules/pause-all", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"changed":2`)
	for _, s := range decodeSchedules(t, getSched(t, r, "/schedules")) {
		require.False(t, s.Enabled)
	}

	w = postSched(t, r, "/schedules/resume-all", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"changed":2`)
}

// Given: cron expressions
// When: validated
// Then: verdict plus next run for valid ones, false for bad ones.
func TestSchedules_whenValidated(t *testing.T) {
	r, _ := scheduleRouter(t)

	w := postSched(t, r, "/schedules/validate", `{"cron":"* * * * *"}`)
	require.Equal(t, http.StatusOK, w.Code)
	var good struct {
		Valid   bool  `json:"valid"`
		NextRun int64 `json:"next_run"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &good))
	require.True(t, good.Valid)
	require.Greater(t, good.NextRun, time.Now().Unix())

	w = postSched(t, r, "/schedules/validate", `{"cron":"nope"}`)
	var bad struct {
		Valid bool `json:"valid"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bad))
	require.False(t, bad.Valid)

	w = postSched(t, r, "/schedules/validate", `{"cron":`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// Given: an existing schedule
// When: duplicated twice
// Then: fresh ids copy- and copy-2 suffixed; unknown id is 404.
func TestSchedules_whenDuplicated(t *testing.T) {
	r, _ := scheduleRouter(t)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"dup1","device_id":"pc","cron":"@daily"}`).Code)

	w := postSched(t, r, "/schedules/dup1/duplicate", "")
	require.Equal(t, http.StatusCreated, w.Code)
	var first models.Schedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	require.Equal(t, "dup1-copy", first.ID)

	w = postSched(t, r, "/schedules/dup1/duplicate", "")
	var second models.Schedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	require.Equal(t, "dup1-copy-2", second.ID)

	w = postSched(t, r, "/schedules/ghost/duplicate", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// newFireServer wires history and a controllable sender for fire-now tests.
func newFireServer(t *testing.T, capN int, sendErr error) (*gin.Engine, *history.History) {
	t.Helper()
	dir := t.TempDir()
	devStore := newStorage(t, filepath.Join(dir, "devices.json"))
	require.NoError(t, devStore.Create(&models.Device{
		ID: "pc", Name: "PC", MAC: "00:11:22:33:44:55", IP: "192.168.1.10",
	}))
	var schedStore *scheduler.Store
	var err error
	if capN > 0 {
		schedStore, err = scheduler.NewStoreWithCap(filepath.Join(dir, "schedules.json"), capN)
	} else {
		schedStore, err = scheduler.NewStore(filepath.Join(dir, "schedules.json"))
	}
	require.NoError(t, err)
	h, err := history.New(filepath.Join(dir, "history.json"))
	require.NoError(t, err)
	sch := scheduler.New(schedStore,
		func(id string) (models.Device, error) {
			dev, err := devStore.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *dev, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	srv := New(&config.Config{BroadcastIP: "255.255.255.255"}, devStore, wshub.NewHub())
	srv.WithHistory(h).WithSchedules(schedStore, sch)
	srv.sendWOL = func(mac, broadcast string) error { return sendErr }
	r := gin.New()
	srv.Mount(r)
	return r, h
}

// Given: a schedule with a reachable target
// When: fired now
// Then: 200, a schedule history entry, and the run record refreshed.
func TestSchedules_whenFiredNow(t *testing.T) {
	r, h := newFireServer(t, 0, nil)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"f1","device_id":"pc","cron":"@daily"}`).Code)

	w := postSched(t, r, "/schedules/f1/fire", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)

	entries := h.List("", 50)
	require.Len(t, entries, 1)
	require.Equal(t, models.TriggerSchedule, entries[0].Trigger)
	require.True(t, entries[0].Success)

	rows := decodeSchedules(t, getSched(t, r, "/schedules"))
	require.Len(t, rows, 1)
	require.Greater(t, rows[0].LastRunAt, int64(0))
	require.Equal(t, "ok", rows[0].LastResult)

	w = postSched(t, r, "/schedules/ghost/fire", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// Given: a failing sender
// When: a schedule is fired now
// Then: 500 and the error run record.
func TestSchedules_whenFireNowFails(t *testing.T) {
	r, _ := newFireServer(t, 0, errFakeSend)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"f9","device_id":"pc","cron":"@daily"}`).Code)

	w := postSched(t, r, "/schedules/f9/fire", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	rows := decodeSchedules(t, getSched(t, r, "/schedules"))
	require.Len(t, rows, 1)
	require.Contains(t, rows[0].LastResult, "error:")
}

// Given: a schedule quota of one
// When: a second schedule is created
// Then: 422 quota error.
func TestSchedules_whenQuotaReached(t *testing.T) {
	r, _ := newFireServer(t, 1, nil)
	require.Equal(t, http.StatusCreated,
		postSched(t, r, "/schedules", `{"id":"s1","device_id":"pc","cron":"@daily"}`).Code)
	w := postSched(t, r, "/schedules", `{"id":"s2","device_id":"pc","cron":"@daily"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
