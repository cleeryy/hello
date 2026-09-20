package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

// listSchedules returns every schedule in one envelope.
func (s *Server) listSchedules(c *gin.Context) {
	if s.schedStore == nil {
		c.JSON(http.StatusOK, gin.H{"schedules": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"schedules": s.schedStore.All()})
}

// createSchedule validates the target device, persists and activates.
func (s *Server) createSchedule(c *gin.Context) {
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
	created, err := s.schedStore.Create(in)
	if err != nil {
		writeScheduleErr(c, err)
		return
	}
	s.sched.Reload()
	c.JSON(http.StatusCreated, created)
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
	c.JSON(http.StatusOK, updated)
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
	case errors.Is(err, models.ErrMissingID),
		errors.Is(err, models.ErrMissingDevice),
		errors.Is(err, models.ErrMissingCron),
		errors.Is(err, models.ErrInvalidCron):
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", err.Error(), nil)
	default:
		writeErr(c, err)
	}
}
