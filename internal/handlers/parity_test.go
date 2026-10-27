package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/monitor"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/scheduler"
)

// seedParityDevices loads a fixed lab/up/down/unknown trio straight into the
// store so parity tests stay focused on the seam under test.
func seedParityDevices(t *testing.T, s *Server) {
	t.Helper()
	for _, d := range []*models.Device{
		{ID: "pc1", Name: "A", MAC: "00:11:22:33:44:01", IP: "192.168.1.11", Status: models.StatusUp, Tags: []string{"lab"}},
		{ID: "pc2", Name: "B", MAC: "00:11:22:33:44:02", IP: "192.168.1.12", Status: models.StatusDown, Tags: []string{"lab", "media"}},
		{ID: "pc3", Name: "C", MAC: "00:11:22:33:44:03", Status: models.StatusUnknown},
	} {
		require.NoError(t, s.store.Create(d))
	}
}

// newParityServer returns a server with history wired and WOL stubbed.
func newParityServer(t *testing.T) (*Server, *history.History) {
	t.Helper()
	s := newTestServer(t)
	h, err := history.New(filepath.Join(t.TempDir(), "hist.json"))
	require.NoError(t, err)
	return s.WithHistory(h), h
}

// newParityFull wires history plus a live scheduler store for schedule tests.
func newParityFull(t *testing.T) (*Server, *history.History, *scheduler.Store) {
	t.Helper()
	s, h := newParityServer(t)
	st := newScheduleStore(t, filepath.Join(t.TempDir(), "sched.json"))
	sched := scheduler.New(st,
		func(id string) (models.Device, error) {
			d, err := s.store.Get(id)
			if err != nil {
				return models.Device{}, err
			}
			return *d, nil
		},
		func(models.Schedule, models.Device) (bool, string) { return true, "" })
	return s.WithSchedules(st, sched), h, st
}

type batchReport struct {
	DryRun  bool     `json:"dry_run"`
	Matched []string `json:"matched"`
	Woken   []string `json:"woken"`
	Failed  []struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	} `json:"failed"`
}

func decodeBatch(t *testing.T, w *httptest.ResponseRecorder) batchReport {
	t.Helper()
	var rep batchReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rep))
	return rep
}

// Given: three registered devices
// When: DELETE /devices with 51 ids
// Then: 422 problem+json and the registry is untouched.
func TestParity_bulkDeleteCap(t *testing.T) {
	s, _ := newParityServer(t)
	seedParityDevices(t, s)

	ids := []string{"pc1"}
	for i := 0; i < 50; i++ {
		ids = append(ids, fmt.Sprintf("ghost-%d", i))
	}
	w := doRequest(s, http.MethodDelete, "/devices?ids="+strings.Join(ids, ","), nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	require.Equal(t, 3, len(s.store.GetAll()))
}

// Given: a registry
// When: POST /devices/import with 201 entries
// Then: 422 problem+json and nothing is written.
func TestParity_importCap(t *testing.T) {
	s, _ := newParityServer(t)
	seedParityDevices(t, s)

	devs := make([]any, 0, 201)
	for i := 0; i < 201; i++ {
		devs = append(devs, map[string]any{
			"id":   fmt.Sprintf("bulk-%03d", i),
			"name": fmt.Sprintf("Bulk %d", i),
			"mac":  fmt.Sprintf("02:00:00:%02x:%02x:%02x", i/65536, (i/256)%256, i%256),
		})
	}
	w := doRequest(s, http.MethodPost, "/devices/import", map[string]any{"devices": devs})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	require.Equal(t, 3, len(s.store.GetAll()))
}

// Given: lab-tagged devices
// When: POST /devices/wake?tag=lab&dry_run=1 then POST /devices/wake?tag=lab
// Then: dry-run previews the tag set, the real call wakes it (wake-batch shape).
func TestParity_devicesWakeTag(t *testing.T) {
	s, _ := newParityServer(t)
	seedParityDevices(t, s)
	var sent []string
	s.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac); return nil }

	w := doRequest(s, http.MethodPost, "/devices/wake?tag=lab&dry_run=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	rep := decodeBatch(t, w)
	require.True(t, rep.DryRun)
	require.ElementsMatch(t, []string{"pc1", "pc2"}, rep.Matched)
	require.Empty(t, sent)

	s2, _ := newParityServer(t)
	seedParityDevices(t, s2)
	s2.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac); return nil }
	sent = nil
	w = doRequest(s2, http.MethodPost, "/devices/wake?tag=lab", nil)
	require.Equal(t, http.StatusOK, w.Code)
	rep = decodeBatch(t, w)
	require.False(t, rep.DryRun)
	require.ElementsMatch(t, []string{"pc1", "pc2"}, rep.Woken)
	require.Empty(t, rep.Failed)
	require.Len(t, sent, 2)
}

// Given: up/down devices and a wake history
// When: POST /wake/all?status=up&note=hello
// Then: only up devices wake, the note lands on each history entry.
func TestParity_wakeAllStatusNote(t *testing.T) {
	s, h := newParityServer(t)
	seedParityDevices(t, s)
	s.sendWOL = func(mac, broadcast string) error { return nil }

	w := doRequest(s, http.MethodPost, "/wake/all?status=up&note=hello", nil)
	require.Equal(t, http.StatusOK, w.Code)
	rep := decodeBatch(t, w)
	require.Equal(t, []string{"pc1"}, rep.Matched)
	require.Equal(t, []string{"pc1"}, rep.Woken)
	require.Empty(t, rep.Failed)

	entries := h.ListFiltered(history.Filter{}, 10)
	require.Len(t, entries, 1)
	require.Equal(t, "pc1", entries[0].DeviceID)
	require.Equal(t, "hello", entries[0].Note)

	s2, _ := newParityServer(t)
	seedParityDevices(t, s2)
	w = doRequest(s2, http.MethodPost, "/wake/all?status=bogus", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a registered device
// When: POST /wake/at with a future timestamp
// Then: 201 with the created one-shot, cancellable via the oneshots route.
func TestParity_wakeAtAlias(t *testing.T) {
	s, _ := newParityServer(t)
	seedParityDevices(t, s)

	at := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	w := doRequest(s, http.MethodPost, "/wake/at", map[string]any{"device_id": "pc1", "at": at})
	require.Equal(t, http.StatusCreated, w.Code)
	var entry struct {
		ID       string `json:"id"`
		DeviceID string `json:"device_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	require.NotEmpty(t, entry.ID)
	require.Equal(t, "pc1", entry.DeviceID)

	w = doRequest(s, http.MethodDelete, "/wake/oneshots/"+entry.ID, nil)
	require.Equal(t, http.StatusNoContent, w.Code)
}

// Given: a per-device wake cooldown
// When: a batch targets a cooling device, and single/batch wakes pass broadcast_ip
// Then: the cooling device lands in failed (nothing sent); broadcast_ip wins.
func TestParity_batchCooldownAndBroadcast(t *testing.T) {
	s, _ := newParityServer(t)
	seedParityDevices(t, s)
	s.cfg.WakeCooldownSec = 60
	require.NoError(t, s.store.RecordWake("pc1", time.Now().Unix()))
	var sent []string
	s.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac+("|")+broadcast); return nil }

	w := doRequest(s, http.MethodPost, "/devices/wake-batch", map[string]any{"ids": []string{"pc1"}})
	require.Equal(t, http.StatusOK, w.Code)
	rep := decodeBatch(t, w)
	require.Equal(t, []string{"pc1"}, rep.Matched)
	require.Empty(t, rep.Woken)
	require.Len(t, rep.Failed, 1)
	require.Equal(t, "pc1", rep.Failed[0].ID)
	require.Contains(t, rep.Failed[0].Error, "cooling down")
	require.Empty(t, sent)

	s2, _ := newParityServer(t)
	seedParityDevices(t, s2)
	var broadcasts []string
	s2.sendWOL = func(mac, broadcast string) error { broadcasts = append(broadcasts, broadcast); return nil }

	w = doRequest(s2, http.MethodPost, "/devices/pc3/wake?broadcast_ip=192.168.9.255", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"192.168.9.255"}, broadcasts)

	s3, _ := newParityServer(t)
	seedParityDevices(t, s3)
	var gotBroadcast string
	s3.sendWOL = func(mac, broadcast string) error { gotBroadcast = broadcast; return nil }
	w = doRequest(s3, http.MethodPost, "/devices/wake-batch?broadcast=192.168.8.255", map[string]any{"ids": []string{"pc3"}})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "192.168.8.255", gotBroadcast)
}

// Given: recorded ok and error wakes at known times
// When: listing with result/until/order aliases
// Then: filtering and ordering apply; bad values 400.
func TestParity_historyResultUntilOrder(t *testing.T) {
	engine, h := mountWithHistory(t)
	now := time.Now().Unix()
	_, err := h.Record(models.WakeEvent{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, Attempts: 1, At: now - 100})
	require.NoError(t, err)
	_, err = h.Record(models.WakeEvent{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: false, Attempts: 1, Error: "boom", At: now})
	require.NoError(t, err)

	get := func(target string) (int, []models.WakeEvent) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		var body struct {
			History []models.WakeEvent `json:"history"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return rec.Code, body.History
	}

	code, rows := get("/history?result=ok")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, rows, 1)
	require.True(t, rows[0].Success)

	code, rows = get("/history?result=error")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Success)

	code, _ = get("/history?result=bogus")
	require.Equal(t, http.StatusBadRequest, code)

	until := time.Unix(now-50, 0).UTC().Format(time.RFC3339)
	code, rows = get("/history?until=" + until)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, rows, 1)
	require.True(t, rows[0].Success)

	code, _ = get("/history?until=not-a-time")
	require.Equal(t, http.StatusBadRequest, code)

	code, rows = get("/history?order=asc")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, rows, 2)
	require.True(t, rows[0].At < rows[1].At)

	code, _ = get("/history?order=sideways")
	require.Equal(t, http.StatusBadRequest, code)

	// Export honors order: asc starts with the oldest entry.
	req := httptest.NewRequest(http.MethodGet, "/history/export?order=asc", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	require.Len(t, lines, 3)
	require.Contains(t, lines[1], "true")
	require.Contains(t, lines[2], "false")
}

// Given: retained wakes across two devices
// When: GET /history/stats
// Then: legacy fields stay and top_devices ranks by attempts.
func TestParity_historyStatsTopDevices(t *testing.T) {
	engine, h := mountWithHistory(t)
	for _, e := range []models.WakeEvent{
		{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, Attempts: 2},
		{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: true, Attempts: 1},
		{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Trigger: models.TriggerManual, Success: false, Attempts: 1, Error: "x"},
		{DeviceID: "pc2", MAC: "00:11:22:33:44:66", Trigger: models.TriggerSchedule, Success: true, Attempts: 1},
	} {
		_, err := h.Record(e)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/history/stats", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var stats struct {
		Total      int `json:"total"`
		OK         int `json:"ok"`
		Error      int `json:"error"`
		TopDevices []struct {
			DeviceID string `json:"device_id"`
			Count    int    `json:"count"`
			OK       int    `json:"ok"`
			Error    int    `json:"error"`
		} `json:"top_devices"`
		ByTrigger map[string]map[string]int `json:"by_trigger"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	require.Equal(t, 4, stats.Total)
	require.Equal(t, 3, stats.OK)
	require.Equal(t, 1, stats.Error)
	require.Len(t, stats.TopDevices, 2)
	require.Equal(t, "pc1", stats.TopDevices[0].DeviceID)
	require.Equal(t, 3, stats.TopDevices[0].Count)
	require.Equal(t, 2, stats.TopDevices[0].OK)
	require.Equal(t, 1, stats.TopDevices[0].Error)
	require.Equal(t, "pc2", stats.TopDevices[1].DeviceID)
	require.NotEmpty(t, stats.ByTrigger)
}

// Given: a schedule and history
// When: the schedule fires, then GET /schedules/:id/history
// Then: the wake carries schedule_id and the endpoint returns it; unknown 404.
func TestParity_scheduleHistory(t *testing.T) {
	s, h, st := newParityFull(t)
	seedParityDevices(t, s)
	_, err := st.Create(models.Schedule{ID: "s1", DeviceID: "pc1", Cron: "@daily"})
	require.NoError(t, err)
	s.sendWOL = func(mac, broadcast string) error { return nil }

	w := doRequest(s, http.MethodPost, "/schedules/s1/fire", nil)
	require.Equal(t, http.StatusOK, w.Code)

	entries := h.ListFiltered(history.Filter{ScheduleID: "s1"}, 10)
	require.Len(t, entries, 1)
	require.Equal(t, "s1", entries[0].ScheduleID)
	require.Equal(t, models.TriggerSchedule, entries[0].Trigger)

	w = doRequest(s, http.MethodGet, "/schedules/s1/history", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		History []models.WakeEvent `json:"history"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.History, 1)
	require.Equal(t, "s1", body.History[0].ScheduleID)

	w = doRequest(s, http.MethodGet, "/schedules/nope/history", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// Given: two schedules
// When: POST /schedules/disable and /schedules/enable with ?ids=
// Then: {updated:[...]} flips both with one reload; unknown ids 422 atomically.
func TestParity_schedulesBulkEnable(t *testing.T) {
	s, _, st := newParityFull(t)
	seedParityDevices(t, s)
	for _, id := range []string{"s1", "s2"} {
		_, err := st.Create(models.Schedule{ID: id, DeviceID: "pc1", Cron: "@daily"})
		require.NoError(t, err)
	}

	w := doRequest(s, http.MethodPost, "/schedules/disable?ids=s1,s2", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var report struct {
		Updated []string `json:"updated"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	require.ElementsMatch(t, []string{"s1", "s2"}, report.Updated)
	got, err := st.Get("s1")
	require.NoError(t, err)
	require.False(t, got.Enabled)

	// Unknown ids fail the whole batch: s1 stays disabled.
	w = doRequest(s, http.MethodPost, "/schedules/enable?ids=s1,nope", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	got, err = st.Get("s1")
	require.NoError(t, err)
	require.False(t, got.Enabled)

	w = doRequest(s, http.MethodPost, "/schedules/enable?ids=s1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	got, err = st.Get("s1")
	require.NoError(t, err)
	require.True(t, got.Enabled)

	many := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		many = append(many, fmt.Sprintf("s-%d", i))
	}
	w = doRequest(s, http.MethodPost, "/schedules/enable?ids="+strings.Join(many, ","), nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a wired scanner
// When: POST /discover with a /20 cidr
// Then: 422 before any scan runs.
func TestParity_discoverCIDRGuard(t *testing.T) {
	s, _ := newParityServer(t)
	s.WithDiscover(discover.New())
	w := doRequest(s, http.MethodPost, "/discover?cidr=10.0.0.0/20", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}

// Given: a wired ignore list
// When: hitting /discover/ignored instead of /discover/ignore
// Then: identical list/add/remove behavior.
func TestParity_discoverIgnoredAlias(t *testing.T) {
	s, _ := newParityServer(t)
	il, err := discover.LoadIgnoreList(filepath.Join(t.TempDir(), "ignored.json"))
	require.NoError(t, err)
	s.WithIgnoreList(il)

	w := doRequest(s, http.MethodGet, "/discover/ignored", nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(s, http.MethodPost, "/discover/ignored", map[string]any{"mac": "00:11:22:33:44:99"})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodGet, "/discover/ignore", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "00:11:22:33:44:99")

	w = doRequest(s, http.MethodDelete, "/discover/ignored?mac=00:11:22:33:44:99", nil)
	require.Equal(t, http.StatusOK, w.Code)
}

// Given: a scan state file from a previous run
// When: the scanner loads it and GET /discover/status runs
// Then: the persisted outcome is reported; corrupt files fail fast.
func TestParity_scanPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scan.json")
	raw, _ := json.Marshal(map[string]any{
		"last_scan_at": int64(1700000000),
		"last_hosts":   7,
		"last_cidr":    "192.168.1.0/24",
	})
	require.NoError(t, writeFile0600(path, raw))

	s, _ := newParityServer(t)
	disc := discover.New()
	require.NoError(t, disc.SetScanFile(path))
	s.WithDiscover(disc)

	w := doRequest(s, http.MethodGet, "/discover/status", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var status struct {
		LastScan  int64  `json:"last_scan_at"`
		LastHosts int    `json:"last_hosts"`
		LastCIDR  string `json:"last_cidr"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	require.Equal(t, int64(1700000000), status.LastScan)
	require.Equal(t, 7, status.LastHosts)
	require.Equal(t, "192.168.1.0/24", status.LastCIDR)

	bad := filepath.Join(dir, "bad-scan.json")
	require.NoError(t, writeFile0600(bad, []byte("{nope")))
	require.Error(t, discover.New().SetScanFile(bad))
}

// Given: a pingable loopback device under a live monitor
// When: a check runs, then GET /devices/:id/uptime
// Then: honest counters (checks/up/down/percent/last_change/since); unknown 404.
func TestParity_deviceUptime(t *testing.T) {
	s, _ := newParityServer(t)
	require.NoError(t, s.store.Create(&models.Device{
		ID: "pc-loop", Name: "Loop", MAC: "00:11:22:33:44:50",
		IP: "127.0.0.1", PingEnabled: true,
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	oldPorts := ping.TCPPorts
	ping.TCPPorts = []string{port}
	defer func() { ping.TCPPorts = oldPorts }()

	mon := monitor.NewWithTimeout(s.store, time.Minute, 200*time.Millisecond)
	s.WithMonitor(mon)

	w := doRequest(s, http.MethodPost, "/monitor/check", nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(s, http.MethodGet, "/devices/pc-loop/uptime", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var up struct {
		DeviceID   string  `json:"device_id"`
		Checks     int     `json:"checks"`
		Up         int     `json:"up"`
		Down       int     `json:"down"`
		Percent    float64 `json:"percent"`
		LastChange int64   `json:"last_change"`
		Since      int64   `json:"since"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &up))
	require.Equal(t, "pc-loop", up.DeviceID)
	require.Equal(t, 1, up.Checks)
	require.Equal(t, 1, up.Up)
	require.Equal(t, 0, up.Down)
	require.Equal(t, float64(100), up.Percent)
	require.Greater(t, up.LastChange, int64(0))
	require.Greater(t, up.Since, int64(0))

	w = doRequest(s, http.MethodGet, "/devices/nope/uptime", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// writeFile0600 stores helper JSON with registry-grade permissions.
func writeFile0600(path string, raw []byte) error {
	return os.WriteFile(path, raw, 0o600)
}
