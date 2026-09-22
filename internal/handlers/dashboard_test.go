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
}
