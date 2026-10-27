package main

import (
	"io"
	"net/http"
	"syscall"
	"testing"
	"time"
)

// Given: a full valid environment on temp files
// When: running the server then SIGTERMing the test binary
// Then: /health answers and run shuts down cleanly (covers main wiring).
func TestRunStartsAndStops(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("API_TOKEN", "WireTestToken-001")
	t.Setenv("DEFAULT_MAC", "AA:BB:CC:DD:EE:FF")
	t.Setenv("PORT", "18099")
	t.Setenv("DEVICES_FILE", dir+"/devices.json")
	t.Setenv("HISTORY_FILE", dir+"/history.json")
	t.Setenv("SCHEDULES_FILE", dir+"/schedules.json")
	t.Setenv("IGNORED_FILE", dir+"/ignored.json")

	errCh := make(chan error, 1)
	go func() { errCh <- run() }()
	up := false
	for i := 0; i < 200 && !up; i++ {
		resp, err := http.Get("http://localhost:18099/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			up = resp.StatusCode == http.StatusOK && len(body) > 0
		}
		if !up {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !up {
		t.Fatal("server never came up")
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("signal self: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run did not stop after SIGTERM")
	}
}
