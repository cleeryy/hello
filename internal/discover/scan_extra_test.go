package discover_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/discover"
)

func loopback30(t *testing.T) *net.IPNet {
	t.Helper()
	_, subnet, err := net.ParseCIDR("127.0.0.1/30")
	require.NoError(t, err)
	return subnet
}

// Given: a loopback sweep
// When: scanning then asking for status
// Then: RetryIn reports the cooldown and Status the last run.
func TestScanRetryInAndStatus(t *testing.T) {
	d := discover.New()
	d.ProbePorts = []int{9}
	d.Subnet = loopback30(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := d.Scan(ctx)
	require.NoError(t, err)
	require.Greater(t, int64(d.RetryIn().Seconds()), int64(0))

	rep := d.Status()
	require.False(t, rep.Scanning)
	require.Greater(t, rep.LastScan, int64(0))
}

// Given: a custom cooldown
// When: scanning twice at once
// Then: the second call cools down (custom branch).
func TestScanCustomCooldown(t *testing.T) {
	d := discover.New()
	d.ProbePorts = []int{9}
	d.Subnet = loopback30(t)
	d.SetCooldown(3600)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := d.Scan(ctx)
	require.NoError(t, err)
	_, err = d.Scan(ctx)
	require.ErrorIs(t, err, discover.ErrCoolingDown)
}

// Given: no subnet
// When: calling ScanSubnet with nil
// Then: an error, never a LAN-wide sweep.
func TestScanSubnetNil(t *testing.T) {
	d := discover.New()
	_, err := d.ScanSubnet(context.Background(), nil)
	require.Error(t, err)
}
