package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

func TestScheduleValidate_whenComplete(t *testing.T) {
	// Given: a fully populated schedule
	s := models.Schedule{ID: "morning", DeviceID: "pc-salon", Cron: "0 7 * * 1-5", Enabled: true}
	// When: validated
	// Then: accepted
	require.NoError(t, s.Validate())
}

func TestScheduleValidate_whenMissingFields(t *testing.T) {
	// Given: schedules lacking one required field each
	// When: validated
	// Then: each names its missing field
	cases := []struct {
		name string
		s    models.Schedule
		err  error
	}{
		{"no id", models.Schedule{DeviceID: "pc", Cron: "0 7 * * *"}, models.ErrMissingID},
		{"no device", models.Schedule{ID: "s", Cron: "0 7 * * *"}, models.ErrMissingDevice},
		{"no cron", models.Schedule{ID: "s", DeviceID: "pc"}, models.ErrMissingCron},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.s.Validate(), tc.err)
		})
	}
}

func TestScheduleValidate_whenBadCron(t *testing.T) {
	// Given: schedules with unparsable or nonstandard cron expressions
	// When: validated
	// Then: invalid cron rejected with the offending value wrapped
	cases := []struct {
		name string
		cron string
	}{
		{"garbage", "not a cron"},
		{"four fields", "0 7 * *"},
		{"six fields standard", "0 0 7 * * *"},
		{"out of range", "99 99 * * *"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := models.Schedule{ID: "s", DeviceID: "pc", Cron: tc.cron}
			err := s.Validate()
			require.ErrorIs(t, err, models.ErrInvalidCron)
			require.ErrorContains(t, err, tc.cron)
		})
	}
}

func TestScheduleValidate_whenDescriptors(t *testing.T) {
	// Given: a schedule using @daily shorthand and whitespace padding
	s := models.Schedule{ID: "s", DeviceID: "pc", Cron: "  @daily "}
	s.Normalize()
	// When: validated
	// Then: descriptors and padding accepted
	require.NoError(t, s.Validate())
}
