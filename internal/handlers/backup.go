package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
)

// backupVersion tags the envelope so future formats stay distinguishable.
const backupVersion = 1

// backup exports the registry, the wake log and the schedules in one
// versioned envelope for download.
func (s *Server) backup(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="hello-backup.json"`)
	c.JSON(http.StatusOK, gin.H{
		"version":     backupVersion,
		"exported_at": time.Now().Unix(),
		"devices":     s.store.GetAll(),
		"history":     s.backupHistory(),
		"schedules":   s.backupSchedules(),
	})
}

func (s *Server) backupHistory() []models.WakeEvent {
	if s.hist == nil {
		return []models.WakeEvent{}
	}
	if size := s.hist.Stats().Size; size > 0 {
		return s.hist.List("", size)
	}
	return []models.WakeEvent{}
}

func (s *Server) backupSchedules() []models.Schedule {
	if s.schedStore == nil {
		return []models.Schedule{}
	}
	if out := s.schedStore.All(); out != nil {
		return out
	}
	return []models.Schedule{}
}

// restoreInput mirrors the backup envelope; unknown fields are ignored.
type restoreInput struct {
	Devices   []*models.Device   `json:"devices"`
	History   []models.WakeEvent `json:"history"`
	Schedules []models.Schedule  `json:"schedules"`
}

// restore validates every present store before touching any file, then
// applies devices, schedules and history in order. Each store write is
// atomic with rollback; a mid-restore failure reports how far it got.
func (s *Server) restore(c *gin.Context) {
	var in restoreInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeBindingError(c, err)
		return
	}
	invalid := func(store string, err error) {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid %s in backup: %s", store, err.Error()), nil)
	}
	if _, err := storage.ValidateDevices(orEmptyDevices(in.Devices)); err != nil {
		invalid("devices", err)
		return
	}
	if s.schedStore != nil {
		if _, err := scheduler.ValidateSchedules(orEmptySchedules(in.Schedules), s.schedStore.Cap()); err != nil {
			invalid("schedules", err)
			return
		}
	} else if len(in.Schedules) > 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"schedules store is not configured", nil)
		return
	}
	if s.hist == nil && len(in.History) > 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"history store is not configured", nil)
		return
	}
	if err := history.ValidateEvents(orEmptyEvents(in.History)); err != nil {
		invalid("history", err)
		return
	}

	restored := gin.H{}
	if dry := isDryRun(c); dry {
		restored["devices"] = len(in.Devices)
		restored["schedules"] = len(in.Schedules)
		restored["history"] = len(in.History)
		c.JSON(http.StatusOK, gin.H{"dry_run": true, "valid": true, "counts": restored})
		return
	}

	failed := func(store string, err error, code int) {
		writeProblem(c, code, "restore failed",
			fmt.Sprintf("%s restore failed: %s", store, err.Error()),
			gin.H{"restored": restored})
	}
	if in.Devices != nil {
		got, err := s.store.ReplaceAll(in.Devices)
		if err != nil {
			failed("devices", err, http.StatusInternalServerError)
			return
		}
		restored["devices"] = len(got)
	}
	if in.Schedules != nil && s.schedStore != nil {
		got, err := s.schedStore.ReplaceAll(in.Schedules)
		if err != nil {
			failed("schedules", err, http.StatusInternalServerError)
			return
		}
		restored["schedules"] = len(got)
		if s.sched != nil {
			s.sched.Reload()
		}
	}
	if in.History != nil && s.hist != nil {
		if err := s.hist.Restore(in.History); err != nil {
			failed("history", err, http.StatusInternalServerError)
			return
		}
		restored["history"] = len(in.History)
	}
	c.JSON(http.StatusOK, gin.H{"restored": restored})
}

func orEmptyDevices(in []*models.Device) []*models.Device {
	if in == nil {
		return []*models.Device{}
	}
	return in
}

func orEmptySchedules(in []models.Schedule) []models.Schedule {
	if in == nil {
		return []models.Schedule{}
	}
	return in
}

func orEmptyEvents(in []models.WakeEvent) []models.WakeEvent {
	if in == nil {
		return []models.WakeEvent{}
	}
	return in
}

// runtimeConfig exposes redacted operational settings. The API token is
// never included; presence flags only say how it was supplied.
func (s *Server) runtimeConfig(c *gin.Context) {
	cfg := s.cfg
	c.JSON(http.StatusOK, gin.H{
		"port":                 cfg.Port,
		"broadcast_ip":         cfg.BroadcastIP,
		"monitor_interval_sec": int(cfg.MonitorInterval.Seconds()),
		"history_cap":          cfg.HistoryCap,
		"schedules_cap":        cfg.SchedulesCap,
		"wake_cooldown_sec":    cfg.WakeCooldownSec,
		"trusted_proxies":      cfg.TrustedProxies,
		"cors_origins":         cfg.CORSOrigins,
		"token_from_file":      cfg.APITokenFile != "",
		"webhooks": gin.H{
			"status": cfg.StatusWebhookURL != "",
			"wake":   cfg.WakeWebhookURL != "",
		},
		"version": Version,
	})
}
