package handlers

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/ping"
	"github.com/cleeryy/hello/internal/wol"
)

func (s *Server) wakeDefault(c *gin.Context) {
	note, broadcast, ok := parseWakeOptions(c, s.cfg.BroadcastIP)
	if !ok {
		return
	}
	if isDryRun(c) {
		s.writeDryRun(c, s.cfg.DefaultMAC, broadcast)
		return
	}
	s.sendMagic(c, s.cfg.DefaultMAC, broadcast, note)
}

func (s *Server) wakeMAC(c *gin.Context) {
	mac := c.Param("macAddress")
	fallback := s.cfg.BroadcastIP
	if id, found := s.store.LookupMAC(mac); found {
		if dev, err := s.store.Get(id); err == nil {
			fallback = wol.BroadcastForIP(dev.IP, s.cfg.BroadcastIP)
		}
	}
	note, broadcast, ok := parseWakeOptions(c, fallback)
	if !ok {
		return
	}
	if isDryRun(c) {
		s.writeDryRun(c, mac, broadcast)
		return
	}
	s.sendMagic(c, mac, broadcast, note)
}

func (s *Server) wakeDevice(c *gin.Context) {
	device, err := s.store.Get(c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	retries, interval, ok := parseRetryParams(c)
	if !ok {
		return
	}
	note, broadcast, ok := parseWakeOptions(c, wol.BroadcastForIP(device.IP, s.cfg.BroadcastIP))
	if !ok {
		return
	}
	if isDryRun(c) {
		s.writeDryRun(c, device.MAC, broadcast)
		return
	}
	if retryAfter := s.wakeCooldown(device); retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		writeProblem(c, http.StatusTooManyRequests, "too many requests",
			fmt.Sprintf("device %q cooling down, retry in %ds", device.ID, retryAfter), nil)
		return
	}
	if retries == 0 {
		s.sendMagic(c, device.MAC, broadcast, note)
		return
	}
	if strings.TrimSpace(device.IP) == "" {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"retry needs a device IP to check reachability", nil)
		return
	}
	s.wakeWithRetry(c, device, broadcast, note, retries, interval)
}

// wakeCooldown returns the seconds left before device may be woken again,
// 0 when no per-device cooldown applies.
func (s *Server) wakeCooldown(device *models.Device) int {
	cooldown := s.cfg.WakeCooldownSec
	if cooldown <= 0 || device.LastWakeAt <= 0 {
		return 0
	}
	left := cooldown - int(time.Now().Unix()-device.LastWakeAt)
	if left < 0 {
		return 0
	}
	return left
}

// isDryRun reports whether the caller only wants validation, no packet.
func isDryRun(c *gin.Context) bool {
	switch strings.ToLower(strings.TrimSpace(c.Query("dry_run"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func (s *Server) writeDryRun(c *gin.Context, mac, broadcast string) {
	if _, err := net.ParseMAC(mac); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid mac address %q", mac), nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"dry_run":   true,
		"mac":       mac,
		"broadcast": broadcast,
	})
}

// parseWakeOptions reads the optional ?note= annotation (or JSON body note)
// and the ?broadcast= override, falling back to the computed broadcast.
func parseWakeOptions(c *gin.Context, fallback string) (string, string, bool) {
	note := strings.TrimSpace(c.Query("note"))
	if note == "" && c.Request.ContentLength != 0 {
		var body struct {
			Note string `json:"note"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeProblem(c, http.StatusBadRequest, "bad request", "malformed json body", nil)
			return "", "", false
		}
		note = strings.TrimSpace(body.Note)
	}
	if len([]rune(note)) > 140 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"note must be at most 140 characters", nil)
		return "", "", false
	}
	broadcast := fallback
	if raw := strings.TrimSpace(c.Query("broadcast")); raw != "" {
		ip := net.ParseIP(raw)
		if ip == nil || ip.To4() == nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("broadcast %q is not a valid IPv4 address", raw), nil)
			return "", "", false
		}
		broadcast = raw
	}
	return note, broadcast, true
}

// Retry bounds keep the worst case inside the 15s server write timeout:
// at most 8s of polling sleeps plus one 1s ping per attempt.
const (
	maxWakeRetries = 5
	minRetryIntS   = 1
	maxRetryIntS   = 5
	dfltRetryIntS  = 2
	maxRetryCostS  = 8
	retryPingWait  = time.Second
)

// parseRetryParams validates ?retries= (0..5) and ?interval= (1..5s).
func parseRetryParams(c *gin.Context) (int, time.Duration, bool) {
	fail := func(detail string) (int, time.Duration, bool) {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", detail, nil)
		return 0, 0, false
	}
	retries := 0
	if raw := c.Query("retries"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > maxWakeRetries {
			return fail(fmt.Sprintf("retries must be an integer 0..%d", maxWakeRetries))
		}
		retries = n
	}
	interval := time.Duration(dfltRetryIntS) * time.Second
	if raw := c.Query("interval"); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil || secs < minRetryIntS || secs > maxRetryIntS {
			return fail(fmt.Sprintf("interval must be an integer %d..%d (seconds)", minRetryIntS, maxRetryIntS))
		}
		interval = time.Duration(secs) * time.Second
	}
	if retries*int(interval.Seconds()) > maxRetryCostS {
		return fail(fmt.Sprintf("retries x interval must stay within %ds (server write timeout)", maxRetryCostS))
	}
	return retries, interval, true
}

// wakeWithRetry resends the magic packet until the device answers a ping or
// retries run out. One history entry records the whole sequence.
func (s *Server) wakeWithRetry(c *gin.Context, device *models.Device, broadcast, note string, retries int, interval time.Duration) {
	if _, err := net.ParseMAC(device.MAC); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid mac address %q", device.MAC), nil)
		return
	}
	check := s.pingHost
	if check == nil {
		check = ping.PingHost
	}
	ip := strings.TrimSpace(device.IP)
	attempts, up, sentOK, errMsg := 0, false, true, ""
	for i := 0; i <= retries; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		if err := s.sendWOL(device.MAC, broadcast); err != nil {
			slog.Error("wol send failed", slog.String("mac", device.MAC), slog.String("broadcast", broadcast), slog.Any("err", err))
			sentOK, errMsg = false, err.Error()
			break
		}
		attempts++
		if check(ip, retryPingWait) {
			up = true
			break
		}
	}
	s.recordWake(device.MAC, sentOK, errMsg, attempts, note)
	if !sentOK {
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"failed to send magic packet", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("magic packet sent to %s !", device.MAC),
		"attempts": attempts,
		"up":       up,
	})
}

func (s *Server) sendMagic(c *gin.Context, mac, broadcastIP, note string) {
	if _, err := net.ParseMAC(mac); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid mac address %q", mac), nil)
		return
	}
	if err := s.sendWOL(mac, broadcastIP); err != nil {
		slog.Error("wol send failed", slog.String("mac", mac), slog.String("broadcast", broadcastIP), slog.Any("err", err))
		s.recordWake(mac, false, err.Error(), 1, note)
		writeProblem(c, http.StatusInternalServerError, "internal error",
			"failed to send magic packet", nil)
		return
	}
	s.recordWake(mac, true, "", 1, note)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("magic packet sent to %s !", mac),
	})
}

// recordWake logs a manual wake to history, resolving the registry id when
// the MAC belongs to a known device. Wake counters bump even without a
// history store: they are registry state, not log state. A missing history
// or a logging failure never breaks the wake response itself.
func (s *Server) recordWake(mac string, success bool, errMsg string, attempts int, note string) {
	deviceID, _ := s.store.LookupMAC(mac)
	if success && deviceID != "" {
		if err := s.store.RecordWake(deviceID, time.Now().Unix()); err != nil {
			slog.Warn("wake counter update failed", slog.String("device", deviceID), slog.Any("err", err))
		}
	}
	if s.hist == nil {
		return
	}
	entry := models.WakeEvent{
		DeviceID: deviceID,
		MAC:      mac,
		Trigger:  models.TriggerManual,
		Success:  success,
		Attempts: attempts,
		Note:     note,
		Error:    errMsg,
	}
	if deviceID == "" {
		entry.DeviceID = mac
	}
	if _, err := s.hist.Record(entry); err != nil {
		slog.Warn("history record failed", slog.Any("err", err))
	}
}
