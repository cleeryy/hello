package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/storage"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

// TestSecurityHeaders_whenHealthRequested verifies S8:
// every response carries baseline security headers.
func TestSecurityHeaders_whenHealthRequested(t *testing.T) {
	// Given: a mounted server.
	gin.SetMode(gin.TestMode)
	store := storage.New(t.TempDir() + "/devices.json")
	srv := New(&config.Config{DefaultMAC: "AA:BB:CC:DD:EE:FF", BroadcastIP: "255.255.255.255"}, store, wshub.NewHub())
	engine := gin.New()
	srv.Mount(engine)

	// When: a client requests the health endpoint.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: baseline security headers are present.
	res := rec.Result()
	defer res.Body.Close()
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", res.Header.Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", res.Header.Get("Referrer-Policy"))
}
