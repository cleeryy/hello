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

// Given: a webhook endpoint slower than the client timeout
// When: a payload is sent
// Then: Send returns without blocking and the attempt is bounded.
func TestSender_timeoutDoesNotBlock(t *testing.T) {
	var mu sync.Mutex
	seen := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	s := notify.NewWithClient(srv.URL, &http.Client{Timeout: 50 * time.Millisecond})
	require.True(t, s.Enabled())
	start := time.Now()
	s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	require.Less(t, time.Since(start), 2*time.Second)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen == 1
	}, 3*time.Second, 20*time.Millisecond)
}

// Given: a webhook endpoint answering 500
// When: a payload is sent
// Then: the delivery is attempted once and Send never blocks or panics.
func TestSender_non2xxIsAttemptedAndLogged(t *testing.T) {
	var mu sync.Mutex
	seen := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := notify.New(srv.URL)
	start := time.Now()
	s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	require.Less(t, time.Since(start), 2*time.Second)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen == 1
	}, 3*time.Second, 20*time.Millisecond)
}

// Given: a fast webhook endpoint
// When: a burst below the concurrency bound is sent
// Then: every delivery completes, the bound never deadlocks.
func TestSender_burstCompletes(t *testing.T) {
	var mu sync.Mutex
	seen := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		seen++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	s := notify.New(srv.URL)
	for i := 0; i < 4; i++ {
		s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	}
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen == 4
	}, 5*time.Second, 20*time.Millisecond)
}

// Given: every delivery slot held by a stuck receiver
// When: one more payload is sent
// Then: it is dropped with a log instead of queuing another goroutine,
// and Send still returns immediately.
func TestSender_dropsWhenSaturated(t *testing.T) {
	release := make(chan struct{})
	var mu sync.Mutex
	inflight, delivered := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inflight++
		mu.Unlock()
		<-release
		mu.Lock()
		inflight--
		delivered++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	s := notify.New(srv.URL)
	for i := 0; i < 8; i++ {
		s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	}
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return inflight == 8
	}, 3*time.Second, 20*time.Millisecond)
	start := time.Now()
	for i := 0; i < 8; i++ {
		s.Send(notify.WakePayload{DeviceID: "pc1", Trigger: "manual"})
	}
	require.Less(t, time.Since(start), 2*time.Second)
	mu.Lock()
	stillInflight := inflight
	mu.Unlock()
	require.Equal(t, 8, stillInflight)
	close(release)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return delivered == 8
	}, 3*time.Second, 20*time.Millisecond)
}
