package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

func newWakeExtraServer(t *testing.T, sent *[]string) (*Server, *history.History) {
	t.Helper()
	dir := t.TempDir()
	store := newStorage(t, filepath.Join(dir, "devices.json"))
	require.NoError(t, store.Create(&models.Device{
		ID: "pc1", Name: "PC 1", MAC: "00:11:22:33:44:55",
		IP: "192.168.5.7", Status: models.StatusUnknown,
	}))
	h, err := history.New(filepath.Join(dir, "history.json"))
	require.NoError(t, err)
	srv := New(&config.Config{BroadcastIP: "255.255.255.255"}, store, wshub.NewHub())
	srv.WithHistory(h)
	srv.sendWOL = func(mac, broadcast string) error {
		*sent = append(*sent, mac+"@"+broadcast)
		return nil
	}
	return srv, h
}

// Given: a device wake with dry_run
// When: posted
// Then: 200 with the validated target, no packet, no history entry.
func TestWake_whenDryRun(t *testing.T) {
	sent := []string{}
	s, h := newWakeExtraServer(t, &sent)

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake?dry_run=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		DryRun    bool   `json:"dry_run"`
		MAC       string `json:"mac"`
		Broadcast string `json:"broadcast"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.DryRun)
	require.Equal(t, "00:11:22:33:44:55", body.MAC)
	require.NotEmpty(t, body.Broadcast)
	require.Empty(t, sent)
	require.Empty(t, h.List("", 50))
}

// Given: dry_run on an invalid MAC
// When: posted
// Then: 422, validation still applies.
func TestWake_whenDryRunInvalidMAC(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodPost, "/wake/not-a-mac?dry_run=1", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a wake note via query and via JSON body
// When: posted
// Then: the note is stored on the history entry.
func TestWake_whenNoteRecorded(t *testing.T) {
	sent := []string{}
	s, h := newWakeExtraServer(t, &sent)

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake?note=soir", nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest(s, http.MethodPost, "/devices/pc1/wake", map[string]any{"note": "maintenance du soir"})
	require.Equal(t, http.StatusOK, w.Code)

	entries := h.List("", 50)
	require.Len(t, entries, 2)
	require.Equal(t, "maintenance du soir", entries[0].Note)
	require.Equal(t, "soir", entries[1].Note)
}

// Given: a wake note over 140 characters
// When: posted
// Then: 422 and nothing sent.
func TestWake_whenNoteTooLong(t *testing.T) {
	sent := []string{}
	s, _ := newWakeExtraServer(t, &sent)

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake", map[string]any{"note": strings.Repeat("x", 141)})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Empty(t, sent)
}

// Given: a broadcast override
// When: posted
// Then: the packet goes to the override address; invalid override is 422.
func TestWake_whenBroadcastOverride(t *testing.T) {
	sent := []string{}
	s, _ := newWakeExtraServer(t, &sent)

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake?broadcast=10.9.9.255", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"00:11:22:33:44:55@10.9.9.255"}, sent)

	w = doRequest(s, http.MethodPost, "/devices/pc1/wake?broadcast=nope", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a per-device cooldown
// When: the same device is woken twice in a row
// Then: the second wake is 429 with Retry-After; disabling the cooldown reopens.
func TestWake_whenCooldown(t *testing.T) {
	sent := []string{}
	s, _ := newWakeExtraServer(t, &sent)
	s.cfg.WakeCooldownSec = 60

	w := doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	require.Len(t, sent, 1)

	s.cfg.WakeCooldownSec = 0
	w = doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
}
