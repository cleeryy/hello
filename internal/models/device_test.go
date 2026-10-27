package models_test

import (
	"encoding/json"
	"strings"
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

func Test_Device_Notes_validation(t *testing.T) {
	// Given
	d := &models.Device{ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55", Notes: "short note"}

	// When / Then
	d.Normalize()
	require.NoError(t, d.Validate())

	// When
	d.Notes = strings.Repeat("n", models.MaxNotesRunes+1)

	// Then
	require.ErrorIs(t, d.Validate(), models.ErrNotesLong)
}

func Test_Device_Tags_normalize_and_validate(t *testing.T) {
	// Given
	d := &models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Tags: []string{"Lab", " lab ", "", "MEDIA", "media"},
	}

	// When
	d.Normalize()

	// Then
	require.NoError(t, d.Validate())
	assert.Equal(t, []string{"lab", "media"}, d.Tags)

	// When: too many tags
	d.Tags = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}

	// Then
	require.ErrorIs(t, d.Validate(), models.ErrTooManyTags)

	// When: invalid tag shape
	d.Tags = []string{"UP PER"}

	// Then
	require.ErrorIs(t, d.Validate(), models.ErrInvalidTag)
}
