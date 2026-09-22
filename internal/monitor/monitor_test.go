package monitor_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/monitor"
	"github.com/cleeryy/hello/internal/storage"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newStorage(t *testing.T, path string) *storage.Storage {
	t.Helper()
	store, err := storage.New(path)
	require.NoError(t, err)
	return store
}

func Test_Monitor_emits_status_change_on_start(t *testing.T) {
	// Given — 192.0.2.1 is TEST-NET-1, guaranteed unroutable: PingHost is false.
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: true,
	}))
	mon := monitor.New(s, time.Hour)
	events := make(chan models.Device, 1)
	mon.OnStatusChange = func(d models.Device) { events <- d }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// When
	mon.Start(ctx)
	defer mon.Stop()

	// Then
	select {
	case got := <-events:
		assert.Equal(t, "pc1", got.ID)
		assert.Equal(t, models.StatusDown, got.Status)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for status change")
	}
}

func Test_Monitor_skips_devices_without_ping(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	require.NoError(t, s.Create(&models.Device{
		ID: "pc1", Name: "PC", MAC: "00:11:22:33:44:55",
		Status: models.StatusUnknown, IP: "192.0.2.1", PingEnabled: false,
	}))
	mon := monitor.New(s, time.Hour)
	fired := false
	mon.OnStatusChange = func(models.Device) { fired = true }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// When
	mon.Start(ctx)
	select {
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
	}
	mon.Stop()

	// Then
	got, err := s.Get("pc1")
	require.NoError(t, err)
	assert.Equal(t, models.StatusUnknown, got.Status)
	assert.False(t, fired)
}

func Test_Monitor_Stop_is_idempotent(t *testing.T) {
	// Given
	s := newStorage(t, filepath.Join(t.TempDir(), "devices.json"))
	mon := monitor.New(s, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)

	// When
	mon.Stop()
	mon.Stop()

	// Then — no deadlock, no panic
}
