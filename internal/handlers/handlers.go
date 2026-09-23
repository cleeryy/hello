package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
	wshub "github.com/cleeryy/hello/internal/websocket"
	"github.com/cleeryy/hello/internal/wol"
)

// Server wires routes to the registry and the realtime hub.
type Server struct {
	cfg        *config.Config
	store      *storage.Storage
	hub        *wshub.Hub
	hist       *history.History
	schedStore *scheduler.Store
	sched      *scheduler.Scheduler
	disc       *discover.Scanner
	sendWOL    func(mac, broadcast string) error
	limiter    *requestLimiter
	adoptLimit *rateLimiter
	// pingHost checks reachability for retry-until-up; stubbed in tests.
	pingHost func(ip string, timeout time.Duration) bool
	started  time.Time
	// oneshots holds pending delayed wakes (in-memory, ephemeral).
	oneshots *oneshotRegistry
}

// New returns a Server sending magic packets via wol.SendWOLPacket.
func New(cfg *config.Config, store *storage.Storage, hub *wshub.Hub) *Server {
	return &Server{
		cfg:        cfg,
		store:      store,
		hub:        hub,
		sendWOL:    wol.SendWOLPacket,
		limiter:    newRequestLimiter(wakeDiscoverCooldown),
		adoptLimit: newRateLimiter(5, time.Minute),
		pingHost:   ping.PingHost,
		started:    time.Now(),
		oneshots:   newOneshotRegistry(),
	}
}

// WithHistory wires the wake log; nil keeps the server running without one.
func (s *Server) WithHistory(h *history.History) *Server {
	s.hist = h
	return s
}

// WithSchedules wires the schedule store and its runner; without it the
// schedule routes stay unregistered and POST wakes remain manual only.
func (s *Server) WithSchedules(store *scheduler.Store, sched *scheduler.Scheduler) *Server {
	s.schedStore = store
	s.sched = sched
	return s
}

// WithDiscover wires the LAN scanner; without it the discover routes stay
// unregistered and onboarding stays manual.
func (s *Server) WithDiscover(d *discover.Scanner) *Server {
	s.disc = d
	return s
}

// RegisterRoutes mounts the API, preserving the legacy flat paths.
func RegisterRoutes(r *gin.Engine, cfg *config.Config, store *storage.Storage, hub *wshub.Hub) {
	New(cfg, store, hub).Mount(r)
}

// Mount registers every route. Docs, welcome, and health stay public;
// everything else requires the API token when one is configured.
func (s *Server) Mount(r *gin.Engine) {
	r.Use(gin.Recovery(), securityHeaders(), requestBodyLimit())
	if origins, err := s.cfg.ParseCORSOrigins(); err == nil {
		r.Use(corsMiddleware(origins))
	}

	r.GET("/", s.welcome)
	r.GET("/health", s.health)
	r.GET("/docs", s.docs)
	r.GET("/openapi.yaml", s.openAPI)

	guarded := r.Group("/", requireToken(s.cfg.APIToken))
	guarded.GET("/ws", wshub.Handler(s.hub))

	throttled := guarded.Group("/", throttle(s.limiter))
	throttled.POST("/wake", s.wakeDefault)
	throttled.GET("/wake", s.wakeDefault)
	throttled.POST("/wake/:macAddress", s.wakeMAC)
	throttled.GET("/wake/:macAddress", s.wakeMAC)
	throttled.POST("/discover", s.scanNetwork)
	throttled.POST("/devices/wake-batch", s.wakeBatch)
	throttled.POST("/wake/oneshots", s.createOneshot)

	guarded.GET("/devices", s.listDevices)
	guarded.POST("/devices", s.createDevice)
	guarded.DELETE("/devices", s.bulkDeleteDevices)
	guarded.GET("/devices/counts", s.deviceCounts)
	guarded.GET("/devices/export", s.exportDevices)
	guarded.POST("/devices/import", s.importDevices)
	guarded.GET("/devices/:id", s.getDevice)
	guarded.PUT("/devices/:id", s.updateDevice)
	guarded.PATCH("/devices/:id", s.patchDevice)
	guarded.DELETE("/devices/:id", s.deleteDevice)
	guarded.POST("/devices/:id/wake", s.wakeDevice)
	guarded.POST("/devices/:id/clone", s.cloneDevice)
	guarded.GET("/history", s.listHistory)
	guarded.GET("/history/stats", s.historyStats)
	guarded.GET("/history/export", s.exportHistory)
	guarded.DELETE("/history", s.purgeHistory)
	guarded.GET("/metrics", s.metrics)
	guarded.GET("/wake/oneshots", s.listOneshots)
	guarded.DELETE("/wake/oneshots/:id", s.cancelOneshot)
	if s.schedStore != nil {
		guarded.GET("/schedules", s.listSchedules)
		guarded.POST("/schedules", s.createSchedule)
		guarded.POST("/schedules/validate", s.validateSchedule)
		guarded.POST("/schedules/pause-all", s.pauseSchedules)
		guarded.POST("/schedules/resume-all", s.resumeSchedules)
		guarded.PUT("/schedules/:id", s.updateSchedule)
		guarded.DELETE("/schedules/:id", s.deleteSchedule)
		guarded.POST("/schedules/:id/duplicate", s.duplicateSchedule)
		guarded.POST("/schedules/:id/fire", s.fireScheduleNow)
	}
	if s.disc != nil {
		guarded.POST("/discover/adopt", s.adoptHosts)
	}
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
