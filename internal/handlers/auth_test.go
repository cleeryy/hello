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

func mountWithToken(t *testing.T, token string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := storage.New(t.TempDir() + "/devices.json")
	srv := New(&config.Config{
		DefaultMAC:  "AA:BB:CC:DD:EE:FF",
		BroadcastIP: "255.255.255.255",
		APIToken:    token,
	}, store, wshub.NewHub())
	engine := gin.New()
	srv.Mount(engine)
	return engine
}

// TestAuth_whenTokenConfiguredAndMissing verifies S11-issue-11:
// protected routes reject anonymous callers with 401 problem+json.
func TestAuth_whenTokenConfiguredAndMissing(t *testing.T) {
	// Given: a server locked with an API token.
	engine := mountWithToken(t, "s3cret")

	// When: an anonymous caller hits a protected route.
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: 401 with the RFC 9457 envelope.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, "application/problem+json", res.Header.Get("Content-Type"))
}

// TestAuth_whenTokenConfiguredAndValid verifies a correct Bearer token opens the route.
func TestAuth_whenTokenConfiguredAndValid(t *testing.T) {
	// Given: a server locked with an API token.
	engine := mountWithToken(t, "s3cret")

	// When: the caller presents the token.
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: the route answers normally.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
}

// TestAuth_whenTokenConfiguredButWrong verifies a bad token is rejected.
func TestAuth_whenTokenConfiguredButWrong(t *testing.T) {
	// Given: a server locked with an API token.
	engine := mountWithToken(t, "s3cret")

	// When: the caller presents a wrong token.
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: 401.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

// TestAuth_whenNoTokenConfigured verifies open mode stays fully public.
func TestAuth_whenNoTokenConfigured(t *testing.T) {
	// Given: a server without token (local dev).
	engine := mountWithToken(t, "")

	// When: an anonymous caller hits a protected route.
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: still open.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
}

// TestAuth_whenHealthRequested verifies /health stays public even when locked.
func TestAuth_whenHealthRequested(t *testing.T) {
	// Given: a locked server.
	engine := mountWithToken(t, "s3cret")

	// When: a load-balancer polls health anonymously.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	// Then: 200, no token needed.
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)
}
