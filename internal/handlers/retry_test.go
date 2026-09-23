package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
)

func newRetryServer(t *testing.T, ping func(string, time.Duration) bool) (*Server, *history.History) {
	t.Helper()
	s := newTestServer(t)
	h, err := history.New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)
	s.WithHistory(h)
	require.NoError(t, s.store.Create(&models.Device{
		ID: "pc1", Name: "PC 1", MAC: "AA:BB:CC:DD:EE:01", IP: "192.0.2.10", Status: models.StatusUnknown,
	}))
	require.NoError(t, s.store.Create(&models.Device{
		ID: "pc2", Name: "No IP", MAC: "AA:BB:CC:DD:EE:02", Status: models.StatusUnknown,
	}))
	if ping != nil {
		s.pingHost = ping
	}
	return s, h
}

type retryResult struct {
	Message  string `json:"message"`
	Attempts int    `json:"attempts"`
	Up       bool   `json:"up"`
}

// Given: a device answering the first ping
// When: POST /devices/pc1/wake?retries=3&interval=1
// Then: 200 with a single attempt and up=true, no sleep wasted.
func TestRetry_whenHostUpImmediately(t *testing.T) {
	s, h := newRetryServer(t, func(string, time.Duration) bool { return true })

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake?retries=3&interval=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got retryResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 1, got.Attempts)
	require.True(t, got.Up)

	entries := h.List("pc1", 10)
	require.Len(t, entries, 1)
	require.Equal(t, 1, entries[0].Attempts)
}

// Given: a device that never answers
// When: POST /devices/pc1/wake?retries=2&interval=1
// Then: 200 with 3 attempts and up=false, one history entry.
func TestRetry_whenHostNeverUp(t *testing.T) {
	s, h := newRetryServer(t, func(string, time.Duration) bool { return false })

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake?retries=2&interval=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got retryResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 3, got.Attempts)
	require.False(t, got.Up)

	entries := h.List("pc1", 10)
	require.Len(t, entries, 1)
	require.Equal(t, 3, entries[0].Attempts)
	require.True(t, entries[0].Success)
}

// Given: a device without IP
// When: POST /devices/pc2/wake?retries=1
// Then: 422, reachability cannot be checked.
func TestRetry_whenNoIP(t *testing.T) {
	s, _ := newRetryServer(t, func(string, time.Duration) bool { return true })

	w := doRequest(s, http.MethodPost, "/devices/pc2/wake?retries=1", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: out-of-range retry parameters
// When: POST with each invalid combination
// Then: 422 every time.
func TestRetry_whenParamsInvalid(t *testing.T) {
	s, _ := newRetryServer(t, func(string, time.Duration) bool { return true })

	for _, q := range []string{
		"retries=9", "retries=-1", "retries=lots",
		"interval=0", "interval=9", "interval=fast",
		"retries=5&interval=5",
	} {
		w := doRequest(s, http.MethodPost, "/devices/pc1/wake?"+q, nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, q)
	}
}

// Given: a plain wake without retry params
// When: POST /devices/pc1/wake
// Then: legacy shape, message only, no attempts key.
func TestRetry_whenLegacyPath(t *testing.T) {
	s, _ := newRetryServer(t, func(string, time.Duration) bool { return true })

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "message")
	require.NotContains(t, w.Body.String(), "attempts")
}
