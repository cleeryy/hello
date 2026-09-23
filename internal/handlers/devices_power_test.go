package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cleeryy/hello/internal/models"
	"github.com/stretchr/testify/require"
)

func seedPowerDevices(t *testing.T) *Server {
	t.Helper()
	s := newTestServer(t)
	devs := []models.Device{
		{ID: "pc1", Name: "Alpha", MAC: "00:11:22:33:44:01", IP: "192.168.1.11", Status: models.StatusUp, Tags: []string{"lab"}},
		{ID: "pc2", Name: "beta", MAC: "00:11:22:33:44:02", IP: "192.168.1.12", Status: models.StatusDown, Tags: []string{"lab", "media"}},
		{ID: "pc3", Name: "Gamma", MAC: "00:11:22:33:44:03", Status: models.StatusUnknown},
	}
	for _, d := range devs {
		w := doRequest(s, http.MethodPost, "/devices", d)
		require.Equal(t, http.StatusCreated, w.Code, "seed %s", d.ID)
	}
	return s
}

type deviceList struct {
	Devices []models.Device `json:"devices"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}

func getList(t *testing.T, s *Server, target string) deviceList {
	t.Helper()
	w := doRequest(s, http.MethodGet, target, nil)
	require.Equal(t, http.StatusOK, w.Code, target)
	var list deviceList
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	return list
}

// TestDevicesPatch verifies JSON merge patch semantics.
func TestDevicesPatch(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF"})
	require.Equal(t, http.StatusCreated, w.Code)

	// When: patching name, notes and tags (mixed case, duplicates, blanks).
	w = doRequest(s, http.MethodPatch, "/devices/pc1", map[string]any{
		"name": "PC Salon", "notes": "  sous le bureau  ", "tags": []string{"Lab", "lab", "", "MEDIA"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	var updated models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, "PC Salon", updated.Name)
	require.Equal(t, "sous le bureau", updated.Notes)
	require.Equal(t, []string{"lab", "media"}, updated.Tags)

	// Then: unknown field, empty patch, id and bad MAC all 422.
	for _, body := range []any{
		map[string]any{"nope": 1},
		map[string]any{},
		map[string]any{"id": "pc2"},
		map[string]any{"mac": "oops"},
		map[string]any{"ping_enabled": "yes"},
		map[string]any{"tags": "lab"},
	} {
		w = doRequest(s, http.MethodPatch, "/devices/pc1", body)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, "patch %v", body)
	}

	// Then: unknown device 404.
	w = doRequest(s, http.MethodPatch, "/devices/nope", map[string]any{"name": "x"})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDevicesListQuery verifies filters, sort and pagination.
func TestDevicesListQuery(t *testing.T) {
	s := seedPowerDevices(t)

	// Plain list carries the envelope with total.
	list := getList(t, s, "/devices")
	require.Equal(t, 3, list.Total)
	require.Equal(t, 1, list.Page)
	require.Equal(t, []string{"pc1", "pc2", "pc3"}, []string{list.Devices[0].ID, list.Devices[1].ID, list.Devices[2].ID},
		"default sort is name asc, case-insensitive")

	// Status, search and tag filters.
	require.Len(t, getList(t, s, "/devices?status=up").Devices, 1)
	require.Len(t, getList(t, s, "/devices?search=192.168.1.12").Devices, 1)
	require.Len(t, getList(t, s, "/devices?search=ALP").Devices, 1)
	require.Len(t, getList(t, s, "/devices?tag=lab").Devices, 2)
	require.Len(t, getList(t, s, "/devices?tag=media&status=down").Devices, 1)
	require.Len(t, getList(t, s, "/devices?tag=nope").Devices, 0)

	// Pagination slices honestly.
	paged := getList(t, s, "/devices?page=2&per_page=1")
	require.Equal(t, 3, paged.Total)
	require.Equal(t, 2, paged.Page)
	require.Len(t, paged.Devices, 1)
	require.Equal(t, "pc2", paged.Devices[0].ID)

	// Invalid params 422.
	for _, target := range []string{
		"/devices?status=bogus", "/devices?sort=bogus", "/devices?order=sideways",
		"/devices?page=0", "/devices?per_page=0", "/devices?per_page=501", "/devices?page=x",
	} {
		w := doRequest(s, http.MethodGet, target, nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, target)
	}
}

// TestDevicesBulkDelete verifies multi-delete with per-id report.
func TestDevicesBulkDelete(t *testing.T) {
	s := seedPowerDevices(t)

	w := doRequest(s, http.MethodDelete, "/devices?ids=pc1,pc2,nope", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var report struct {
		Deleted  []string `json:"deleted"`
		NotFound []string `json:"not_found"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	require.Equal(t, []string{"pc1", "pc2"}, report.Deleted)
	require.Equal(t, []string{"nope"}, report.NotFound)
	require.Equal(t, 1, getList(t, s, "/devices").Total)

	w = doRequest(s, http.MethodDelete, "/devices", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestDevicesClone verifies copy semantics and counter reset.
func TestDevicesClone(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{
		ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.1.10",
		Notes: "n", Tags: []string{"lab"},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(s, http.MethodPost, "/devices/pc1/clone?new_id=pc9", nil)
	require.Equal(t, http.StatusCreated, w.Code)
	var cloned models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cloned))
	require.Equal(t, "pc9", cloned.ID)
	require.Equal(t, "PC (copy)", cloned.Name)
	require.Equal(t, "AA:BB:CC:DD:EE:FF", cloned.MAC)
	require.Equal(t, models.StatusUnknown, cloned.Status)
	require.Equal(t, []string{"lab"}, cloned.Tags)
	require.Equal(t, 0, cloned.WakeCount)

	w = doRequest(s, http.MethodPost, "/devices/pc1/clone", nil)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	w = doRequest(s, http.MethodPost, "/devices/pc1/clone?new_id=pc9", nil)
	require.Equal(t, http.StatusConflict, w.Code)
	w = doRequest(s, http.MethodPost, "/devices/nope/clone?new_id=pc8", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestWakeBatch verifies selection, dry-run and per-device report.
func TestWakeBatch(t *testing.T) {
	newSeeded := func(t *testing.T) *Server {
		t.Helper()
		s := newTestServer(t)
		for _, d := range []models.Device{
			{ID: "pc1", Name: "A", MAC: "00:11:22:33:44:01", IP: "192.168.1.11", Tags: []string{"lab"}},
			{ID: "pc2", Name: "B", MAC: "00:11:22:33:44:02", Tags: []string{"lab"}},
		} {
			w := doRequest(s, http.MethodPost, "/devices", d)
			require.Equal(t, http.StatusCreated, w.Code)
		}
		return s
	}

	// Dry-run previews without sending.
	s := newSeeded(t)
	var sent []string
	s.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac); return nil }
	w := doRequest(s, http.MethodPost, "/devices/wake-batch", map[string]any{"tag": "lab", "dry_run": true})
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, sent)
	var preview struct {
		DryRun  bool     `json:"dry_run"`
		Matched []string `json:"matched"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &preview))
	require.True(t, preview.DryRun)
	require.Len(t, preview.Matched, 2)

	// Real batch sends to the tag union (fresh server: shared 30s throttle).
	s = newSeeded(t)
	s.sendWOL = func(mac, broadcast string) error { sent = append(sent, mac); return nil }
	w = doRequest(s, http.MethodPost, "/devices/wake-batch", map[string]any{"ids": []string{"pc1"}, "tag": "lab"})
	require.Equal(t, http.StatusOK, w.Code)
	var report struct {
		Matched []string `json:"matched"`
		Woken   []string `json:"woken"`
		Failed  []any    `json:"failed"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	require.Len(t, report.Matched, 2)
	require.Len(t, report.Woken, 2)
	require.Empty(t, report.Failed)
	require.Len(t, sent, 2)

	// Batch bumps wake counters server-side, exactly once per device.
	got := getList(t, s, "/devices?search=pc1")
	require.Equal(t, 1, got.Devices[0].WakeCount)
	require.Greater(t, got.Devices[0].LastWakeAt, int64(0))

	// Unknown id 404s the whole batch (nothing sent).
	s = newSeeded(t)
	w = doRequest(s, http.MethodPost, "/devices/wake-batch", map[string]any{"ids": []string{"nope"}})
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDevicesExportImport verifies download and the three import modes.
func TestDevicesExportImport(t *testing.T) {
	s := seedPowerDevices(t)

	// Export downloads a JSON array as attachment.
	w := doRequest(s, http.MethodGet, "/devices/export", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	var exported []models.Device
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &exported))
	require.Len(t, exported, 3)

	fresh := func(id, name, mac string) map[string]any {
		return map[string]any{"id": id, "name": name, "mac": mac}
	}

	// Abort mode refuses existing ids and writes nothing.
	w = doRequest(s, http.MethodPost, "/devices/import", map[string]any{
		"devices": []any{fresh("pc1", "Dup", "00:11:22:33:44:09"), fresh("pc9", "New", "00:11:22:33:44:09")},
		"mode":    "abort",
	})
	require.Equal(t, http.StatusConflict, w.Code)
	require.Equal(t, 3, getList(t, s, "/devices").Total)

	// Skip mode creates the new one and reports the skipped id.
	w = doRequest(s, http.MethodPost, "/devices/import", map[string]any{
		"devices": []any{fresh("pc1", "Dup", "00:11:22:33:44:09"), fresh("pc9", "New", "00:11:22:33:44:09")},
		"mode":    "skip",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var skipped struct {
		Created []models.Device `json:"created"`
		Skipped []string        `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &skipped))
	require.Len(t, skipped.Created, 1)
	require.Equal(t, []string{"pc1"}, skipped.Skipped)

	// Replace mode overwrites.
	w = doRequest(s, http.MethodPost, "/devices/import", map[string]any{
		"devices": []any{fresh("pc9", "Renamed", "00:11:22:33:44:09")},
		"mode":    "replace",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	got := getList(t, s, "/devices?search=pc9")
	require.Equal(t, "Renamed", got.Devices[0].Name)

	// A bad batch writes nothing.
	before := getList(t, s, "/devices").Total
	for _, bad := range []any{
		map[string]any{"devices": []any{}},
		map[string]any{"devices": []any{fresh("x", "", "00:11:22:33:44:0A")}},
		map[string]any{"devices": []any{fresh("y", "Y", "oops")}},
		map[string]any{"devices": []any{fresh("z", "Z", "00:11:22:33:44:0B"), fresh("z", "Z2", "00:11:22:33:44:0C")}},
		map[string]any{"devices": []any{fresh("w", "W", "00:11:22:33:44:0D")}, "mode": "bogus"},
		map[string]any{"devices": nil},
	} {
		w = doRequest(s, http.MethodPost, "/devices/import", bad)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, "import %v", bad)
	}
	require.Equal(t, before, getList(t, s, "/devices").Total)
}

// TestDeviceCounts verifies cheap polling totals.
func TestDeviceCounts(t *testing.T) {
	s := seedPowerDevices(t)
	w := doRequest(s, http.MethodGet, "/devices/counts", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var counts map[string]int
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &counts))
	require.Equal(t, map[string]int{"total": 3, "up": 1, "down": 1, "unknown": 1}, counts)
}

// TestWakeCountBump verifies a manual wake bumps the device counters.
func TestWakeCountBump(t *testing.T) {
	engine, _ := mountWithHistory(t)

	req := httptest.NewRequest(http.MethodPost, "/devices/pc1/wake", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/devices/pc1", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var d models.Device
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	require.Equal(t, 1, d.WakeCount)
	require.Greater(t, d.LastWakeAt, int64(0))
}

// TestStorageRecordWake verifies the atomic counter bump.
func TestStorageRecordWake(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices", models.Device{ID: "pc1", Name: "PC", MAC: "AA:BB:CC:DD:EE:FF"})
	require.Equal(t, http.StatusCreated, w.Code)

	require.NoError(t, s.store.RecordWake("pc1", 1700000000))
	require.NoError(t, s.store.RecordWake("pc1", 1700000060))
	d, err := s.store.Get("pc1")
	require.NoError(t, err)
	require.Equal(t, 2, d.WakeCount)
	require.Equal(t, int64(1700000060), d.LastWakeAt)
	require.Error(t, s.store.RecordWake("nope", 1))
}

// TestDevicesListEnvelopeStability pins the list envelope for dashboard clients.
func TestDevicesListEnvelopeStability(t *testing.T) {
	s := seedPowerDevices(t)
	w := doRequest(s, http.MethodGet, "/devices", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	for _, key := range []string{`"devices"`, `"total"`, `"page"`, `"per_page"`} {
		require.True(t, strings.Contains(body, key), "envelope misses %s", key)
	}
}
