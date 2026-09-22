package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	goleak.VerifyTestMain(m)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devices.json")
	store := newStorage(t, path)
	s := New(&config.Config{}, store, nil)
	s.sendWOL = func(mac, broadcast string) error { return nil }
	return s
}

func newStorage(t *testing.T, path string) *storage.Storage {
	t.Helper()
	store, err := storage.New(path)
	require.NoError(t, err)
	return store
}

func newScheduleStore(t *testing.T, path string) *scheduler.Store {
	t.Helper()
	store, err := scheduler.NewStore(path)
	require.NoError(t, err)
	return store
}

func doRequest(s *Server, method, target string, body any) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine := gin.New()
	s.Mount(engine)
	engine.ServeHTTP(w, req)
	return w
}

// Given: un serveur frais
// When: GET / puis GET /health
// Then: 200 + payloads attendus.
func TestIndexAndHealth(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "welcome to the hello api!")

	w = doRequest(s, http.MethodGet, "/health", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var health map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &health))
	require.Equal(t, "ok", health["status"])
}

// Given: aucune donnée
// When: GET /wake/:macAddress avec une MAC invalide
// Then: 422.
func TestWakeInvalidMAC(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodGet, "/wake/not-a-mac", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: un stub sendWOL qui enregistre l'appel
// When: GET /wake/:macAddress avec une MAC valide
// Then: 200 et le stub a été appelé.
func TestWakeValidMAC(t *testing.T) {
	s := newTestServer(t)
	var gotMAC string
	s.sendWOL = func(mac, broadcast string) error { gotMAC = mac; return nil }

	w := doRequest(s, http.MethodGet, "/wake/AA:BB:CC:DD:EE:FF", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "AA:BB:CC:DD:EE:FF", gotMAC)
}

// Given: un store vide
// When: cycle CRUD complet + cas d'erreur
// Then: 201/200/409/422/404/204 selon le cas.
func TestDevicesCRUD(t *testing.T) {
	s := newTestServer(t)
	dev := models.Device{ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.1.10"}

	// Create 201.
	w := doRequest(s, http.MethodPost, "/devices", dev)
	require.Equal(t, http.StatusCreated, w.Code)

	// List contient le device.
	w = doRequest(s, http.MethodGet, "/devices", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Devices []models.Device `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Devices, 1)

	// Duplicate 409.
	w = doRequest(s, http.MethodPost, "/devices", dev)
	require.Equal(t, http.StatusConflict, w.Code)

	// Corps invalide 422.
	w = doRequest(s, http.MethodPost, "/devices", map[string]string{"id": "x"})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// Update 200 + nom persisté.
	dev.Name = "PC Salon"
	w = doRequest(s, http.MethodPut, "/devices/pc1", dev)
	require.Equal(t, http.StatusOK, w.Code)
	var updated models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, "PC Salon", updated.Name)

	// Update inexistant 404.
	w = doRequest(s, http.MethodPut, "/devices/nope", dev)
	require.Equal(t, http.StatusNotFound, w.Code)

	// Wake-by-ID 200 via le stub.
	var woke bool
	s.sendWOL = func(mac, broadcast string) error { woke = true; return nil }
	w = doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, woke)

	// Wake-by-ID inexistant 404.
	w = doRequest(s, http.MethodPost, "/devices/nope/wake", nil)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = doRequest(s, http.MethodDelete, "/devices/pc1", nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	w = doRequest(s, http.MethodDelete, "/devices/pc1", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}
