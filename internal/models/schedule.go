package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	ErrMissingDevice = errors.New("device_id is required")
	ErrMissingCron   = errors.New("cron is required")
	ErrInvalidCron   = errors.New("cron expression is invalid")
	ErrMissingOnceAt = errors.New("once_at is required for one-shot schedules")
	ErrPastOnceAt    = errors.New("once_at must be in the future")
)

// Schedule fires a wake for one registry device on a cron timetable.
// A one-shot schedule (Once) fires once at OnceAt, then disables itself.
// LastRunAt/LastResult record the latest fire; NextRun is computed at
// serve time and never persisted.
type Schedule struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	Cron     string `json:"cron"`
	Enabled  bool   `json:"enabled"`
	// Once marks a run-once schedule firing at OnceAt (unix seconds).
	Once   bool  `json:"once,omitempty"`
	OnceAt int64 `json:"once_at,omitempty"`
	// LastRunAt and LastResult record the latest fire (unix seconds).
	LastRunAt  int64  `json:"last_run_at,omitempty"`
	LastResult string `json:"last_result,omitempty"`
	// NextRun is the next computed fire time, serve-time only.
	NextRun int64 `json:"next_run,omitempty"`
}

// Normalize trims padding and enables the schedule by default.
func (s *Schedule) Normalize() {
	s.ID = strings.TrimSpace(s.ID)
	s.DeviceID = strings.TrimSpace(s.DeviceID)
	s.Cron = strings.TrimSpace(s.Cron)
}

// Validate rejects missing fields, unparsable cron expressions, and
// one-shot schedules without a future fire time.
func (s *Schedule) Validate() error {
	switch {
	case s.ID == "":
		return ErrMissingID
	case s.DeviceID == "":
		return ErrMissingDevice
	}
	if s.Once {
		if s.OnceAt == 0 {
			return ErrMissingOnceAt
		}
		if s.OnceAt <= time.Now().Unix() {
			return ErrPastOnceAt
		}
		if s.Cron == "" {
			return ErrMissingCron
		}
	} else if s.Cron == "" {
		return ErrMissingCron
	}
	if _, err := cron.ParseStandard(s.Cron); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidCron, s.Cron)
	}
	return nil
}

// CronForTime builds a yearly cron expression matching t at minute precision.
func CronForTime(t time.Time) string {
	return fmt.Sprintf("%d %d %d %d *", t.Minute(), t.Hour(), t.Day(), int(t.Month()))
}
