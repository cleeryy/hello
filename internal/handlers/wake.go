package handlers

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
)

func (s *Server) wakeDefault(c *gin.Context) {
	s.sendMagic(c, s.cfg.DefaultMAC)
}

func (s *Server) wakeMAC(c *gin.Context) {
	s.sendMagic(c, c.Param("macAddress"))
}

func (s *Server) wakeDevice(c *gin.Context) {
	device, err := s.store.Get(c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	s.sendMagic(c, device.MAC)
}

func (s *Server) sendMagic(c *gin.Context, mac string) {
	if _, err := net.ParseMAC(mac); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid mac address %q", mac), nil)
		return
	}
	if err := s.sendWOL(mac, s.cfg.BroadcastIP); err != nil {
		slog.Error("wol send failed", slog.String("mac", mac), slog.Any("err", err))
		s.recordWake(mac, false, err.Error())
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"failed to send magic packet", nil)
		return
	}
	s.recordWake(mac, true, "")
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("magic packet sent to %s !", mac),
	})
}

// recordWake logs a manual wake to history, resolving the registry id when
// the MAC belongs to a known device. A missing history or a logging failure
// never breaks the wake response itself.
func (s *Server) recordWake(mac string, success bool, errMsg string) {
	if s.hist == nil {
		return
	}
	deviceID, _ := s.store.LookupMAC(mac)
	entry := models.WakeEvent{
		DeviceID: deviceID,
		MAC:      mac,
		Trigger:  models.TriggerManual,
		Success:  success,
		Error:    errMsg,
	}
	if deviceID == "" {
		entry.DeviceID = mac
	}
	if _, err := s.hist.Record(entry); err != nil {
		slog.Warn("history record failed", slog.Any("err", err))
	}
}
