package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/monitor"
)

// checkMonitor runs one immediate polling pass and reports what it saw.
func (s *Server) checkMonitor(c *gin.Context) {
	checked, changed := s.mon.Check()
	c.JSON(http.StatusOK, gin.H{"checked": checked, "changed": changed})
}

// monitorTransitions returns the latest reachability changes, newest first.
func (s *Server) monitorTransitions(c *gin.Context) {
	got := s.mon.Transitions()
	if got == nil {
		got = []monitor.Transition{}
	}
	c.JSON(http.StatusOK, gin.H{"transitions": got})
}

type uptimeEntry struct {
	DeviceID string  `json:"device_id"`
	Up       int     `json:"up"`
	Total    int     `json:"total"`
	Percent  float64 `json:"percent"`
}

// monitorUptime reports per-device poll uptime since process start.
func (s *Server) monitorUptime(c *gin.Context) {
	out := []uptimeEntry{}
	for _, d := range s.store.GetAll() {
		up, total, ok := s.mon.Uptime(d.ID)
		if !ok || total == 0 {
			continue
		}
		out = append(out, uptimeEntry{
			DeviceID: d.ID,
			Up:       up,
			Total:    total,
			Percent:  float64(up) / float64(total) * 100,
		})
	}
	c.JSON(http.StatusOK, gin.H{"devices": out})
}

// monitorFlapping lists devices flapping right now (4+ changes in 5 minutes).
func (s *Server) monitorFlapping(c *gin.Context) {
	devices := s.mon.Flapping()
	if devices == nil {
		devices = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// monitorStatus reports the monitor tuning and its latest pass.
func (s *Server) monitorStatus(c *gin.Context) {
	interval, timeout, lastRun, lastChecked, lastChanged := s.mon.Status()
	c.JSON(http.StatusOK, gin.H{
		"interval_seconds": int64(interval.Seconds()),
		"timeout_seconds":  int64(timeout.Seconds()),
		"last_run":         lastRun,
		"last_checked":     lastChecked,
		"last_changed":     lastChanged,
	})
}
