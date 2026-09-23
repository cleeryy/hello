package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
	"github.com/cleeryy/hello/internal/wol"
)

// listSchedules returns every schedule in one envelope, sorted by id and
// enriched with the computed next fire time.
func (s *Server) listSchedules(c *gin.Context) {
	if s.schedStore == nil {
		c.JSON(http.StatusOK, gin.H{"schedules": []any{}})
		return
	}
	all := s.schedStore.All()
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	for i := range all {
		all[i] = enrichSchedule(all[i])
	}
	c.JSON(http.StatusOK, gin.H{"schedules": all})
}

// enrichSchedule attaches the next computed fire time, serve-time only.
func enrichSchedule(sched models.Schedule) models.Schedule {
	if parsed, err := cron.ParseStandard(sched.Cron); err == nil {
		sched.NextRun = parsed.Next(time.Now()).Unix()
	}
	return sched
}

// scheduleInput is the create payload: classic cron fields plus an optional
// RFC3339 once_at turning the row into a one-shot schedule.
type scheduleInput struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	Cron     string `json:"cron"`
	Enabled  bool   `json:"enabled"`
	OnceAt   string `json:"once_at"`
}

// createSchedule validates the target device, persists and activates.
// ?dry_run=1 validates only. once_at builds a one-shot schedule firing
// once at minute precision, then disabling itself.
func (s *Server) createSchedule(c *gin.Context) {
	var in scheduleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, "bad request",
			"body must be a schedule object", nil)
		return
	}
	sched := models.Schedule{
		ID:       strings.TrimSpace(in.ID),
		DeviceID: strings.TrimSpace(in.DeviceID),
		Cron:     strings.TrimSpace(in.Cron),
		Enabled:  in.Enabled,
	}
	if raw := strings.TrimSpace(in.OnceAt); raw != "" {
		at, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				"once_at must be an RFC3339 timestamp", nil)
			return
		}
		if time.Until(at) < time.Minute {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				"once_at must be at least a minute in the future", nil)
			return
		}
		if at.After(time.Now().Add(366 * 24 * time.Hour)) {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				"once_at must be within the next year", nil)
			return
		}
		sched.Once = true
		sched.OnceAt = at.Unix()
		sched.Cron = models.CronForTime(at)
	}
	if _, err := s.store.Get(sched.DeviceID); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("unknown device_id %q", sched.DeviceID), nil)
		return
	}
	if isDryRun(c) {
		sched.Normalize()
		if err := sched.Validate(); err != nil {
			writeScheduleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"dry_run": true, "schedule": enrichSchedule(sched)})
		return
	}
	created, err := s.schedStore.Create(sched)
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusCreated, enrichSchedule(created))
}

// updateSchedule replaces the schedule, keeping the path id.
func (s *Server) updateSchedule(c *gin.Context) {
	id := c.Param("id")
	var in models.Schedule
	if err := c.ShouldBindJSON(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, "bad request",
			"body must be a schedule object", nil)
		return
	}
	if _, err := s.store.Get(in.DeviceID); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("unknown device_id %q", in.DeviceID), nil)
		return
	}
	updated, err := s.schedStore.Update(id, in)
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusOK, enrichSchedule(updated))
}

// pauseSchedules disables every schedule at once and reloads the runner.
func (s *Server) pauseSchedules(c *gin.Context) {
	paused, err := s.schedStore.SetAllEnabled(false)
	if err != nil {
		writeErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusOK, gin.H{"changed": paused})
}

// resumeSchedules re-enables every schedule at once and reloads the runner.
func (s *Server) resumeSchedules(c *gin.Context) {
	resumed, err := s.schedStore.SetAllEnabled(true)
	if err != nil {
		writeErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusOK, gin.H{"changed": resumed})
}

// validateSchedule checks a cron expression without persisting anything.
func (s *Server) validateSchedule(c *gin.Context) {
	var in struct {
		Cron string `json:"cron"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, "bad request",
			"body must be a schedule object", nil)
		return
	}
	parsed, err := cron.ParseStandard(strings.TrimSpace(in.Cron))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "next_run": parsed.Next(time.Now()).Unix()})
}

// duplicateSchedule copies a schedule under a fresh id.
func (s *Server) duplicateSchedule(c *gin.Context) {
	src, err := s.schedStore.Get(c.Param("id"))
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	copy := models.Schedule{DeviceID: src.DeviceID, Cron: src.Cron}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-copy", src.ID)
		if i > 1 {
			candidate = fmt.Sprintf("%s-copy-%d", src.ID, i)
		}
		copy.ID = candidate
		if _, err := s.schedStore.Get(candidate); err != nil {
			break
		}
		if i > 100 {
			writeProblem(c, http.StatusConflict, "conflict",
				"too many copies, rename one first", nil)
			return
		}
	}
	created, err := s.schedStore.Create(copy)
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusCreated, enrichSchedule(created))
}

// fireScheduleNow wakes a schedule target immediately, logging a schedule
// trigger entry and refreshing its run record.
func (s *Server) fireScheduleNow(c *gin.Context) {
	sched, err := s.schedStore.Get(c.Param("id"))
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	dev, err := s.store.Get(sched.DeviceID)
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("unknown device_id %q", sched.DeviceID), nil)
		return
	}
	broadcast := wol.BroadcastForIP(dev.IP, s.cfg.BroadcastIP)
	if err := s.sendWOL(dev.MAC, broadcast); err != nil {
		s.schedStore.MarkFired(sched.ID, false, err.Error())
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"failed to send magic packet", nil)
		return
	}
	if s.hist != nil {
		if _, err := s.hist.Record(models.WakeEvent{
			DeviceID: dev.ID,
			MAC:      dev.MAC,
			Trigger:  models.TriggerSchedule,
			Success:  true,
			Attempts: 1,
		}); err != nil {
			writeErr(c, err)
			return
		}
	}
	if successErr := s.store.RecordWake(dev.ID, time.Now().Unix()); successErr != nil {
		writeErr(c, successErr)
		return
	}
	s.schedStore.MarkFired(sched.ID, true, "")
	s.sched.Reload()
	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("schedule %q fired for device %q", sched.ID, dev.ID),
		"schedule_id": sched.ID,
		"device_id":   dev.ID,
		"success":     true,
	})
}

// deleteSchedule removes a schedule and unloads it from the runner.
func (s *Server) deleteSchedule(c *gin.Context) {
	if err := s.schedStore.Delete(c.Param("id")); err != nil {
		writeScheduleErr(c, err)
		return
	}
	s.sched.Reload()
	c.Status(http.StatusNoContent)
}

func writeScheduleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, scheduler.ErrNotFound):
		writeProblem(c, http.StatusNotFound, "not found", err.Error(), nil)
	case errors.Is(err, scheduler.ErrAlreadyExists):
		writeProblem(c, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, scheduler.ErrTooMany):
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", err.Error(), nil)
	case errors.Is(err, models.ErrMissingID),
		errors.Is(err, models.ErrMissingDevice),
		errors.Is(err, models.ErrMissingCron),
		errors.Is(err, models.ErrMissingOnceAt),
		errors.Is(err, models.ErrPastOnceAt),
		errors.Is(err, models.ErrInvalidCron):
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", err.Error(), nil)
	default:
		writeErr(c, err)
	}
}
