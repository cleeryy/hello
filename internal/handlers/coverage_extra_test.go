package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/history"
)

// Given: a pending one-shot entry
// When: firing it directly
// Then: the packet goes out, history carries the note, the entry is gone.
func TestFireOneshot(t *testing.T) {
	s := newTestServer(t)
	h, err := history.New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)
	s.WithHistory(h)
	var sent []string
	s.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac); return nil }
	s.oneshots.items["o-9"] = &oneshotEntry{MAC: "AA:BB:CC:DD:EE:09", Note: "late"}

	s.fireOneshot("o-9")

	require.Equal(t, []string{"AA:BB:CC:DD:EE:09"}, sent)
	_, found := s.oneshots.items["o-9"]
	require.False(t, found)
	entries := h.List("", 10)
	require.Len(t, entries, 1)
	require.True(t, entries[0].Success)
	require.Equal(t, "late", entries[0].Note)

	// Unknown ids are a silent no-op.
	s.fireOneshot("o-missing")
	require.Len(t, h.List("", 10), 1)
}

// Given: a failing sender
// When: firing a one-shot
// Then: history records the error.
func TestFireOneshotSendFail(t *testing.T) {
	s := newTestServer(t)
	h, err := history.New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)
	s.WithHistory(h)
	s.sendWOL = func(mac, broadcast string) error { return errors.New("no route") }
	s.oneshots.items["o-10"] = &oneshotEntry{MAC: "AA:BB:CC:DD:EE:0A"}

	s.fireOneshot("o-10")

	entries := h.List("", 10)
	require.Len(t, entries, 1)
	require.False(t, entries[0].Success)
	require.Contains(t, entries[0].Error, "no route")
}

// Given: candidate hosts in the adopt body
// When: POSTing /discover/adopt for real
// Then: 201 persists mapped devices; bad batches get 422/409.
func TestAdoptRealHosts(t *testing.T) {
	r := discoverFullRouter(t, 0, "")

	w := doPOST(t, r, "/discover/adopt", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:62","ip":"192.168.9.62","hostname":"lab"}]}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		Devices []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Len(t, created.Devices, 1)
	require.Equal(t, "host-aabbccddee62", created.Devices[0].ID)
	require.Equal(t, "lab", created.Devices[0].Name)

	w = doPOST(t, r, "/discover/adopt", `{"hosts":[{"mac":"not-a-mac","ip":"192.168.9.63"}]}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	w = doPOST(t, r, "/discover/adopt", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:63","ip":"192.168.9.63"},{"mac":"aa:bb:cc:dd:ee:63","ip":"192.168.9.64"}]}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	w = doPOST(t, r, "/discover/adopt", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:62","ip":"192.168.9.62"}]}`)
	require.Equal(t, http.StatusConflict, w.Code)
}

// Given: a finished scan
// When: scanning again at once
// Then: 429 with a Retry-After hint (cooldown branch of runScan).
func TestRunScanCooldown(t *testing.T) {
	r := discoverFullRouter(t, 0, "")

	w := doPOST(t, r, "/discover?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusOK, w.Code)

	w = doPOST(t, r, "/discover?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.NotEmpty(t, w.Header().Get("Retry-After"))
}

// Given: the legacy route helper
// When: mounting through RegisterRoutes
// Then: public routes answer.
func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newStorage(t, t.TempDir()+"/devices.json")
	r := gin.New()
	RegisterRoutes(r, &config.Config{}, store, nil)

	req := doGET(t, r, "/")
	require.Equal(t, http.StatusOK, req.Code)
}
