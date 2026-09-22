package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/discover"
	"github.com/cleeryy/hello/internal/models"
)

// scanNetwork runs one LAN sweep, annotating candidates already in the
// registry. Busy and cooling-down scans surface as 429, everything else
// keeps the shared problem+json shape.
func (s *Server) scanNetwork(c *gin.Context) {
	if s.disc == nil {
		writeProblem(c, http.StatusServiceUnavailable, "unavailable",
			"discovery is not configured", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	hosts, err := s.disc.Scan(ctx)
	switch {
	case err == nil:
	case errors.Is(err, discover.ErrScanBusy), errors.Is(err, discover.ErrCoolingDown):
		if retry := s.disc.RetryIn(); retry > 0 {
			c.Header("Retry-After", fmt.Sprint(int(retry.Seconds())+1))
		}
		writeProblem(c, http.StatusTooManyRequests, "too many requests",
			"a scan is already running or cooling down", nil)
		return
	default:
		writeErr(c, err)
		return
	}
	for i := range hosts {
		_, hosts[i].Known = s.store.LookupMAC(hosts[i].MAC)
	}
	c.JSON(http.StatusOK, gin.H{"hosts": hosts})
}

// adoptHosts imports discovery candidates as registry devices. Validation and
// duplicate checks happen before one locked, atomic registry write.
func (s *Server) adoptHosts(c *gin.Context) {
	if s.adoptLimit != nil && !s.adoptLimit.allow(c.ClientIP()) {
		c.Header("Retry-After", fmt.Sprint(s.adoptLimit.retryAfter(c.ClientIP())))
		writeProblem(c, http.StatusTooManyRequests, "too many requests",
			"adopt rate limit exceeded, retry later", nil)
		return
	}
	var in struct {
		Hosts []struct {
			MAC      string `json:"mac" binding:"required"`
			IP       string `json:"ip"`
			Name     string `json:"name"`
			Hostname string `json:"hostname"`
		} `json:"hosts" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeBindingError(c, err)
		return
	}

	devices := make([]*models.Device, 0, len(in.Hosts))
	seenMAC := make(map[string]struct{}, len(in.Hosts))
	for _, h := range in.Hosts {
		mac, err := net.ParseMAC(h.MAC)
		if err != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("invalid mac %q", h.MAC),
				gin.H{"errors": map[string][]string{"hosts": {"each host needs a valid mac"}}})
			return
		}
		canonicalMAC := mac.String()
		if _, duplicate := seenMAC[canonicalMAC]; duplicate {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("duplicate mac %q in adoption batch", canonicalMAC), nil)
			return
		}
		seenMAC[canonicalMAC] = struct{}{}
		if h.IP != "" && net.ParseIP(h.IP) == nil {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("invalid ip %q", h.IP),
				gin.H{"errors": map[string][]string{"hosts": {"ip must be a valid address"}}})
			return
		}
		if _, found := s.store.LookupMAC(canonicalMAC); found {
			writeProblem(c, http.StatusConflict, "already exists",
				fmt.Sprintf("device with mac %s already exists", canonicalMAC),
				nil)
			return
		}
		name := h.Name
		if name == "" {
			name = h.Hostname
		}
		if name == "" {
			name = "host-" + strings.ReplaceAll(h.IP, ".", "-")
		}
		devices = append(devices, &models.Device{
			ID:          "host-" + strings.ReplaceAll(strings.ToLower(canonicalMAC), ":", ""),
			Name:        name,
			MAC:         canonicalMAC,
			IP:          h.IP,
			Status:      models.StatusUnknown,
			PingEnabled: h.IP != "",
		})
	}

	created, err := s.store.CreateMany(devices)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"devices": created})
}
