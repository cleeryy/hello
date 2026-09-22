package wol

import (
	"errors"
	"fmt"
	"net"

	"github.com/linde12/gowol"
)

// ErrInvalidMAC is returned when the MAC address cannot be parsed.
var ErrInvalidMAC = errors.New("wol: invalid mac address")

// DefaultBroadcastIP is used when no broadcast address is provided.
const DefaultBroadcastIP = "255.255.255.255"

// BroadcastForIP derives a /24 directed broadcast from a device IPv4
// (192.168.1.50 -> 192.168.1.255) so magic packets target the right
// subnet instead of the global broadcast, which many interfaces refuse.
// Falls back to fallback (then DefaultBroadcastIP) for empty/non-IPv4.
func BroadcastForIP(deviceIP, fallback string) string {
	if ip := net.ParseIP(deviceIP); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return fmt.Sprintf("%d.%d.%d.255", v4[0], v4[1], v4[2])
		}
	}
	if fallback != "" {
		return fallback
	}
	return DefaultBroadcastIP
}

// SendWOLPacket sends a magic packet to macAddress via broadcastIP.
func SendWOLPacket(macAddress, broadcastIP string) error {
	if _, err := net.ParseMAC(macAddress); err != nil {
		return fmt.Errorf("wol: mac %q: %w", macAddress, ErrInvalidMAC)
	}
	if broadcastIP == "" {
		broadcastIP = DefaultBroadcastIP
	}
	packet, err := gowol.NewMagicPacket(macAddress)
	if err != nil {
		return fmt.Errorf("wol: build packet: %w", err)
	}
	if err := packet.Send(broadcastIP); err != nil {
		return fmt.Errorf("wol: send to %s via %s: %w", macAddress, broadcastIP, err)
	}
	return nil
}
