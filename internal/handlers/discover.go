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
// keeps the shared problem+json shape. ?cidr= scans an explicit range
// instead of the auto-detected subnet; ignored addresses are filtered out
// and reported as a count.
func (s *Server) scanNetwork(c *gin.Context) {
	if s.disc == nil {
		writeProblem(c, http.StatusServiceUnavailable, "unavailable",
			"discovery is not configured", nil)
		return
	}
	var subnet, ok = querySubnet(c)
	if !ok {
		return
	}
	hosts, ok := s.runScan(c, subnet)
	if !ok {
		return
	}
	hosts, ignored := s.dropIgnored(hosts)
	for i := range hosts {
		_, hosts[i].Known = s.store.LookupMAC(hosts[i].MAC)
	}
	c.JSON(http.StatusOK, gin.H{"hosts": hosts, "ignored": ignored})
}

// querySubnet parses an optional ?cidr= override. It reports false when it
// already answered with a 422.
func querySubnet(c *gin.Context) (*net.IPNet, bool) {
	if raw := strings.TrimSpace(c.Query("cidr")); raw != "" {
		_, parsed, err := net.ParseCIDR(raw)
		if err != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("invalid cidr %q", raw), nil)
			return nil, false
		}
		return parsed, true
	}
	return nil, true
}

// runScan executes one sweep over subnet (nil for auto-detect) and maps
// scan-guard failures to 429. It reports false when it already answered.
func (s *Server) runScan(c *gin.Context, subnet *net.IPNet) ([]discover.Host, bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	var (
		hosts []discover.Host
		err   error
	)
	if subnet == nil {
		hosts, err = s.disc.Scan(ctx)
	} else {
		hosts, err = s.disc.ScanSubnet(ctx, subnet)
	}
	switch {
	case err == nil:
		return hosts, true
	case errors.Is(err, discover.ErrScanBusy), errors.Is(err, discover.ErrCoolingDown):
		if retry := s.disc.RetryIn(); retry > 0 {
			c.Header("Retry-After", fmt.Sprint(int(retry.Seconds())+1))
		}
		writeProblem(c, http.StatusTooManyRequests, "too many requests",
			"a scan is already running or cooling down", nil)
		return nil, false
	case errors.Is(err, discover.ErrSubnetLarge):
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"subnet is too large to scan", nil)
		return nil, false
	default:
		writeErr(c, err)
		return nil, false
	}
}

// dropIgnored removes denylisted hosts, keeping the dropped count so callers
// can report it honestly.
func (s *Server) dropIgnored(hosts []discover.Host) ([]discover.Host, int) {
	if s.ignore == nil {
		return hosts, 0
	}
	kept := hosts[:0]
	dropped := 0
	for _, h := range hosts {
		if s.ignore.Contains(h.MAC, h.IP) {
			dropped++
			continue
		}
		kept = append(kept, h)
	}
	return kept, dropped
}

// discoverStatus reports the scanner guards and the latest finished scan.
func (s *Server) discoverStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.disc.Status())
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

	if tags := parseTagQuery(c.Query("tags")); len(tags) > 0 {
		for _, d := range devices {
			d.Tags = append(append([]string(nil), d.Tags...), tags...)
			d.Normalize()
		}
	}
	if isDryRun(c) {
		c.JSON(http.StatusOK, gin.H{"devices": devices, "dry_run": true})
		return
	}
	created, err := s.store.CreateMany(devices)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"devices": created})
}

// parseTagQuery splits a comma-separated tag list, dropping blanks.
func parseTagQuery(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// buildAdoptDevices validates candidate hosts and maps them to registry
// devices, answering 422/409 itself and returning nil when it did.
func (s *Server) buildAdoptDevices(c *gin.Context, hosts []discover.Host) []*models.Device {
	devices := make([]*models.Device, 0, len(hosts))
	seenMAC := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		mac, err := net.ParseMAC(h.MAC)
		if err != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("invalid mac %q", h.MAC),
				gin.H{"errors": map[string][]string{"hosts": {"each host needs a valid mac"}}})
			return nil
		}
		canonicalMAC := mac.String()
		if _, duplicate := seenMAC[canonicalMAC]; duplicate {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("duplicate mac %q in adoption batch", canonicalMAC), nil)
			return nil
		}
		seenMAC[canonicalMAC] = struct{}{}
		if h.IP != "" && net.ParseIP(h.IP) == nil {
			writeProblem(c, http.StatusUnprocessableEntity, "validation failed",
				fmt.Sprintf("invalid ip %q", h.IP),
				gin.H{"errors": map[string][]string{"hosts": {"ip must be a valid address"}}})
			return nil
		}
		if _, found := s.store.LookupMAC(canonicalMAC); found {
			writeProblem(c, http.StatusConflict, "already exists",
				fmt.Sprintf("device with mac %s already exists", canonicalMAC),
				nil)
			return nil
		}
		name := h.Hostname
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
	return devices
}

// adoptAll scans (auto-detected subnet) then adopts every new candidate with
// a MAC. Already known, ignored, or MAC-less hosts are skipped and counted.
func (s *Server) adoptAll(c *gin.Context) {
	subnet, ok := querySubnet(c)
	if !ok {
		return
	}
	hosts, ok := s.runScan(c, subnet)
	if !ok {
		return
	}
	scanned := len(hosts)
	hosts, ignored := s.dropIgnored(hosts)
	fresh := make([]discover.Host, 0, len(hosts))
	skipped := 0
	for _, h := range hosts {
		if strings.TrimSpace(h.MAC) == "" {
			skipped++
			continue
		}
		if _, found := s.store.LookupMAC(h.MAC); found {
			skipped++
			continue
		}
		fresh = append(fresh, h)
	}
	devices := s.buildAdoptDevices(c, fresh)
	if devices == nil && len(fresh) > 0 {
		return
	}
	if len(devices) == 0 {
		c.JSON(http.StatusCreated, gin.H{"devices": []any{}, "scanned": scanned, "skipped": skipped, "ignored": ignored})
		return
	}
	created, err := s.store.CreateMany(devices)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"devices": created, "scanned": scanned, "skipped": skipped, "ignored": ignored})
}

// listIgnore returns every denylisted address.
func (s *Server) listIgnore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ignored": s.ignore.List()})
}

// addIgnore denylists one MAC and/or IP address.
func (s *Server) addIgnore(c *gin.Context) {
	var in struct {
		MAC string `json:"mac"`
		IP  string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		writeBindingError(c, err)
		return
	}
	created, err := s.ignore.Add(in.MAC, in.IP)
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", ignoreErrText(err), nil)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ignored": gin.H{"mac": in.MAC, "ip": in.IP}, "created": created})
}

// deleteIgnore lifts one address from the denylist via ?mac= and/or ?ip=.
func (s *Server) deleteIgnore(c *gin.Context) {
	removed, err := s.ignore.Remove(c.Query("mac"), c.Query("ip"))
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", ignoreErrText(err), nil)
		return
	}
	if !removed {
		writeProblem(c, http.StatusNotFound, "not found", "no such ignored address", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": true})
}

func ignoreErrText(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid mac"):
		return msg
	case strings.Contains(msg, "invalid ip"):
		return msg
	default:
		return "mac or ip is required"
	}
}
