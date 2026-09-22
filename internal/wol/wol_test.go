package wol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cleeryy/hello/internal/wol"
)

func Test_SendWOLPacket_rejects_invalid_mac(t *testing.T) {
	// Given
	mac := "not-a-mac"

	// When
	err := wol.SendWOLPacket(mac, "")

	// Then
	require.ErrorIs(t, err, wol.ErrInvalidMAC)
}

func Test_BroadcastForIP_derives_directed_broadcast(t *testing.T) {
	require.Equal(t, "192.168.1.255", wol.BroadcastForIP("192.168.1.50", "255.255.255.255"))
	require.Equal(t, "255.255.255.255", wol.BroadcastForIP("", "255.255.255.255"))
	require.Equal(t, "10.0.0.255", wol.BroadcastForIP("not-an-ip", "10.0.0.255"))
	require.Equal(t, wol.DefaultBroadcastIP, wol.BroadcastForIP("", ""))
}
