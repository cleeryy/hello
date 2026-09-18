package ping

import (
	"net"
	"time"

	goping "github.com/go-ping/ping"
)

// PingICMP reports whether ipAddress answers a single ICMP echo request.
func PingICMP(ipAddress string, timeout time.Duration) bool {
	pinger, err := goping.NewPinger(ipAddress)
	if err != nil {
		return false
	}
	pinger.Count = 1
	pinger.Timeout = timeout
	if err := pinger.Run(); err != nil {
		return false
	}
	return pinger.Statistics().PacketsRecv > 0
}

// PingTCP reports whether ipAddress accepts TCP on a well-known port.
func PingTCP(ipAddress string, timeout time.Duration) bool {
	for _, port := range []string{"22", "80"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipAddress, port), timeout)
		if err != nil {
			continue
		}
		_ = conn.Close()
		return true
	}
	return false
}

// PingHost reports whether a host is reachable, ICMP first then TCP fallback.
func PingHost(ipAddress string, timeout time.Duration) bool {
	if PingICMP(ipAddress, timeout) {
		return true
	}
	return PingTCP(ipAddress, timeout)
}
