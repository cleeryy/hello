package websocket_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cleeryy/hello/internal/models"
	wshub "github.com/cleeryy/hello/internal/websocket"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	goleak.VerifyTestMain(m)
}

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

func Test_Hub_rejects_cross_origin_websocket(t *testing.T) {
	// Given: a hub with a closed-by-default origin policy.
	hub := wshub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	r := gin.New()
	r.GET("/ws", wshub.Handler(hub))
	srv := httptest.NewServer(r)
	defer srv.Close()

	// When: a browser-like client sends a foreign Origin.
	header := make(map[string][]string)
	header["Origin"] = []string{"https://evil.example.com"}
	_, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/ws"), header)

	// Then: the handshake is rejected.
	require.Error(t, err)
}

func Test_Hub_broadcasts_status_to_subscriber(t *testing.T) {
	// Given
	hub := wshub.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	r := gin.New()
	r.GET("/ws", wshub.Handler(hub))
	srv := httptest.NewServer(r)
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/ws"), nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// When: broadcasts race the async registration, so retry until received.
	var got wshub.Message
	gotOne := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !gotOne {
		hub.Broadcast(models.Device{
			ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55", Status: models.StatusUp,
		})
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if err := conn.ReadJSON(&got); err == nil {
			gotOne = true
		}
	}

	// Then
	require.True(t, gotOne, "no broadcast received within 5s")
	assert.Equal(t, "device.status", got.Type)
	assert.Equal(t, "pc1", got.Device.ID)
	assert.Equal(t, models.StatusUp, got.Device.Status)
}

func Test_Hub_rejects_untrusted_browser_origin(t *testing.T) {
	hub := wshub.NewHub("https://wake.example.com")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	r := gin.New()
	r.GET("/ws", wshub.Handler(hub))
	srv := httptest.NewServer(r)
	defer srv.Close()

	dialer := websocket.DefaultDialer
	dialer.Subprotocols = []string{"device-status"}
	conn, response, err := dialer.Dial(wsURL(srv, "/ws"), http.Header{
		"Origin":                 []string{"https://evil.example.com"},
		"Sec-WebSocket-Protocol": []string{"device-status"},
	})
	if conn != nil {
		_ = conn.Close()
	}
	if response != nil {
		defer func() { _ = response.Body.Close() }()
	}

	require.Error(t, err)
	if assert.NotNil(t, response) {
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
	}
}
