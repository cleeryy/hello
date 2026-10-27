package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
)

// Given: a slow unroutable sweep in flight
// When: scanning again while it runs
// Then: 429 (busy branch of runScan, or cooldown right after).
func TestRunScanBusy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := discover.New()
	d.ProbePorts = []int{9}
	srv := New(&config.Config{}, newStorage(t, t.TempDir()+"/d.json"), nil).WithDiscover(d)
	r := gin.New()
	srv.Mount(r)

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		first <- doPOST(t, r, "/discover?cidr=192.0.2.0/24", "")
	}()
	seen := false
	for i := 0; i < 300 && !seen; i++ {
		w := doGET(t, r, "/discover/status")
		var rep struct {
			Scanning bool `json:"scanning"`
		}
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &rep); err == nil && rep.Scanning {
				seen = true
			}
		}
		if !seen {
			time.Sleep(50 * time.Millisecond)
		}
	}
	require.True(t, seen, "first scan never showed as running")

	w := doPOST(t, r, "/discover?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusTooManyRequests, w.Code)

	select {
	case <-first:
	case <-time.After(2 * time.Minute):
		t.Fatal("first scan never finished")
	}
}

// Given: a server without history or schedules
// When: GETting /health and /history
// Then: zero values, no crash (nil branches).
func TestHealthAndHistoryMinimal(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/health", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"total":0`)

	w = doRequest(s, http.MethodGet, "/history", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"history":[]`)
}

// Given: an empty schedule store
// When: touching unknown schedule ids
// Then: 404 on update, delete, duplicate, and fire.
func TestScheduleUnknownIDs(t *testing.T) {
	r, _ := scheduleRouter(t)

	w := doPOST(t, r, "/schedules/nope/duplicate", "")
	require.Equal(t, http.StatusNotFound, w.Code)

	w = doPOST(t, r, "/schedules/nope/fire", "")
	require.Equal(t, http.StatusNotFound, w.Code)

	req, _ := http.NewRequest(http.MethodPut, "/schedules/nope",
		bytes.NewReader([]byte(`{"id":"nope","device_id":"pc","cron":"* * * * *"}`)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)

	req, _ = http.NewRequest(http.MethodDelete, "/schedules/nope", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// Given: a failing sender
// When: POSTing a raw MAC wake
// Then: 500 (sendMagic error branch).
func TestWakeMACSendFail(t *testing.T) {
	s := newTestServer(t)
	s.sendWOL = func(mac, broadcast string) error { return errors.New("boom") }

	w := doRequest(s, http.MethodPost, "/wake/00:11:22:33:44:55", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// Given: an ignore list
// When: adding invalid entries
// Then: 422 for empty, bad MAC, and bad IP.
func TestAddIgnoreInvalid(t *testing.T) {
	r := discoverFullRouter(t, 0, t.TempDir()+"/ignored.json")

	for _, body := range []string{`{}`, `{"mac":"bad"}`, `{"ip":"999.1.1.1"}`} {
		w := doPOST(t, r, "/discover/ignore", body)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, body)
	}
}
