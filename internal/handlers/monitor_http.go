package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/monitor"
)

// checkMonitor runs one immediate polling pass and reports what it saw.
func (s *Server) checkMonitor(c *gin.Context) {
	if id := strings.TrimSpace(c.Query("device_id")); id != "" {
		if _, err := s.store.Get(id); err != nil {
			writeErr(c, err)
			return
		}
		checked, changed := s.mon.CheckOne(id)
		c.JSON(http.StatusOK, gin.H{"checked": checked, "changed": changed, "device_id": id})
		return
	}
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

// deviceUptime reports one device's honest poll summary (F-57): raw check
// counters from the monitor plus the latest recorded change and the
// monitor start time. Unknown devices 404; never-polled devices report
// zero checks with the monitor start time.
func (s *Server) deviceUptime(c *gin.Context) {
	dev, err := s.store.Get(c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	detail, ok := s.mon.Detail(dev.ID)
	if !ok {
		detail = monitor.DeviceUptime{Since: s.mon.Since()}
	}
	up, total := detail.Up, detail.Total
	down := total - up
	percent := 0.0
	if total > 0 {
		percent = float64(up) / float64(total) * 100
	}
	c.JSON(http.StatusOK, gin.H{
		"device_id":   dev.ID,
		"checks":      total,
		"up":          up,
		"down":        down,
		"percent":     percent,
		"last_change": detail.LastChange,
		"since":       detail.Since,
	})
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
