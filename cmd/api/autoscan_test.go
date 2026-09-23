package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cleeryy/hello/internal/discover"
)

// Given: a scanner pinned to an oversized subnet
// When: running autoScan briefly
// Then: ticks fail fast with a warning and cancel stops the loop.
func TestAutoScan_errorAndDone(t *testing.T) {
	d := discover.New()
	_, subnet, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	d.Subnet = subnet

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		autoScan(ctx, d, 5*time.Millisecond)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("autoScan did not stop after cancel")
	}
}
