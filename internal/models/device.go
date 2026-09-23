package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Status is the reachability state of a device.
type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown"
)

// IsValid reports whether s is a known status value.
func (s Status) IsValid() bool {
	switch s {
	case StatusUp, StatusDown, StatusUnknown:
		return true
	}
	return false
}

// UnmarshalJSON accepts the known status literals and maps "" to unknown.
func (s *Status) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == "" {
		*s = StatusUnknown
		return nil
	}
	parsed := Status(raw)
	if !parsed.IsValid() {
		return fmt.Errorf("invalid status %q", raw)
	}
	*s = parsed
	return nil
}

var (
	ErrMissingID   = errors.New("models: missing device id")
	ErrMissingName = errors.New("models: missing device name")
	ErrInvalidMAC  = errors.New("models: invalid mac address")
	ErrInvalidIP   = errors.New("models: invalid ip address")
	ErrInvalidStat = errors.New("models: invalid status")
	ErrNotesLong   = errors.New("models: notes too long")
	ErrInvalidTag  = errors.New("models: invalid tag")
	ErrTooManyTags = errors.New("models: too many tags")
)

const (
	// MaxNotesRunes caps the free-text note attached to a device.
	MaxNotesRunes = 500
	// MaxTags caps how many tags a device can carry.
	MaxTags = 10
)

// tagPattern allows lowercase slugs: letters, digits, dash, underscore.
var tagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// Device is the stored representation of a wakeable host.
// JSON field names are part of the on-disk/API contract.
type Device struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MAC         string   `json:"mac"`
	Status      Status   `json:"status"`
	IP          string   `json:"ip,omitempty"`
	PingEnabled bool     `json:"ping_enabled"`
	LastSeen    int64    `json:"last_seen"`
	Notes       string   `json:"notes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	WakeCount   int      `json:"wake_count"`
	LastWakeAt  int64    `json:"last_wake_at,omitempty"`
}

// Normalize fills defaults for absent fields.
func (d *Device) Normalize() {
	d.ID = strings.TrimSpace(d.ID)
	d.Name = strings.TrimSpace(d.Name)
	d.MAC = strings.TrimSpace(d.MAC)
	d.IP = strings.TrimSpace(d.IP)
	d.Notes = strings.TrimSpace(d.Notes)
	if d.Status == "" {
		d.Status = StatusUnknown
	}
	seen := make(map[string]struct{}, len(d.Tags))
	tags := make([]string, 0, len(d.Tags))
	for _, tag := range d.Tags {
		normal := strings.ToLower(strings.TrimSpace(tag))
		if normal == "" {
			continue
		}
		if _, dup := seen[normal]; dup {
			continue
		}
		seen[normal] = struct{}{}
		tags = append(tags, normal)
	}
	d.Tags = tags
	if d.WakeCount < 0 {
		d.WakeCount = 0
	}
	if d.LastWakeAt < 0 {
		d.LastWakeAt = 0
	}
}

// Validate checks the invariants of a device.
// Leading/trailing whitespace is ignored for the checks.
func (d *Device) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return ErrMissingID
	}
	if strings.TrimSpace(d.Name) == "" {
		return ErrMissingName
	}
	if _, err := net.ParseMAC(strings.TrimSpace(d.MAC)); err != nil {
		return fmt.Errorf("mac %q: %w", d.MAC, ErrInvalidMAC)
	}
	if d.IP != "" && net.ParseIP(d.IP) == nil {
		return fmt.Errorf("ip %q: %w", d.IP, ErrInvalidIP)
	}
	if !d.Status.IsValid() {
		return fmt.Errorf("status %q: %w", d.Status, ErrInvalidStat)
	}
	if utf8.RuneCountInString(d.Notes) > MaxNotesRunes {
		return fmt.Errorf("notes: %w", ErrNotesLong)
	}
	if len(d.Tags) > MaxTags {
		return fmt.Errorf("tags: %w", ErrTooManyTags)
	}
	for _, tag := range d.Tags {
		if !tagPattern.MatchString(tag) {
			return fmt.Errorf("tag %q: %w", tag, ErrInvalidTag)
		}
	}
	return nil
}
