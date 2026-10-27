package models_test

import (
	"strings"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: a wake event with an overlong note
// When: validated
// Then: the 140-character bound rejects it.
func TestWakeEvent_whenNoteTooLong(t *testing.T) {
	e := models.WakeEvent{
		DeviceID: "pc", MAC: "00:11:22:33:44:55",
		Trigger: models.TriggerManual, Note: strings.Repeat("x", 141),
	}
	require.ErrorContains(t, e.Validate(), "at most 140")

	e.Note = strings.Repeat("x", 140)
	require.NoError(t, e.Validate())
}

// Given: one-shot schedule rows
// When: validated
// Then: missing or past fire times are rejected with typed errors.
func TestSchedule_whenOnceValidated(t *testing.T) {
	missing := models.Schedule{ID: "o", DeviceID: "pc", Cron: "@daily", Once: true}
	require.ErrorIs(t, missing.Validate(), models.ErrMissingOnceAt)

	past := models.Schedule{
		ID: "o", DeviceID: "pc", Cron: "@daily",
		Once: true, OnceAt: time.Now().Add(-time.Hour).Unix(),
	}
	require.ErrorIs(t, past.Validate(), models.ErrPastOnceAt)

	future := models.Schedule{
		ID: "o", DeviceID: "pc", Cron: "@daily",
		Once: true, OnceAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, future.Validate())
}

// Given: a future time
// When: a cron expression is built for it
// Then: the standard parser accepts it.
func TestCronForTime_parses(t *testing.T) {
	expr := models.CronForTime(time.Now().Add(2 * time.Hour))
	_, err := cron.ParseStandard(expr)
	require.NoError(t, err)
}
