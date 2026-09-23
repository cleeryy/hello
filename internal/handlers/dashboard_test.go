package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Given: le dashboard enregistré comme dans main
// When: GET /dashboard
// Then: 200 HTML Tailwind avec le titre produit.
func TestDashboard_whenRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterDashboard(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "cdn.tailwindcss.com")
	require.Contains(t, w.Body.String(), "<title>hello</title>")
	require.Contains(t, w.Body.String(), "Edit device")
	require.Contains(t, w.Body.String(), "method:'PUT'")
	require.Contains(t, w.Body.String(), "Discover")
	require.Contains(t, w.Body.String(), "Scan the LAN")
	require.Contains(t, w.Body.String(), "Wake history")
	require.Contains(t, w.Body.String(), "/history?limit=")
	require.Contains(t, w.Body.String(), "Schedules")
	require.Contains(t, w.Body.String(), "/schedules")
	require.Contains(t, w.Body.String(), "/manifest.json")
	require.Contains(t, w.Body.String(), "/devices/wake-batch")
	require.Contains(t, w.Body.String(), "Wake all down")
	require.Contains(t, w.Body.String(), "/devices/export")
	require.Contains(t, w.Body.String(), "per_page=500")
	require.Contains(t, w.Body.String(), "eNotes")
}

// Given: le manifest PWA enregistré comme dans main
// When: GET /manifest.json
// Then: 200 JSON installable pointant sur /dashboard.
func TestManifest_whenRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterDashboard(r)

	req := httptest.NewRequest(http.MethodGet, "/manifest.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/manifest+json")
	require.Contains(t, w.Body.String(), `"start_url":"/dashboard"`)
	require.Contains(t, w.Body.String(), `"name":"hello"`)
}
