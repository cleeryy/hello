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
