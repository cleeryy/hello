package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/storage"
)

// backupVersion tags the envelope so future formats stay distinguishable.
const backupVersion = 1

// backup exports the registry, the wake log, the schedules and the
// discovery ignore list in one versioned envelope for download. Devices
// and schedules are sorted by id and the ignore list is already sorted,
// so two backups of unchanged state are byte-identical.
func (s *Server) backup(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="hello-backup.json"`)
	c.JSON(http.StatusOK, gin.H{
		"version":   backupVersion,
		"devices":   s.backupDevices(),
		"history":   s.backupHistory(),
		"schedules": s.backupSchedules(),
		"ignore":    s.backupIgnore(),
	})
}

func (s *Server) backupDevices() []*models.Device {
	out := s.store.GetAll()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
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
	out := s.schedStore.All()
	if out == nil {
		return []models.Schedule{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Server) backupIgnore() []discover.Entry {
	if s.ignore == nil {
		return []discover.Entry{}
	}
	if out := s.ignore.List(); out != nil {
		return out
	}
	return []discover.Entry{}
}

// restoreInput mirrors the backup envelope; unknown fields are ignored.
type restoreInput struct {
	Devices   []*models.Device   `json:"devices"`
	History   []models.WakeEvent `json:"history"`
	Schedules []models.Schedule  `json:"schedules"`
	Ignore    []discover.Entry   `json:"ignore"`
}

// restore applies a backup envelope atomically: the whole payload is
// validated first (including schedule device references and the history
// capacity bound), then devices, schedules, history and the ignore list
// are applied in order. Any apply failure rolls the already-applied
// stores back to their pre-restore snapshots and reports 500, so the
// registry is never left half-restored. dry_run validates only and never
// mutates. Stores absent from the payload are left untouched.
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
	preparedDevices, err := storage.ValidateDevices(orEmptyDevices(in.Devices))
	if err != nil {
		invalid("devices", err)
		return
	}
	var preparedSchedules []models.Schedule
	if s.schedStore != nil {
		preparedSchedules, err = scheduler.ValidateSchedules(orEmptySchedules(in.Schedules), s.schedStore.Cap())
		if err != nil {
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
	if s.hist != nil && in.History != nil && len(in.History) > s.hist.Cap() {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid history in backup: history: retention quota reached: at most %d entries", s.hist.Cap()), nil)
		return
	}
	if err := s.checkScheduleDevices(preparedDevices, preparedSchedules, in); err != nil {
		invalid("schedules", err)
		return
	}
	if s.ignore == nil && len(in.Ignore) > 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"ignore list is not configured", nil)
		return
	}

	restored := gin.H{}
	if dry := isDryRun(c); dry {
		restored["devices"] = len(in.Devices)
		restored["schedules"] = len(in.Schedules)
		restored["history"] = len(in.History)
		restored["ignore"] = len(in.Ignore)
		c.JSON(http.StatusOK, gin.H{"dry_run": true, "valid": true, "counts": restored})
		return
	}

	// Snapshot every store the payload touches, so a mid-restore failure
	// can roll the already-applied stores back.
	var prevDevices []*models.Device
	var prevSchedules []models.Schedule
	var prevHistory []models.WakeEvent
	var prevIgnore []discover.Entry
	if in.Devices != nil {
		prevDevices = s.store.GetAll()
	}
	if in.Schedules != nil && s.schedStore != nil {
		prevSchedules = s.schedStore.All()
	}
	if in.History != nil && s.hist != nil {
		if size := s.hist.Stats().Size; size > 0 {
			prevHistory = s.hist.List("", size)
		} else {
			prevHistory = []models.WakeEvent{}
		}
	}
	if in.Ignore != nil && s.ignore != nil {
		prevIgnore = s.ignore.List()
	}
	rollback := func() {
		if prevDevices != nil {
			if _, err := s.store.ReplaceAll(prevDevices); err != nil {
				slog.Error("restore rollback failed", slog.String("store", "devices"), slog.Any("err", err))
			}
		}
		if prevSchedules != nil && s.schedStore != nil {
			if _, err := s.schedStore.ReplaceAll(prevSchedules); err != nil {
				slog.Error("restore rollback failed", slog.String("store", "schedules"), slog.Any("err", err))
			}
			if s.sched != nil {
				s.sched.Reload()
			}
		}
		if prevHistory != nil && s.hist != nil {
			if err := s.hist.Restore(prevHistory); err != nil {
				slog.Error("restore rollback failed", slog.String("store", "history"), slog.Any("err", err))
			}
		}
		if prevIgnore != nil && s.ignore != nil {
			if err := s.ignore.ReplaceAll(prevIgnore); err != nil {
				slog.Error("restore rollback failed", slog.String("store", "ignore"), slog.Any("err", err))
			}
		}
	}
	failed := func(store string, err error) {
		rollback()
		writeProblem(c, http.StatusInternalServerError, "restore failed",
			fmt.Sprintf("%s restore failed: %s", store, err.Error()),
			gin.H{"restored": restored})
	}

	applyDevices := s.applyDevices
	if applyDevices == nil {
		applyDevices = s.store.ReplaceAll
	}
	applySchedules := s.applySchedules
	if applySchedules == nil && s.schedStore != nil {
		applySchedules = s.schedStore.ReplaceAll
	}
	applyHistory := s.applyHistory
	if applyHistory == nil && s.hist != nil {
		applyHistory = s.hist.Restore
	}
	applyIgnore := s.applyIgnore
	if applyIgnore == nil && s.ignore != nil {
		applyIgnore = s.ignore.ReplaceAll
	}

	if in.Devices != nil {
		got, err := applyDevices(in.Devices)
		if err != nil {
			failed("devices", err)
			return
		}
		restored["devices"] = len(got)
	}
	if in.Schedules != nil && s.schedStore != nil {
		got, err := applySchedules(in.Schedules)
		if err != nil {
			failed("schedules", err)
			return
		}
		restored["schedules"] = len(got)
		if s.sched != nil {
			s.sched.Reload()
		}
	}
	if in.History != nil && s.hist != nil {
		if err := applyHistory(in.History); err != nil {
			failed("history", err)
			return
		}
		restored["history"] = len(in.History)
	}
	if in.Ignore != nil && s.ignore != nil {
		if err := applyIgnore(in.Ignore); err != nil {
			failed("ignore", err)
			return
		}
		restored["ignore"] = len(in.Ignore)
	}
	c.JSON(http.StatusOK, gin.H{"restored": restored})
}

// checkScheduleDevices rejects schedules targeting devices that will not
// exist after the restore: a schedule may reference a device already in
// the registry or one included in the same restore payload. Schedules are
// only checked when they are present in the payload; otherwise the stored
// set is untouched.
func (s *Server) checkScheduleDevices(preparedDevices []*models.Device, preparedSchedules []models.Schedule, in restoreInput) error {
	if in.Schedules == nil || s.schedStore == nil {
		return nil
	}
	known := make(map[string]struct{}, len(preparedDevices))
	if in.Devices != nil {
		for _, d := range preparedDevices {
			known[d.ID] = struct{}{}
		}
	} else {
		for _, d := range s.store.GetAll() {
			known[d.ID] = struct{}{}
		}
	}
	for _, item := range preparedSchedules {
		if _, ok := known[item.DeviceID]; !ok {
			return fmt.Errorf("schedule %q targets unknown device %q", item.ID, item.DeviceID)
		}
	}
	return nil
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
