package main

import (
	"github.com/linde12/gowol"
)

func SendWOLPacket(macAddress string) error {
	packet, err := gowol.NewMagicPacket(macAddress)
	if err != nil {
		return err
	}

	err = packet.Send("255.255.255.255")
	return err
}
