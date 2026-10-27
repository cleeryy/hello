package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

func newOpsServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	store := newStorage(t, filepath.Join(dir, "devices.json"))
	err := store.Create(&models.Device{ID: "pc1", Name: "Test PC", MAC: "00:11:22:33:44:55", IP: "192.168.9.9"})
	require.NoError(t, err)
	h, err := history.New(filepath.Join(dir, "hist.json"))
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, Attempts: 1})
	require.NoError(t, err)
	st := newScheduleStore(t, filepath.Join(dir, "sched.json"))
	_, err = st.Create(models.Schedule{ID: "s1", DeviceID: "pc1", Cron: "@daily"})
	require.NoError(t, err)
	sched := scheduler.New(st,
		func(id string) (models.Device, error) {
			d, err := store.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *d, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	s := New(&config.Config{}, store, nil)
	s.sendWOL = func(mac, broadcast string) error { return nil }
	return s.WithHistory(h).WithSchedules(st, sched)
}

func decodeOps(t *testing.T, data []byte, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(data, v))
}

func doRaw(s *Server, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	engine := gin.New()
	s.Mount(engine)
	engine.ServeHTTP(w, req)
	return w
}

func deviceTotal(t *testing.T, s *Server) (int, []string) {
	t.Helper()
	w := doRequest(s, http.MethodGet, "/devices", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Total   int `json:"total"`
		Devices []struct {
			ID string `json:"id"`
		} `json:"devices"`
	}
	decodeOps(t, w.Body.Bytes(), &env)
	ids := make([]string, 0, len(env.Devices))
	for _, d := range env.Devices {
		ids = append(ids, d.ID)
	}
	return env.Total, ids
}

// Given: a server with one device, one history entry, one schedule
// When: GET /backup
// Then: 200 attachment with version 1 and one item per store.
func TestOps_backupRoundTrip(t *testing.T) {
	s := newOpsServer(t)
	w := doRequest(s, http.MethodGet, "/backup", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	var env struct {
		Version   int   `json:"version"`
		Devices   []any `json:"devices"`
		History   []any `json:"history"`
		Schedules []any `json:"schedules"`
	}
	decodeOps(t, w.Body.Bytes(), &env)
	require.Equal(t, 1, env.Version)
	require.Len(t, env.Devices, 1)
	require.Len(t, env.History, 1)
	require.Len(t, env.Schedules, 1)
}

// Given: a backup payload
// When: POST /restore?dry_run=1
// Then: 200 and the registry is untouched.
func TestOps_restoreDryRun(t *testing.T) {
	s := newOpsServer(t)
	payload := map[string]any{
		"devices":   []any{map[string]any{"id": "pc1", "name": "Test PC", "mac": "00:11:22:33:44:55", "ip": "192.168.9.9"}},
		"history":   []any{},
		"schedules": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore?dry_run=1", payload)
	require.Equal(t, http.StatusOK, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
}

// Given: a restore payload with an invalid MAC
// When: POST /restore
// Then: 422 and the registry is untouched.
func TestOps_restoreRejectsBad(t *testing.T) {
	s := newOpsServer(t)
	payload := map[string]any{
		"devices":   []any{map[string]any{"id": "bad", "name": "Bad", "mac": "not-a-mac"}},
		"history":   []any{},
		"schedules": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
}

// Given: a restore payload with a brand new device
// When: POST /restore
// Then: 200 and the registry holds only the new device.
func TestOps_restoreApplies(t *testing.T) {
	s := newOpsServer(t)
	payload := map[string]any{
		"devices":   []any{map[string]any{"id": "new1", "name": "New One", "mac": "AA:BB:CC:DD:EE:FF"}},
		"history":   []any{},
		"schedules": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusOK, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"new1"}, ids)
}

// Given: a configured token
// When: GET /config
// Then: 200 with settings and version, never the token itself.
func TestOps_configRedacted(t *testing.T) {
	s := newOpsServer(t)
	s.cfg.APIToken = "redact-me-token-67890"
	w := doRaw(s, http.MethodGet, "/config", map[string]string{"Authorization": "Bearer redact-me-token-67890"})
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "version")
	require.NotContains(t, body, "redact-me-token-67890")
	require.NotContains(t, strings.ToLower(body), "api_token")
}

// Given: a device list
// When: GET /devices twice, second with If-None-Match
// Then: stable ETag then 304.
func TestOps_etag304(t *testing.T) {
	s := newTestServer(t)
	first := doRaw(s, http.MethodGet, "/devices", nil)
	require.Equal(t, http.StatusOK, first.Code)
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	second := doRaw(s, http.MethodGet, "/devices", map[string]string{"If-None-Match": etag})
	require.Equal(t, http.StatusNotModified, second.Code)
}

// Given: a fresh throttled server
// When: POST /wake twice
// Then: rate headers on allow, 429 with Retry-After on deny.
func TestOps_throttleHeaders(t *testing.T) {
	s := newTestServer(t)
	allow := doRequest(s, http.MethodPost, "/wake", nil)
	require.Equal(t, "1", allow.Header().Get("X-RateLimit-Limit"))
	deny := doRequest(s, http.MethodPost, "/wake", nil)
	require.Equal(t, http.StatusTooManyRequests, deny.Code)
	require.NotEmpty(t, deny.Header().Get("Retry-After"))
	require.Equal(t, "0", deny.Header().Get("X-RateLimit-Remaining"))
}

// Given: any request
// When: with, without, and with invalid X-Request-ID
// Then: the response always carries a request id, echoing valid input.
func TestOps_requestID(t *testing.T) {
	s := newTestServer(t)
	minted := doRaw(s, http.MethodGet, "/devices", nil)
	require.NotEmpty(t, minted.Header().Get("X-Request-ID"))
	echoed := doRaw(s, http.MethodGet, "/devices", map[string]string{"X-Request-ID": "ops-test-123"})
	require.Equal(t, "ops-test-123", echoed.Header().Get("X-Request-ID"))
	reminted := doRaw(s, http.MethodGet, "/devices", map[string]string{"X-Request-ID": "bad id"})
	require.NotEmpty(t, reminted.Header().Get("X-Request-ID"))
	require.NotEqual(t, "bad id", reminted.Header().Get("X-Request-ID"))
}

// Given: a wired scanner
// When: POST /discover/adopt with empty hosts
// Then: adopt rate headers are present.
func TestOps_adoptHeaders(t *testing.T) {
	s := newTestServer(t).WithDiscover(discover.New())
	w := doRequest(s, http.MethodPost, "/discover/adopt", map[string]any{"hosts": []any{}})
	require.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	require.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
}
