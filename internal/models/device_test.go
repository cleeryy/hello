package models_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

func Test_Device_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*models.Device)
		wantErr error
	}{
		{"accepts valid device", func(*models.Device) {}, nil},
		{"rejects empty id", func(d *models.Device) { d.ID = "  " }, models.ErrMissingID},
		{"rejects empty name", func(d *models.Device) { d.Name = "" }, models.ErrMissingName},
		{"rejects bad mac", func(d *models.Device) { d.MAC = "not-a-mac" }, models.ErrInvalidMAC},
		{"rejects bad ip", func(d *models.Device) { d.IP = "999.1.1.1" }, models.ErrInvalidIP},
		{"rejects bad status", func(d *models.Device) { d.Status = "sleeping" }, models.ErrInvalidStat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			d := &models.Device{
				ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
				Status: models.StatusUnknown,
			}
			tt.mutate(d)

			// When
			err := d.Validate()

			// Then
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_Device_Normalize_defaults_status_to_unknown(t *testing.T) {
	// Given
	d := &models.Device{ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55"}

	// When
	d.Normalize()

	// Then
	require.NoError(t, d.Validate())
	assert.Equal(t, models.StatusUnknown, d.Status)
}

func Test_Device_Status_unmarshal(t *testing.T) {
	// Given
	var d models.Device

	// When
	require.NoError(t, json.Unmarshal([]byte(`{"status":"up"}`), &d))

	// Then
	assert.Equal(t, models.StatusUp, d.Status)

	// When
	err := json.Unmarshal([]byte(`{"status":"sleeping"}`), &d)

	// Then
	require.Error(t, err)
}
