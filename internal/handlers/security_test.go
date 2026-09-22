package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

// TestSecurityHeaders_whenHealthRequested verifies S8:
// every response carries baseline security headers.
func TestSecurityHeaders_whenHealthRequested(t *testing.T) {
	// Given: a mounted server.
	gin.SetMode(gin.TestMode)
	store := newStorage(t, t.TempDir()+"/devices.json")
	srv := New(&config.Config{DefaultMAC: "AA:BB:CC:DD:EE:FF", BroadcastIP: "255.255.255.255"}, store, wshub.NewHub())
	engine := gin.New()
	srv.Mount(engine)

	// When: a client requests the health endpoint.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: baseline security headers are present.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", res.Header.Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", res.Header.Get("Referrer-Policy"))
	require.Contains(t, res.Header.Get("Content-Security-Policy"), "default-src 'self'")
	require.Equal(t, "max-age=31536000; includeSubDomains", res.Header.Get("Strict-Transport-Security"))
}

func TestCORS_whenExplicitOriginAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newStorage(t, t.TempDir()+"/devices.json")
	srv := New(&config.Config{
		DefaultMAC:  "AA:BB:CC:DD:EE:FF",
		BroadcastIP: "255.255.255.255",
		CORSOrigins: "https://wake.example.com",
	}, store, wshub.NewHub())
	r := gin.New()
	srv.Mount(r)

	req := httptest.NewRequest(http.MethodOptions, "/devices", nil)
	req.Header.Set("Origin", "https://wake.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "https://wake.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_whenOriginNotWhitelisted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newStorage(t, t.TempDir()+"/devices.json")
	srv := New(&config.Config{
		DefaultMAC:  "AA:BB:CC:DD:EE:FF",
		BroadcastIP: "255.255.255.255",
		CORSOrigins: "https://wake.example.com",
	}, store, wshub.NewHub())
	r := gin.New()
	srv.Mount(r)

	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
