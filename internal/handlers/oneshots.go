package handlers

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/wol"
)

// One-shot bounds: a delayed wake fires between 5s and 24h in the future.
// Entries live in memory only and vanish on restart.
const (
	minOneshotDelay = 5 * time.Second
	maxOneshotDelay = 24 * time.Hour
)

// oneshotEntry is a pending delayed wake.
type oneshotEntry struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id,omitempty"`
	MAC      string `json:"mac"`
	At       int64  `json:"at"`
	Note     string `json:"note,omitempty"`
	timer    *time.Timer
}

// oneshotRegistry holds pending delayed wakes. Entries are ephemeral:
// a restart drops them, which the API documents honestly.
type oneshotRegistry struct {
	mu    sync.Mutex
	items map[string]*oneshotEntry
	seq   uint64
}

func newOneshotRegistry() *oneshotRegistry {
	return &oneshotRegistry{items: map[string]*oneshotEntry{}}
}

// createOneshot schedules a single delayed wake for a device or raw MAC.
func (s *Server) createOneshot(c *gin.Context) {
	var in struct {
		DeviceID string `json:"device_id"`
		MAC      string `json:"mac"`
		At       string `json:"at"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, "bad request", "malformed json body", nil)
		return
	}
	deviceID := strings.TrimSpace(in.DeviceID)
	mac := strings.TrimSpace(in.MAC)
	if (deviceID == "") == (mac == "") {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"exactly one of device_id or mac is required", nil)
		return
	}
	if deviceID != "" {
		dev, err := s.store.Get(deviceID)
		if err != nil {
			writeErr(c, err)
			return
		}
		mac = dev.MAC
	} else if _, err := net.ParseMAC(mac); err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			fmt.Sprintf("invalid mac address %q", mac), nil)
		return
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(in.At))
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"at must be an RFC3339 timestamp", nil)
		return
	}
	delay := time.Until(at)
	if delay < minOneshotDelay {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"at must be at least 5s in the future", nil)
		return
	}
	if delay > maxOneshotDelay {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"at must be within the next 24h", nil)
		return
	}
	note := strings.TrimSpace(in.Note)
	if len([]rune(note)) > 140 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"note must be at most 140 characters", nil)
		return
	}
	s.oneshots.mu.Lock()
	s.oneshots.seq++
	entry := &oneshotEntry{
		ID:       fmt.Sprintf("o-%d-%d", time.Now().UnixNano(), s.oneshots.seq),
		DeviceID: deviceID,
		MAC:      mac,
		At:       at.Unix(),
		Note:     note,
	}
	id := entry.ID
	entry.timer = time.AfterFunc(delay, func() { s.fireOneshot(id) })
	s.oneshots.items[id] = entry
	s.oneshots.mu.Unlock()
	c.JSON(http.StatusCreated, entry)
}

// listOneshots returns pending delayed wakes, soonest first.
func (s *Server) listOneshots(c *gin.Context) {
	s.oneshots.mu.Lock()
	out := make([]oneshotEntry, 0, len(s.oneshots.items))
	for _, e := range s.oneshots.items {
		out = append(out, *e)
	}
	s.oneshots.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	c.JSON(http.StatusOK, gin.H{"oneshots": out})
}

// cancelOneshot drops a pending delayed wake before it fires.
func (s *Server) cancelOneshot(c *gin.Context) {
	id := c.Param("id")
	s.oneshots.mu.Lock()
	entry, found := s.oneshots.items[id]
	if found {
		entry.timer.Stop()
		delete(s.oneshots.items, id)
	}
	s.oneshots.mu.Unlock()
	if !found {
		writeProblem(c, http.StatusNotFound, "not found",
			fmt.Sprintf("one-shot %q not found", id), nil)
		return
	}
	c.Status(http.StatusNoContent)
}

// fireOneshot sends the delayed packet and logs it as a manual wake.
func (s *Server) fireOneshot(id string) {
	s.oneshots.mu.Lock()
	entry, found := s.oneshots.items[id]
	if found {
		delete(s.oneshots.items, id)
	}
	s.oneshots.mu.Unlock()
	if !found {
		return
	}
	broadcast := s.cfg.BroadcastIP
	if id, found := s.store.LookupMAC(entry.MAC); found {
		if dev, err := s.store.Get(id); err == nil {
			broadcast = wol.BroadcastForIP(dev.IP, s.cfg.BroadcastIP)
		}
	}
	if err := s.sendWOL(entry.MAC, broadcast); err != nil {
		slog.Error("one-shot wol failed", slog.String("id", id), slog.Any("err", err))
		s.recordWake(entry.MAC, false, err.Error(), 1, entry.Note)
		return
	}
	s.recordWake(entry.MAC, true, "", 1, entry.Note)
}
