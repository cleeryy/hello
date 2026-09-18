package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/storage"
	wshub "github.com/cleeryy/hello/internal/websocket"
	"github.com/cleeryy/hello/internal/wol"
)

// Server wires routes to the registry and the realtime hub.
type Server struct {
	cfg     *config.Config
	store   *storage.Storage
	hub     *wshub.Hub
	sendWOL func(mac, broadcast string) error
}

// New returns a Server sending magic packets via wol.SendWOLPacket.
func New(cfg *config.Config, store *storage.Storage, hub *wshub.Hub) *Server {
	return &Server{cfg: cfg, store: store, hub: hub, sendWOL: wol.SendWOLPacket}
}

// RegisterRoutes mounts the API, preserving the legacy flat paths.
func RegisterRoutes(r *gin.Engine, cfg *config.Config, store *storage.Storage, hub *wshub.Hub) {
	New(cfg, store, hub).Mount(r)
}

// Mount registers every route.
func (s *Server) Mount(r *gin.Engine) {
	r.Use(securityHeaders())
	r.GET("/", s.welcome)
	r.GET("/health", s.health)
	r.GET("/docs", s.docs)
	r.GET("/openapi.yaml", s.openAPI)
	r.GET("/ws", wshub.Handler(s.hub))
	r.POST("/wake", s.wakeDefault)
	r.GET("/wake", s.wakeDefault)
	r.POST("/wake/:macAddress", s.wakeMAC)
	r.GET("/wake/:macAddress", s.wakeMAC)
	r.GET("/devices", s.listDevices)
	r.POST("/devices", s.createDevice)
	r.GET("/devices/:id", s.getDevice)
	r.PUT("/devices/:id", s.updateDevice)
	r.DELETE("/devices/:id", s.deleteDevice)
	r.POST("/devices/:id/wake", s.wakeDevice)
}

func (s *Server) welcome(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":    "welcome to the hello api!",
		"defaultMac": s.cfg.DefaultMAC,
	})
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

const problemContentType = "application/problem+json"

func writeProblem(c *gin.Context, status int, title, detail string, extra gin.H) {
	body := gin.H{
		"type":   "about:blank",
		"title":  title,
		"status": status,
		"detail": detail,
	}
	for k, v := range extra {
		body[k] = v
	}
	c.Header("Content-Type", problemContentType)
	c.JSON(status, body)
}

func writeBindingError(c *gin.Context, err error) {
	var vErr validator.ValidationErrors
	if errors.As(err, &vErr) {
		out := make(map[string]string, len(vErr))
		for _, fe := range vErr {
			out[fe.Field()] = fe.Tag()
		}
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"request validation failed", gin.H{"errors": out})
		return
	}
	writeProblem(c, http.StatusBadRequest, "bad request", "malformed json body", nil)
}

// writeErr maps domain and storage errors to HTTP responses.
func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeProblem(c, http.StatusNotFound, "not found", err.Error(), nil)
	case errors.Is(err, storage.ErrAlreadyExists):
		writeProblem(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, models.ErrMissingID),
		errors.Is(err, models.ErrMissingName),
		errors.Is(err, models.ErrInvalidMAC),
		errors.Is(err, models.ErrInvalidIP),
		errors.Is(err, models.ErrInvalidStat),
		errors.Is(err, wol.ErrInvalidMAC):
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", err.Error(), nil)
	default:
		slog.Error("unhandled handler error", slog.Any("err", err))
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"something went wrong", nil)
	}
}
