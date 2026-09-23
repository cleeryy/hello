package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/models"
)

// discoverFullRouter pins a scanner to 127.0.0.1/30 and optionally wires an
// ignore list, returning the mounted engine.
func discoverFullRouter(t *testing.T, lnPort int, ignorePath string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	_, subnet, err := net.ParseCIDR("127.0.0.1/30")
	require.NoError(t, err)
	d := discover.New()
	d.Subnet = subnet
	d.ProbePorts = []int{lnPort}
	d.ResolveHostname = func(string) string { return "" }
	srv := New(&config.Config{}, newStorage(t, t.TempDir()+"/d.json"), nil).WithDiscover(d)
	if ignorePath != "" {
		il, err := discover.LoadIgnoreList(ignorePath)
		require.NoError(t, err)
		srv.WithIgnoreList(il)
	}
	r := gin.New()
	srv.Mount(r)
	return r
}

func doGET(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doDELETE(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Given: a listener on loopback
// When: POSTing /discover?cidr=127.0.0.1/30
// Then: 200 with loopback found and an ignored count of zero.
func TestDiscoverCIDR(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	r := discoverFullRouter(t, ln.Addr().(*net.TCPAddr).Port, "")

	w := doPOST(t, r, "/discover?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Hosts   []discover.Host `json:"hosts"`
		Ignored int             `json:"ignored"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Ignored)
	found := false
	for _, h := range body.Hosts {
		if h.IP == "127.0.0.1" {
			found = true
		}
	}
	require.True(t, found, "loopback missing: %+v", body.Hosts)
}

// Given: bad cidr values
// When: POSTing /discover?cidr=...
// Then: 422 before any scan runs (fresh router per case: the throttle
// middleware consumes its bucket on every request, even invalid ones).
func TestDiscoverCIDRInvalid(t *testing.T) {
	for _, raw := range []string{"not-a-cidr", "10.0.0.0/8"} {
		r := discoverFullRouter(t, 0, "")
		w := doPOST(t, r, "/discover?cidr="+raw, "")
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, raw)
		require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	}
}

// Given: a fresh scanner
// When: GETting /discover/status
// Then: 200 with idle guards and no finished scan.
func TestDiscoverStatus(t *testing.T) {
	r := discoverFullRouter(t, 0, "")

	w := doGET(t, r, "/discover/status")
	require.Equal(t, http.StatusOK, w.Code)
	var rep discover.Report
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rep))
	require.False(t, rep.Scanning)
	require.Equal(t, int64(0), rep.RetryInS)
	require.Equal(t, int64(0), rep.LastScan)
	require.Equal(t, 0, rep.LastHosts)
}

// Given: one candidate host
// When: POSTing /discover/adopt?dry_run=1
// Then: 200 preview, nothing stored.
func TestAdoptDryRun(t *testing.T) {
	r := discoverFullRouter(t, 0, "")

	w := doPOST(t, r, "/discover/adopt?dry_run=1", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:61","ip":"192.168.7.61"}]}`)
	require.Equal(t, http.StatusOK, w.Code)
	var preview struct {
		DryRun  bool `json:"dry_run"`
		Devices []struct {
			ID string `json:"id"`
		} `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &preview))
	require.True(t, preview.DryRun)
	require.Len(t, preview.Devices, 1)

	w = doGET(t, r, "/devices?per_page=100")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Equal(t, 0, list.Total)
}

// Given: a tag query
// When: POSTing /discover/adopt?tags=lab,prod
// Then: 201 with tags on the device; invalid tags 422.
func TestAdoptTags(t *testing.T) {
	r := discoverFullRouter(t, 0, "")

	w := doPOST(t, r, "/discover/adopt?tags=lab,prod", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:62"}]}`)
	require.Equal(t, http.StatusCreated, w.Code)

	w = doGET(t, r, "/devices?per_page=100")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Devices []models.Device `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Devices, 1)
	require.Equal(t, []string{"lab", "prod"}, list.Devices[0].Tags)

	w = doPOST(t, r, "/discover/adopt?tags=BAD-TAG!", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:63"}]}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// Given: a listener on loopback and no MAC-bearing hosts
// When: POSTing /discover/adopt-all?cidr=127.0.0.1/30
// Then: 201 with empty devices and honest counts. The explicit CIDR keeps
// the test hermetic: a bare adopt-all would scan the real LAN and flake
// wherever a gateway answers (seen on CI runners).
func TestAdoptAll(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	r := discoverFullRouter(t, ln.Addr().(*net.TCPAddr).Port, "")

	w := doPOST(t, r, "/discover/adopt-all?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusCreated, w.Code)
	var body struct {
		Devices []any `json:"devices"`
		Scanned int   `json:"scanned"`
		Skipped int   `json:"skipped"`
		Ignored int   `json:"ignored"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Empty(t, body.Devices)
	require.GreaterOrEqual(t, body.Scanned, 1)
	require.GreaterOrEqual(t, body.Skipped, 1)
	require.Equal(t, 0, body.Ignored)
}

// Given: an ignore list
// When: driving the ignore CRUD
// Then: 201/200/404 with created/removed flags, bad input 422.
func TestIgnoreLifecycle(t *testing.T) {
	r := discoverFullRouter(t, 0, t.TempDir()+"/ignored.json")

	w := doPOST(t, r, "/discover/ignore", `{"mac":"AA:BB:CC:DD:EE:71"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var added struct {
		Created bool `json:"created"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &added))
	require.True(t, added.Created)

	w = doGET(t, r, "/discover/ignore")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Ignored []discover.Entry `json:"ignored"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Ignored, 1)
	require.Equal(t, "aa:bb:cc:dd:ee:71", list.Ignored[0].MAC)

	w = doPOST(t, r, "/discover/ignore", `{"mac":"AA:BB:CC:DD:EE:71"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &added))
	require.False(t, added.Created)

	w = doPOST(t, r, "/discover/ignore", `{}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	w = doDELETE(t, r, "/discover/ignore?mac=aa:bb:cc:dd:ee:71")
	require.Equal(t, http.StatusOK, w.Code)
	var removed struct {
		Removed bool `json:"removed"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &removed))
	require.True(t, removed.Removed)

	w = doDELETE(t, r, "/discover/ignore?mac=aa:bb:cc:dd:ee:71")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// Given: loopback on the denylist
// When: scanning its /30
// Then: loopback is filtered and counted as ignored.
func TestScanSkipsIgnored(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	r := discoverFullRouter(t, ln.Addr().(*net.TCPAddr).Port, t.TempDir()+"/ignored.json")

	w := doPOST(t, r, "/discover/ignore", `{"ip":"127.0.0.1"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	w = doPOST(t, r, "/discover?cidr=127.0.0.1/30", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Hosts   []discover.Host `json:"hosts"`
		Ignored int             `json:"ignored"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 1, body.Ignored)
	for _, h := range body.Hosts {
		require.NotEqual(t, "127.0.0.1", h.IP)
	}
}
