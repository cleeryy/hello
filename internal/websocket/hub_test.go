package websocket_test

import (
	"context"
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
	defer conn.Close()

	// When
	hub.Broadcast(models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55", Status: models.StatusUp,
	})

	// Then
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var got wshub.Message
	require.NoError(t, conn.ReadJSON(&got))
	assert.Equal(t, "device.status", got.Type)
	assert.Equal(t, "pc1", got.Device.ID)
	assert.Equal(t, models.StatusUp, got.Device.Status)
}
