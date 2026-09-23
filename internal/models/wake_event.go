package models

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Trigger names what caused a wake: a human via the API or a schedule firing.
type Trigger string

const (
	TriggerManual   Trigger = "manual"
	TriggerSchedule Trigger = "schedule"
)

// IsValid reports whether t is a known trigger value.
func (t Trigger) IsValid() bool {
	switch t {
	case TriggerManual, TriggerSchedule:
		return true
	}
	return false
}

// ErrInvalidTrigger is returned for trigger values outside the known set.
var ErrInvalidTrigger = errors.New("models: invalid trigger")

// maxNoteRunes bounds the free-text note attached to a wake.
const maxNoteRunes = 140

// WakeEvent is one sent magic packet, success or failure.
// JSON field names are part of the on-disk/API contract.
type WakeEvent struct {
	ID       string  `json:"id"`
	DeviceID string  `json:"device_id"`
	MAC      string  `json:"mac"`
	At       int64   `json:"at"`
	Trigger  Trigger `json:"trigger"`
	Success  bool    `json:"success"`
	// Attempts counts sent packets for this entry (retry-until-up).
	Attempts int `json:"attempts,omitempty"`
	// Note is an optional human annotation recorded with the wake.
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
}

// Validate checks the invariants of a wake event.
func (e *WakeEvent) Validate() error {
	if strings.TrimSpace(e.DeviceID) == "" {
		return ErrMissingID
	}
	if _, err := net.ParseMAC(strings.TrimSpace(e.MAC)); err != nil {
		return fmt.Errorf("mac %q: %w", e.MAC, ErrInvalidMAC)
	}
	if !e.Trigger.IsValid() {
		return fmt.Errorf("trigger %q: %w", e.Trigger, ErrInvalidTrigger)
	}
	if len([]rune(e.Note)) > maxNoteRunes {
		return fmt.Errorf("note must be at most %d characters", maxNoteRunes)
	}
	return nil
}
