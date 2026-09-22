package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Given: un stub sendWOL qui enregistre l'appel
// When: POST /wake/:macAddress (canonique, sans effet de bord en GET)
// Then: 200 et le stub a été appelé.
func TestPostWakeMAC(t *testing.T) {
	s := newTestServer(t)
	var gotMAC string
	s.sendWOL = func(mac, broadcast string) error { gotMAC = mac; return nil }

	w := doRequest(s, http.MethodPost, "/wake/AA:BB:CC:DD:EE:FF", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "AA:BB:CC:DD:EE:FF", gotMAC)
}

// Given: une config avec une MAC par défaut
// When: POST /wake
// Then: 200.
func TestPostWakeDefault(t *testing.T) {
	s := newTestServer(t)
	s.cfg.DefaultMAC = "00:11:22:33:44:55"

	w := doRequest(s, http.MethodPost, "/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
}

// Given: une MAC invalide
// When: GET /wake/nope
// Then: 422 problem+json avec title/status/detail.
func TestWakeInvalidMACProblem(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/wake/nope", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	var problem map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	require.Equal(t, float64(422), problem["status"])
	require.NotEmpty(t, problem["title"])
	require.NotEmpty(t, problem["detail"])
}

// Given: un corps invalide
// When: POST /devices
// Then: 422 problem+json.
func TestCreateInvalidProblem(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodPost, "/devices", map[string]string{"id": "x"})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}

// Given: un device existant
// When: DELETE /devices/:id
// Then: 204 corps vide, puis 404 problem+json.
func TestDeleteNoContent(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", map[string]string{
		"id": "pc1", "name": "PC", "mac": "AA:BB:CC:DD:EE:FF",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodDelete, "/devices/pc1", nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	require.Empty(t, w.Body.String())

	w = doRequest(s, http.MethodDelete, "/devices/pc1", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}

// Given: un réveil réussi
// When: POST /wake/:macAddress
// Then: le corps contient message sans champ status redondant.
func TestWakeSuccessShape(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodPost, "/wake/AA:BB:CC:DD:EE:FF", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Contains(t, body, "message")
	require.NotContains(t, body, "status")
}

func TestWake_whenCooldownActive(t *testing.T) {
	s := newTestServer(t)
	s.sendWOL = func(mac, broadcast string) error { return nil }

	first := doRequest(s, http.MethodPost, "/wake/AA:BB:CC:DD:EE:FF", nil)
	second := doRequest(s, http.MethodPost, "/wake/AA:BB:CC:DD:EE:FF", nil)

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Equal(t, "30", second.Header().Get("Retry-After"))
}

// Given: le serveur monté
// When: GET /openapi.yaml puis GET /docs
// Then: 200, YAML contenant openapi: 3.1 et HTML contenant swagger.
func TestDocsAndSpec(t *testing.T) {
	s := newTestServer(t)

	w := doRequest(s, http.MethodGet, "/openapi.yaml", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "yaml")
	require.True(t, strings.Contains(w.Body.String(), "openapi: 3.1"))
	require.Contains(t, w.Body.String(), "/devices")

	w = doRequest(s, http.MethodGet, "/docs", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, strings.ToLower(w.Body.String()), "swagger")
}
