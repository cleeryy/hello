package scheduler_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/scheduler"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestScheduler_whenTickFires(t *testing.T) {
	// Given: a scheduler with one per-second schedule and a recording fire
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "tick", DeviceID: "pc", Cron: "@every 1s"})
	require.NoError(t, err)
	var mu sync.Mutex
	var fired []models.Schedule
	sch := scheduler.New(st,
		func(id string) (models.Device, error) {
			return models.Device{ID: id, Name: "pc", MAC: "00:11:22:33:44:55"}, nil
		},
		func(sched models.Schedule, _ models.Device) (bool, string) {
			mu.Lock()
			fired = append(fired, sched)
			mu.Unlock()
			return true, ""
		})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Start(ctx)
	// When: a second elapses
	// Then: the schedule fired at least once with its id
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(fired) >= 1
	}, 5*time.Second, 50*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "tick", fired[0].ID)
}

func TestScheduler_whenReloadReflectsStore(t *testing.T) {
	// Given: a scheduler on an empty store
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	sch := scheduler.New(st,
		func(id string) (models.Device, error) {
			return models.Device{ID: id}, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	// When: schedules are added then removed with Reload between
	// Then: active entry count tracks the store
	require.Equal(t, 0, sch.Entries())
	_, err := st.Create(models.Schedule{ID: "a", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	_, err = st.Create(models.Schedule{ID: "b", DeviceID: "pc", Cron: "@daily"})
	require.NoError(t, err)
	_, err = st.Update("b", models.Schedule{ID: "b", DeviceID: "pc", Cron: "@daily", Enabled: true})
	require.NoError(t, err)
	sch.Reload()
	require.Equal(t, 2, sch.Entries())
	_, err = st.Update("b", models.Schedule{ID: "b", DeviceID: "pc", Cron: "@daily", Enabled: false})
	require.NoError(t, err)
	sch.Reload()
	require.Equal(t, 1, sch.Entries())
	require.NoError(t, st.Delete("a"))
	sch.Reload()
	require.Equal(t, 0, sch.Entries())
}

func TestScheduler_whenDeviceGone(t *testing.T) {
	// Given: a schedule whose device vanished, fire must not run
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "schedules.json"))
	_, err := st.Create(models.Schedule{ID: "ghost", DeviceID: "gone", Cron: "@every 1s"})
	require.NoError(t, err)
	calls := make(chan struct{}, 10)
	sch := scheduler.New(st,
		func(id string) (models.Device, error) {
			return models.Device{}, scheduler.ErrNotFound
		},
		func(models.Schedule, models.Device) (bool, string) { calls <- struct{}{}; return true, "" })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Start(ctx)
	// When: two seconds elapse
	// Then: no fire for the orphaned schedule
	select {
	case <-calls:
		t.Fatal("fire must not run for an unknown device")
	case <-time.After(2 * time.Second):
	}
}
