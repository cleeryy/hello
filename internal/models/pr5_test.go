package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

func validPR5Device() models.Device {
	return models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF", Status: models.StatusUnknown,
	}
}

// Given: per-device monitor intervals
// When: validated
// Then: 0 means global, 5..86400 passes, anything else ErrInvalidMon.
func TestDevice_monitorSecs(t *testing.T) {
	d := validPR5Device()
	d.MonitorSecs = 0
	require.NoError(t, d.Validate())

	d.MonitorSecs = 60
	require.NoError(t, d.Validate())

	for _, secs := range []int{1, 4, -1, 86401} {
		d.MonitorSecs = secs
		require.ErrorIs(t, d.Validate(), models.ErrInvalidMon, "secs=%d", secs)
	}
}
