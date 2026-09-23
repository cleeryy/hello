package notify_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/notify"
)

// Given: a blank URL or nil sender
// When: Send is called
// Then: nothing happens, no panic, no block.
func TestSender_disabled(t *testing.T) {
	s := notify.New("")
	require.False(t, s.Enabled())
	s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	var nilSender *notify.Sender
	require.False(t, nilSender.Enabled())
	nilSender.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
}

// Given: a live webhook endpoint
// When: a wake payload is sent
// Then: the endpoint receives the JSON with the device fields.
func TestSender_delivers(t *testing.T) {
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
	s := notify.New(srv.URL)
	require.True(t, s.Enabled())
	s.Send(notify.WakePayload{DeviceID: "pc1", MAC: "00:11:22:33:44:55", Success: true, Attempts: 1, Trigger: "manual"})
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) > 0
	}, 3*time.Second, 20*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, string(got), `"device_id":"pc1"`)
	require.Contains(t, string(got), `"success":true`)
}
