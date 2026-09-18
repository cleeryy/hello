package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/cleeryy/hello/internal/models"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Message is the broadcast envelope sent to subscribers.
type Message struct {
	Type   string        `json:"type"`
	Device models.Device `json:"device"`
}

// Hub fans out device status changes to WebSocket subscribers.
type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan models.Device
	clients    map[*client]struct{}
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub returns a Hub. Call Run before Broadcast.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan models.Device, 64),
		clients:    make(map[*client]struct{}),
	}
}

// Run dispatches registrations and broadcasts until ctx is done.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				close(c.send)
				_ = c.conn.Close()
			}
			return
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				_ = c.conn.Close()
			}
		case d := <-h.broadcast:
			msg := Message{Type: "device.status", Device: d}
			for c := range h.clients {
				select {
				case c.send <- encode(msg):
				default:
					delete(h.clients, c)
					close(c.send)
					_ = c.conn.Close()
				}
			}
		}
	}
}

// Broadcast publishes a device status change to all subscribers.
func (h *Hub) Broadcast(d models.Device) {
	h.broadcast <- d
}

func encode(msg Message) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

// ServeWS upgrades the connection and pumps messages until it closes.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", slog.Any("err", err))
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 16)}
	h.register <- c

	var once sync.Once
	done := make(chan struct{})
	defer func() {
		once.Do(func() { close(done) })
		h.unregister <- c
	}()

	go func() {
		defer once.Do(func() { close(done) })
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case payload, ok := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Handler adapts the hub to gin.
func Handler(h *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		h.ServeWS(c.Writer, c.Request)
	}
}
