package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

var (
	ErrMissingDevice = errors.New("device_id is required")
	ErrMissingCron   = errors.New("cron is required")
	ErrInvalidCron   = errors.New("cron expression is invalid")
)

// Schedule fires a wake for one registry device on a cron timetable.
type Schedule struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	Cron     string `json:"cron"`
	Enabled  bool   `json:"enabled"`
}

// Normalize trims padding and enables the schedule by default.
func (s *Schedule) Normalize() {
	s.ID = strings.TrimSpace(s.ID)
	s.DeviceID = strings.TrimSpace(s.DeviceID)
	s.Cron = strings.TrimSpace(s.Cron)
}

// Validate rejects missing fields and unparsable cron expressions.
func (s *Schedule) Validate() error {
	switch {
	case s.ID == "":
		return ErrMissingID
	case s.DeviceID == "":
		return ErrMissingDevice
	case s.Cron == "":
		return ErrMissingCron
	}
	if _, err := cron.ParseStandard(s.Cron); err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidCron, s.Cron)
	}
	return nil
}
