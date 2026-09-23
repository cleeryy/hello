package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
)

// listDevices supports filtering, sorting and pagination:
// ?status=up&search=pc&tag=lab&sort=name&order=asc&page=1&per_page=50.
// The envelope always carries the total so clients can page honestly.
func (s *Server) listDevices(c *gin.Context) {
	devices := s.store.GetAll()

	if raw := c.Query("status"); raw != "" {
		if !models.Status(raw).IsValid() {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("invalid status filter %q", raw), nil)
			return
		}
		kept := devices[:0]
		for _, d := range devices {
			if string(d.Status) == raw {
				kept = append(kept, d)
			}
		}
		devices = kept
	}
	if raw := strings.ToLower(c.Query("search")); raw != "" {
		kept := devices[:0]
		for _, d := range devices {
			haystack := strings.ToLower(d.ID + " " + d.Name + " " + d.MAC + " " + d.IP)
			if strings.Contains(haystack, raw) {
				kept = append(kept, d)
			}
		}
		devices = kept
	}
	if raw := strings.ToLower(strings.TrimSpace(c.Query("tag"))); raw != "" {
		kept := devices[:0]
		for _, d := range devices {
			for _, tag := range d.Tags {
				if tag == raw {
					kept = append(kept, d)
					break
				}
			}
		}
		devices = kept
	}

	sortBy := c.DefaultQuery("sort", "name")
	order := c.DefaultQuery("order", "asc")
	if order != "asc" && order != "desc" {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			`order must be "asc" or "desc"`, nil)
		return
	}
	var less func(a, b *models.Device) bool
	switch sortBy {
	case "name":
		less = func(a, b *models.Device) bool {
			if strings.ToLower(a.Name) == strings.ToLower(b.Name) {
				return a.ID < b.ID
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	case "status":
		less = func(a, b *models.Device) bool {
			if a.Status == b.Status {
				return a.ID < b.ID
			}
			return a.Status < b.Status
		}
	case "last_seen":
		less = func(a, b *models.Device) bool {
			if a.LastSeen == b.LastSeen {
				return a.ID < b.ID
			}
			return a.LastSeen < b.LastSeen
		}
	case "wake_count":
		less = func(a, b *models.Device) bool {
			if a.WakeCount == b.WakeCount {
				return a.ID < b.ID
			}
			return a.WakeCount < b.WakeCount
		}
	default:
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			`sort must be one of "name", "status", "last_seen", "wake_count"`, nil)
		return
	}
	sort.SliceStable(devices, func(i, j int) bool {
		if order == "desc" {
			return less(devices[j], devices[i])
		}
		return less(devices[i], devices[j])
	})

	page, perPage, ok := parsePageParams(c)
	if !ok {
		return
	}
	total := len(devices)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, gin.H{
		"devices":  devices[start:end],
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// parsePageParams validates ?page= (from 1) and ?per_page= (1..500, default 100).
func parsePageParams(c *gin.Context) (int, int, bool) {
	fail := func(detail string) (int, int, bool) {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", detail, nil)
		return 0, 0, false
	}
	page := 1
	if raw := c.Query("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return fail("page must be an integer >= 1")
		}
		page = n
	}
	perPage := 100
	if raw := c.Query("per_page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			return fail("per_page must be an integer 1..500")
		}
		perPage = n
	}
	return page, perPage, true
}

func (s *Server) createDevice(c *gin.Context) {
	var device models.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		writeBindingError(c, err)
		return
	}
	if err := s.store.Create(&device); err != nil {
		writeErr(c, err)
		return
	}
	stored, err := s.store.Get(device.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, stored)
}

func (s *Server) getDevice(c *gin.Context) {
	device, err := s.store.Get(c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, device)
}

func (s *Server) updateDevice(c *gin.Context) {
	var device models.Device
	if err := c.ShouldBindJSON(&device); err != nil {
		writeBindingError(c, err)
		return
	}
	id := c.Param("id")
	if err := s.store.Update(id, &device); err != nil {
		writeErr(c, err)
		return
	}
	updated, err := s.store.Get(id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// patchDevice applies a JSON merge patch: only the keys present are changed.
// The id is immutable here; copy a device with POST /devices/:id/clone.
func (s *Server) patchDevice(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		writeBindingError(c, err)
		return
	}
	if len(patch) == 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"empty patch: send at least one field", nil)
		return
	}
	id := c.Param("id")
	stored, err := s.store.Get(id)
	if err != nil {
		writeErr(c, err)
		return
	}
	updated := *stored
	for key, value := range patch {
		var ferr error
		switch key {
		case "name":
			updated.Name, ferr = patchString(value)
		case "mac":
			updated.MAC, ferr = patchString(value)
		case "ip":
			updated.IP, ferr = patchString(value)
		case "notes":
			updated.Notes, ferr = patchString(value)
		case "status":
			var raw string
			raw, ferr = patchString(value)
			updated.Status = models.Status(raw)
		case "ping_enabled":
			updated.PingEnabled, ferr = patchBool(value)
		case "tags":
			updated.Tags, ferr = patchTags(value)
		case "id":
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				"id is immutable: use POST /devices/:id/clone to copy a device", nil)
			return
		default:
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("unknown field %q", key), nil)
			return
		}
		if ferr != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("field %q: %s", key, ferr.Error()), nil)
			return
		}
	}
	if err := s.store.Update(id, &updated); err != nil {
		writeErr(c, err)
		return
	}
	saved, err := s.store.Get(id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

func patchString(value any) (string, error) {
	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("must be a string")
	}
	return raw, nil
}

func patchBool(value any) (bool, error) {
	raw, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("must be a boolean")
	}
	return raw, nil
}

func patchTags(value any) ([]string, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array of strings")
	}
	tags := make([]string, 0, len(raw))
	for _, item := range raw {
		name, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("must be an array of strings")
		}
		tags = append(tags, name)
	}
	return tags, nil
}

func (s *Server) deleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.Delete(id); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
