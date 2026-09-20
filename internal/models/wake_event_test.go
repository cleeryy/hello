package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWakeEvent_whenValid verifies a complete wake record passes validation.
func TestWakeEvent_whenValid(t *testing.T) {
	// Given: a manual wake of a known device.
	e := WakeEvent{
		DeviceID: "pc-salon",
		MAC:      "00:11:22:33:44:55",
		At:       1758210000,
		Trigger:  TriggerManual,
		Success:  true,
	}

	// When: validated.
	// Then: no error.
	require.NoError(t, e.Validate())
}

// TestWakeEvent_whenTriggerUnknown verifies the trigger discriminate is exhaustive.
func TestWakeEvent_whenTriggerUnknown(t *testing.T) {
	// Given: an event with a trigger outside the known set.
	e := WakeEvent{
		DeviceID: "pc-salon",
		MAC:      "00:11:22:33:44:55",
		At:       1758210000,
		Trigger:  "cron",
		Success:  true,
	}

	// When: validated.
	// Then: ErrInvalidTrigger.
	require.ErrorIs(t, e.Validate(), ErrInvalidTrigger)
}

// TestWakeEvent_whenMissingFields verifies required fields are enforced.
func TestWakeEvent_whenMissingFields(t *testing.T) {
	// Given: events missing device id, then MAC.
	noID := WakeEvent{MAC: "00:11:22:33:44:55", Trigger: TriggerManual}
	noMAC := WakeEvent{DeviceID: "pc-salon", Trigger: TriggerManual}

	// When: validated.
	// Then: the matching sentinel errors.
	require.ErrorIs(t, noID.Validate(), ErrMissingID)
	require.ErrorIs(t, noMAC.Validate(), ErrInvalidMAC)
}
