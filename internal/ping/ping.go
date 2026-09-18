package ping

import (
	"context"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

// PingICMP reports whether ipAddress answers a single ICMP echo request.
// It delegates to the operating system's ping binary, which owns the raw
// socket privileges, instead of a third-party Go implementation.
func PingICMP(ipAddress string, timeout time.Duration) bool {
	if ipAddress == "" {
		return false
	}
	secs := max(1, int(timeout.Seconds()))
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"-c", "1", "-t", strconv.Itoa(secs), ipAddress}
	case "windows":
		args = []string{"-n", "1", "-w", strconv.Itoa(int(timeout.Milliseconds())), ipAddress}
	default: // linux, busybox, and friends
		args = []string{"-c", "1", "-W", strconv.Itoa(secs), ipAddress}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "ping", args...).Run() == nil
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
