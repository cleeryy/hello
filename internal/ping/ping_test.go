package ping

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestPingICMP_whenLoopback verifies the OS ping path answers locally.
func TestPingICMP_whenLoopback(t *testing.T) {
	// Given: the loopback interface, always up.
	// When: a single echo request with a short timeout.
	// Then: it answers.
	require.True(t, PingICMP("127.0.0.1", 2*time.Second))
}

// TestPingICMP_whenUnreachable verifies failures and bad input stay false.
func TestPingICMP_whenUnreachable(t *testing.T) {
	// Given: TEST-NET-1 (RFC 5737, never routable) and empty input.
	// When: echo requests are sent.
	// Then: both report unreachable without error.
	require.False(t, PingICMP("192.0.2.1", time.Second))
	require.False(t, PingICMP("", time.Second))
}
