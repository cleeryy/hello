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
