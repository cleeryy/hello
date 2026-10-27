package history

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/models"
)

// Given: a history with capacity 2
// When: three wakes are recorded
// Then: only the two newest survive.
func TestHistory_whenCapacityBounds(t *testing.T) {
	h, err := NewWithCapacity(filepath.Join(t.TempDir(), "hist.json"), 2)
	require.NoError(t, err)
	for _, id := range []string{"a", "b", "c"} {
		_, err := h.Record(models.WakeEvent{
			DeviceID: id, MAC: "00:11:22:33:44:55",
			Trigger: models.TriggerManual, Success: true,
		})
		require.NoError(t, err)
	}
	got := h.List("", 50)
	require.Len(t, got, 2)
	require.Equal(t, "c", got[0].DeviceID)
	require.Equal(t, "b", got[1].DeviceID)
}
