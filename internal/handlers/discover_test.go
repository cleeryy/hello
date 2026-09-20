package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/storage"
)

func discoverRouter(t *testing.T, lnPort int) (*gin.Engine, *discover.Scanner) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	_, subnet, err := net.ParseCIDR("127.0.0.1/30")
	require.NoError(t, err)
	d := discover.New()
	d.Subnet = subnet
	d.ProbePorts = []int{lnPort}
	d.ResolveHostname = func(string) string { return "" }
	srv := New(&config.Config{}, storage.New(t.TempDir()+"/d.json"), nil).WithDiscover(d)
	r := gin.New()
	srv.Mount(r)
	return r, d
}

func doPOST(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Given: a listener on loopback and a scanner pinned to its /30
// When: POSTing /discover
// Then: 200 with a host envelope containing loopback, known=false.
func TestDiscover_whenScan(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	r, _ := discoverRouter(t, ln.Addr().(*net.TCPAddr).Port)

	w := doPOST(t, r, "/discover", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Hosts []discover.Host `json:"hosts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	found := false
	for _, h := range body.Hosts {
		require.NotEmpty(t, h.IP)
		if h.IP == "127.0.0.1" {
			found = true
			require.False(t, h.Known)
		}
	}
	require.True(t, found, "loopback missing: %+v", body.Hosts)
}

// Given: a scan just finished
// When: POSTing /discover again at once
// Then: 429 problem+json, the cooldown guards the network.
func TestDiscover_whenImmediateRescan(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	r, _ := discoverRouter(t, ln.Addr().(*net.TCPAddr).Port)

	require.Equal(t, http.StatusOK, doPOST(t, r, "/discover", "").Code)
	w := doPOST(t, r, "/discover", "")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
}

// Given: unknown candidate hosts with MACs
// When: POSTing /discover/adopt
// Then: 201 creates them as unknown devices, re-adopting 409s.
func TestAdopt_whenNewHosts(t *testing.T) {
	r, _ := discoverRouter(t, 0)

	w := doPOST(t, r, "/discover/adopt", `{"hosts":[
		{"mac":"AA:BB:CC:DD:EE:01","ip":"192.168.7.11","name":"box-one"},
		{"mac":"AA:BB:CC:DD:EE:02","ip":"192.168.7.12"}
	]}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var body struct {
		Devices []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Devices, 2)
	require.Equal(t, "box-one", body.Devices[0].Name)
	require.NotEmpty(t, body.Devices[1].Name)

	w = doPOST(t, r, "/discover/adopt", `{"hosts":[{"mac":"AA:BB:CC:DD:EE:01"}]}`)
	require.Equal(t, http.StatusConflict, w.Code)
}

// Given: adopt payloads with bad MAC, bad IP, or empty list
// When: POSTing /discover/adopt
// Then: 422 problem+json, nothing stored.
func TestAdopt_whenInvalid(t *testing.T) {
	r, _ := discoverRouter(t, 0)
	for _, payload := range []string{
		`{"hosts":[]}`,
		`{"hosts":[{"mac":"not-a-mac"}]}`,
		`{"hosts":[{"mac":"AA:BB:CC:DD:EE:03","ip":"999.1.1.1"}]}`,
	} {
		w := doPOST(t, r, "/discover/adopt", payload)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, payload)
		require.Contains(t, w.Header().Get("Content-Type"), "application/problem+json")
	}
}

var _ = context.Background
