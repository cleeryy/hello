package handlers

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
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
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"failed to send magic packet", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("magic packet sent to %s !", mac),
	})
}
