package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/notify"
	"github.com/cleeryy/hello/internal/scheduler"
)

func newAtomicServer(t *testing.T, historyCap int) *Server {
	t.Helper()
	dir := t.TempDir()
	store := newStorage(t, filepath.Join(dir, "devices.json"))
	require.NoError(t, store.Create(&models.Device{ID: "pc1", Name: "Test PC", MAC: "00:11:22:33:44:55", IP: "192.168.9.9"}))
	h, err := history.NewWithCapacity(filepath.Join(dir, "hist.json"), historyCap)
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, Attempts: 1})
	require.NoError(t, err)
	st := newScheduleStore(t, filepath.Join(dir, "sched.json"))
	_, err = st.Create(models.Schedule{ID: "s1", DeviceID: "pc1", Cron: "@daily"})
	require.NoError(t, err)
	sched := scheduler.New(st,
		func(id string) (models.Device, error) {
			d, err := store.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *d, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	il, err := discover.LoadIgnoreList(filepath.Join(dir, "ignored.json"))
	require.NoError(t, err)
	s := New(&config.Config{}, store, nil)
	s.sendWOL = func(mac, broadcast string) error { return nil }
	return s.WithHistory(h).WithSchedules(st, sched).WithIgnoreList(il)
}

func scheduleTotal(t *testing.T, s *Server) []string {
	t.Helper()
	w := doRequest(s, http.MethodGet, "/schedules", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	ids := make([]string, 0, len(env.Schedules))
	for _, item := range env.Schedules {
		ids = append(ids, item.ID)
	}
	return ids
}

func historyTotal(t *testing.T, s *Server) int {
	t.Helper()
	w := doRequest(s, http.MethodGet, "/history", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		History []models.WakeEvent `json:"history"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return len(env.History)
}

// Given: a server with one device, one schedule, one history entry
// When: restore applies devices then the schedule apply fails
// Then: 500 problem+json and every store keeps its pre-restore state.
func TestRestore_rollsBackWhenSchedulesFail(t *testing.T) {
	s := newAtomicServer(t, 200)
	s.applySchedules = func([]models.Schedule) ([]models.Schedule, error) {
		return nil, errors.New("injected schedules failure")
	}
	payload := map[string]any{
		"devices": []any{
			map[string]any{"id": "new1", "name": "New One", "mac": "AA:BB:CC:DD:EE:FF"},
		},
		"schedules": []any{
			map[string]any{"id": "s2", "device_id": "new1", "cron": "@daily"},
		},
		"history": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
	require.Equal(t, []string{"s1"}, scheduleTotal(t, s))
	require.Equal(t, 1, historyTotal(t, s))
}

// Given: a server with one device and one history entry
// When: restore applies devices then the history apply fails
// Then: 500 and the registry still holds the original device.
func TestRestore_rollsBackWhenHistoryFails(t *testing.T) {
	s := newAtomicServer(t, 200)
	s.applyHistory = func([]models.WakeEvent) error {
		return errors.New("injected history failure")
	}
	payload := map[string]any{
		"devices": []any{
			map[string]any{"id": "new1", "name": "New One", "mac": "AA:BB:CC:DD:EE:FF"},
		},
		"history": []any{
			map[string]any{"device_id": "new1", "mac": "AA:BB:CC:DD:EE:FF", "trigger": "manual"},
		},
		"schedules": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
	require.Equal(t, 1, historyTotal(t, s))
}

// Given: a restore payload with a schedule for an unknown device
// When: POST /restore and POST /restore?dry_run=1
// Then: 422 both times and every store is untouched.
func TestRestore_rejectsScheduleForUnknownDevice(t *testing.T) {
	for _, target := range []string{"/restore", "/restore?dry_run=1"} {
		s := newAtomicServer(t, 200)
		payload := map[string]any{
			"devices": []any{
				map[string]any{"id": "pc1", "name": "Test PC", "mac": "00:11:22:33:44:55", "ip": "192.168.9.9"},
			},
			"schedules": []any{
				map[string]any{"id": "ghost", "device_id": "no-such-device", "cron": "@daily"},
			},
			"history": []any{},
		}
		w := doRequest(s, http.MethodPost, target, payload)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, target)
		total, ids := deviceTotal(t, s)
		require.Equal(t, 1, total)
		require.Equal(t, []string{"pc1"}, ids)
		require.Equal(t, []string{"s1"}, scheduleTotal(t, s))
		require.Equal(t, 1, historyTotal(t, s))
	}
}

// Given: a schedule for a device included in the same payload
// When: POST /restore
// Then: 200, the reference to the new device is accepted.
func TestRestore_acceptsScheduleForPayloadDevice(t *testing.T) {
	s := newAtomicServer(t, 200)
	payload := map[string]any{
		"devices": []any{
			map[string]any{"id": "new1", "name": "New One", "mac": "AA:BB:CC:DD:EE:FF"},
		},
		"schedules": []any{
			map[string]any{"id": "s2", "device_id": "new1", "cron": "@daily"},
		},
		"history": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusOK, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"new1"}, ids)
	require.Equal(t, []string{"s2"}, scheduleTotal(t, s))
}

// Given: a history store capped at 2 entries
// When: POST /restore with 3 history entries
// Then: 422 and the log is untouched, never silently truncated.
func TestRestore_rejectsHistoryOverCapacity(t *testing.T) {
	s := newAtomicServer(t, 2)
	event := func(id string) any {
		return map[string]any{"id": id, "device_id": "pc1", "mac": "00:11:22:33:44:55", "trigger": "manual"}
	}
	payload := map[string]any{
		"devices":   []any{map[string]any{"id": "pc1", "name": "Test PC", "mac": "00:11:22:33:44:55", "ip": "192.168.9.9"}},
		"history":   []any{event("h-1-1"), event("h-2-2"), event("h-3-3")},
		"schedules": []any{},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Equal(t, 1, historyTotal(t, s))
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
}

// Given: a restored log whose ids carry a high sequence number
// When: a new wake is recorded after the restore
// Then: the minted id keeps moving forward, never colliding backwards.
func TestRestore_keepsHistorySeqMonotonic(t *testing.T) {
	s := newAtomicServer(t, 200)
	payload := map[string]any{
		"history": []any{
			map[string]any{"id": "h-1700000000000000000-41", "device_id": "pc1", "mac": "00:11:22:33:44:55", "trigger": "manual"},
		},
	}
	w := doRequest(s, http.MethodPost, "/restore", payload)
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest(s, http.MethodPost, "/devices/pc1/wake", nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest(s, http.MethodGet, "/history", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		History []models.WakeEvent `json:"history"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.NotEmpty(t, env.History)
	require.NotEqual(t, "h-1700000000000000000-41", env.History[0].ID)
	require.Contains(t, env.History[0].ID, "-42")
}

// Given: devices and schedules created out of order, plus ignore entries
// When: GET /backup twice
// Then: both bodies are byte-identical, devices/schedules sorted by id,
// and the ignore list is included.
func TestBackup_deterministicSortedAndComplete(t *testing.T) {
	s := newAtomicServer(t, 200)
	require.NoError(t, s.store.Create(&models.Device{ID: "zebra", Name: "Z", MAC: "AA:BB:CC:DD:EE:01"}))
	require.NoError(t, s.store.Create(&models.Device{ID: "apple", Name: "A", MAC: "AA:BB:CC:DD:EE:02"}))
	_, err := s.schedStore.Create(models.Schedule{ID: "z-sched", DeviceID: "pc1", Cron: "@daily"})
	require.NoError(t, err)
	_, err = s.schedStore.Create(models.Schedule{ID: "a-sched", DeviceID: "pc1", Cron: "@hourly"})
	require.NoError(t, err)
	_, err = s.ignore.Add("", "10.0.0.9")
	require.NoError(t, err)
	_, err = s.ignore.Add("AA:BB:CC:DD:EE:09", "")
	require.NoError(t, err)

	first := doRequest(s, http.MethodGet, "/backup", nil)
	require.Equal(t, http.StatusOK, first.Code)
	second := doRequest(s, http.MethodGet, "/backup", nil)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, first.Body.String(), second.Body.String())

	var env struct {
		Version   int               `json:"version"`
		Devices   []models.Device   `json:"devices"`
		Schedules []models.Schedule `json:"schedules"`
		Ignore    []discover.Entry  `json:"ignore"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &env))
	require.Equal(t, 1, env.Version)
	gotDevices := make([]string, 0, len(env.Devices))
	for _, d := range env.Devices {
		gotDevices = append(gotDevices, d.ID)
	}
	require.Equal(t, []string{"apple", "pc1", "zebra"}, gotDevices)
	gotSchedules := make([]string, 0, len(env.Schedules))
	for _, item := range env.Schedules {
		gotSchedules = append(gotSchedules, item.ID)
	}
	require.Equal(t, []string{"a-sched", "s1", "z-sched"}, gotSchedules)
	require.Len(t, env.Ignore, 2)
	require.Equal(t, "aa:bb:cc:dd:ee:09", env.Ignore[0].MAC)
	require.Equal(t, "10.0.0.9", env.Ignore[1].IP)
}

// Given: a full restore payload
// When: POST /restore?dry_run=1
// Then: 200 and no store is mutated.
func TestRestore_dryRunLeavesAllStoresUntouched(t *testing.T) {
	s := newAtomicServer(t, 200)
	payload := map[string]any{
		"devices": []any{
			map[string]any{"id": "new1", "name": "New One", "mac": "AA:BB:CC:DD:EE:FF"},
		},
		"schedules": []any{
			map[string]any{"id": "s2", "device_id": "new1", "cron": "@daily"},
		},
		"history": []any{
			map[string]any{"device_id": "new1", "mac": "AA:BB:CC:DD:EE:FF", "trigger": "manual"},
		},
		"ignore": []any{
			map[string]any{"ip": "10.0.0.9"},
		},
	}
	w := doRequest(s, http.MethodPost, "/restore?dry_run=1", payload)
	require.Equal(t, http.StatusOK, w.Code)
	total, ids := deviceTotal(t, s)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"pc1"}, ids)
	require.Equal(t, []string{"s1"}, scheduleTotal(t, s))
	require.Equal(t, 1, historyTotal(t, s))
	require.Empty(t, s.ignore.List())
}

// Given: a status webhook endpoint wired through the Server
// When: a reachability change is fanned out
// Then: the endpoint receives the JSON status payload.
func TestNotifyStatus_postsWebhook(t *testing.T) {
	var mu sync.Mutex
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = body
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	s := newAtomicServer(t, 200).WithNotify(notify.New(srv.URL), nil)
	s.NotifyStatus(models.Device{ID: "pc1", Status: models.StatusUp})
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) > 0
	}, 3*time.Second, 20*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, string(got), `"device_id":"pc1"`)
	require.Contains(t, string(got), `"status":"up"`)
}

// Given: no status webhook configured
// When: a reachability change is fanned out
// Then: nothing happens, no panic, no block.
func TestNotifyStatus_disabledWithoutSender(t *testing.T) {
	s := newAtomicServer(t, 200)
	s.NotifyStatus(models.Device{ID: "pc1", Status: models.StatusDown})
}
