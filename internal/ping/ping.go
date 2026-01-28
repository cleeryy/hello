package ping

import (
	"fmt"
	"net"
	"time"

	"github.com/go-ping/ping"
)

func PingICMP(ipAddress string, timeout time.Duration) bool {
	pinger, err := ping.NewPinger(ipAddress)
	if err != nil {
		return false
	}

	pinger.Count = 1
	pinger.Timeout = timeout

	err = pinger.Run()
	if err != nil {
		return false
	}

	return pinger.Statistics().PacketsRecv > 0
}

func PingTCP(ipAddress string, timeout time.Duration) bool {
	address := fmt.Sprintf("%s:22", ipAddress)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err == nil {
		defer conn.Close()
		return true
	}

	address = fmt.Sprintf("%s:80", ipAddress)
	conn, err = net.DialTimeout("tcp", address, timeout)
	if err == nil {
		defer conn.Close()
		return true
	}

	return false
}

func PingHost(ipAddress string, timeout time.Duration) bool {
	if PingICMP(ipAddress, timeout) {
		return true
	}

	return PingTCP(ipAddress, timeout)
}
