package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// doRequestRaw sends an untouched body, for malformed-JSON cases.
func doRequestRaw(s *Server, method, target, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine := gin.New()
	s.Mount(engine)
	engine.ServeHTTP(w, req)
	return w
}

type oneshotJSON struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	MAC      string `json:"mac"`
	At       int64  `json:"at"`
	Note     string `json:"note"`
}

// Given: a device and a future timestamp
// When: a one-shot is created, listed, then cancelled
// Then: 201 with resolved MAC, visible in list, gone after delete.
func TestOneshot_lifecycle(t *testing.T) {
	s := newTestServer(t)
	w := doRequest(s, http.MethodPost, "/devices",
		map[string]any{"id": "nas", "name": "NAS", "mac": "AA:BB:CC:DD:EE:FF"})
	require.Equal(t, http.StatusCreated, w.Code)

	at := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	w = doRequest(s, http.MethodPost, "/wake/oneshots", map[string]any{
		"device_id": "nas", "at": at, "note": "backup window",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var created oneshotJSON
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, "nas", created.DeviceID)
	require.Equal(t, "AA:BB:CC:DD:EE:FF", created.MAC)
	require.Equal(t, "backup window", created.Note)

	w = doRequest(s, http.MethodGet, "/wake/oneshots", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Oneshots []oneshotJSON `json:"oneshots"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Oneshots, 1)
	require.Equal(t, created.ID, list.Oneshots[0].ID)

	w = doRequest(s, http.MethodDelete, "/wake/oneshots/"+created.ID, nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	w = doRequest(s, http.MethodDelete, "/wake/oneshots/"+created.ID, nil)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = doRequest(s, http.MethodGet, "/wake/oneshots", nil)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Empty(t, list.Oneshots)
}

// Given: invalid one-shot payloads
// When: posted (one throttled POST per fresh server)
// Then: the documented status codes.
func TestOneshot_validation(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	far := time.Now().Add(25 * time.Hour).UTC().Format(time.RFC3339)
	cases := []struct {
		name string
		body string
		code int
	}{
		{"both targets", `{"device_id":"a","mac":"AA:BB:CC:DD:EE:FF","at":"` + future + `"}`, http.StatusUnprocessableEntity},
		{"no target", `{"at":"` + future + `"}`, http.StatusUnprocessableEntity},
		{"bad mac", `{"mac":"nope","at":"` + future + `"}`, http.StatusUnprocessableEntity},
		{"unknown device", `{"device_id":"ghost","at":"` + future + `"}`, http.StatusNotFound},
		{"bad timestamp", `{"mac":"AA:BB:CC:DD:EE:FF","at":"tomorrow"}`, http.StatusUnprocessableEntity},
		{"past timestamp", `{"mac":"AA:BB:CC:DD:EE:FF","at":"` + past + `"}`, http.StatusUnprocessableEntity},
		{"too far", `{"mac":"AA:BB:CC:DD:EE:FF","at":"` + far + `"}`, http.StatusUnprocessableEntity},
		{"malformed json", `{"mac":`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t)
			w := doRequestRaw(s, http.MethodPost, "/wake/oneshots", tc.body)
			require.Equal(t, tc.code, w.Code)
		})
	}
}
