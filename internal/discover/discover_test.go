package discover

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// Given: a Darwin/Linux ARP table dump
// When: parsing it
// Then: only entries with a valid MAC are kept, keyed by IP.
func TestParseARPTable_whenDarwinAndLinuxFormats(t *testing.T) {
	out := `? (192.168.1.1) at ab:cd:ef:01:02:03 on en0 ifscope [ethernet]
? (192.168.1.10) at 11:22:33:44:55:66 on en0 ifscope [ethernet]
? (192.168.1.11) at <incomplete> on en0
server (10.0.0.5) at aa:bb:cc:dd:ee:ff [ether] on eth0
`
	got := parseARPTable(out)
	require.Len(t, got, 3)
	require.Equal(t, "ab:cd:ef:01:02:03", got["192.168.1.1"])
	require.Equal(t, "11:22:33:44:55:66", got["192.168.1.10"])
	require.Equal(t, "aa:bb:cc:dd:ee:ff", got["10.0.0.5"])
}

// Given: a listener on loopback and a scanner pinned to 127.0.0.0/30
// When: scanning
// Then: the loopback address shows up even without an ARP entry.
func TestScan_whenLoopbackSubnet(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	_, subnet, err := net.ParseCIDR("127.0.0.1/30")
	require.NoError(t, err)
	s := New()
	s.Subnet = subnet
	s.ProbePorts = []int{port}
	s.ResolveHostname = func(string) string { return "" }

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	hosts, err := s.Scan(ctx)
	require.NoError(t, err)
	ips := make([]string, 0, len(hosts))
	for _, h := range hosts {
		ips = append(ips, h.IP)
	}
	require.Contains(t, ips, "127.0.0.1")
}

// Given: a scan already running
// When: a second scan starts
// Then: it fails fast with ErrScanBusy.
func TestScan_whenConcurrent(t *testing.T) {
	s := New()
	release := make(chan struct{})
	s.sweep = func(context.Context, *net.IPNet) map[string]struct{} {
		<-release
		return map[string]struct{}{}
	}
	defer close(release)

	done := make(chan error, 1)
	go func() {
		_, err := s.Scan(context.Background())
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	_, err := s.Scan(context.Background())
	require.ErrorIs(t, err, ErrScanBusy)
	release <- struct{}{}
	require.NoError(t, <-done)
}

// Given: a just-finished scan
// When: scanning again within the cooldown
// Then: it fails with ErrCoolingDown.
func TestScan_whenWithinCooldown(t *testing.T) {
	s := New()
	s.cooldown = 5 * time.Second
	s.sweep = func(context.Context, *net.IPNet) map[string]struct{} {
		return map[string]struct{}{}
	}
	s.readARP = func(context.Context) map[string]string { return map[string]string{} }

	_, err := s.Scan(context.Background())
	require.NoError(t, err)
	_, err = s.Scan(context.Background())
	require.ErrorIs(t, err, ErrCoolingDown)
}
