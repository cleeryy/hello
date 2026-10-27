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
	require.Contains(t, w.Body.String(), "method:'PATCH'")
	require.Contains(t, w.Body.String(), "impFile")
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

// Given: le dashboard enrichi (batch F-82..F-95)
// When: GET /dashboard
// Then: les marqueurs des nouvelles fonctions sont présents.
func TestDashboard_whenRequested_UXBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterDashboard(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// F-85 import UI + report
	require.Contains(t, body, "/devices/import")
	require.Contains(t, body, "impMode")
	// F-86 collapsible sections persistées
	require.Contains(t, body, "<details")
	require.Contains(t, body, "hello.sec.")
	// F-87 auto-refresh
	require.Contains(t, body, `id="refresh"`)
	require.Contains(t, body, "hello.refresh")
	require.Contains(t, body, "document.hidden")
	// F-88 theme system/light/dark persisté, appliqué avant paint
	require.Contains(t, body, `id="theme"`)
	require.Contains(t, body, "hello.theme")
	require.Contains(t, body, "prefers-color-scheme")
	// F-89 history filters + stats
	require.Contains(t, body, "/history/stats")
	require.Contains(t, body, `id="hTrig"`)
	require.Contains(t, body, "device_id")
	// F-90 device → filtered history link
	require.Contains(t, body, "hDev")
	// F-91 next_run/last_result/once
	require.Contains(t, body, "next_run")
	require.Contains(t, body, "last_result")
	require.Contains(t, body, "pause-all")
	require.Contains(t, body, "resume-all")
	// F-92 clickable tag chips
	require.Contains(t, body, "tagFilter")
	// F-93 footer version + health dot
	require.Contains(t, body, "/health")
	require.Contains(t, body, "/version")
	require.Contains(t, body, "healthDot")
	// F-94 empty states with CTA
	require.Contains(t, body, "Clear filters")
	// F-95 shortcuts
	require.Contains(t, body, "Press /")
	// wake-batch failed[] report
	require.Contains(t, body, "failed")
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
