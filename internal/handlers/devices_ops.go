package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/wol"
)

// bulkDeleteDevices removes several devices in one call:
// DELETE /devices?ids=pc1,pc2. Unknown ids are reported, not fatal.
func (s *Server) bulkDeleteDevices(c *gin.Context) {
	raw := c.Query("ids")
	if strings.TrimSpace(raw) == "" {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"query param ids is required, e.g. ?ids=pc1,pc2", nil)
		return
	}
	deleted := []string{}
	notFound := []string{}
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := s.store.Delete(id); err != nil {
			notFound = append(notFound, id)
			continue
		}
		deleted = append(deleted, id)
	}
	if len(deleted) == 0 && len(notFound) == 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"query param ids carries no usable id", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "not_found": notFound})
}

// cloneDevice copies a device under a new id: POST /devices/:id/clone?new_id=pc9.
// Counters and reachability reset; identity and network settings are kept.
func (s *Server) cloneDevice(c *gin.Context) {
	src, err := s.store.Get(c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	newID := strings.TrimSpace(c.Query("new_id"))
	if newID == "" {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"query param new_id is required", nil)
		return
	}
	copy := &models.Device{
		ID:          newID,
		Name:        src.Name + " (copy)",
		MAC:         src.MAC,
		Status:      models.StatusUnknown,
		IP:          src.IP,
		PingEnabled: src.PingEnabled,
		Notes:       src.Notes,
		Tags:        append([]string{}, src.Tags...),
	}
	if err := s.store.Create(copy); err != nil {
		writeErr(c, err)
		return
	}
	stored, err := s.store.Get(newID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, stored)
}

// wakeBatchRequest selects wake targets by explicit ids, by tag, or both.
// An empty selection means the whole registry.
type wakeBatchRequest struct {
	IDs    []string `json:"ids"`
	Tag    string   `json:"tag"`
	DryRun bool     `json:"dry_run"`
}

// wakeBatch wakes several devices at once, reports per-device failures, and
// supports dry_run to preview the target set without sending anything.
func (s *Server) wakeBatch(c *gin.Context) {
	var in wakeBatchRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&in); err != nil {
			writeBindingError(c, err)
			return
		}
	}
	targets, problem := s.selectWakeTargets(in)
	if problem != "" {
		writeProblem(c, http.StatusNotFound, "not found", problem, nil)
		return
	}
	matched := make([]string, 0, len(targets))
	for _, d := range targets {
		matched = append(matched, d.ID)
	}
	if in.DryRun {
		c.JSON(http.StatusOK, gin.H{
			"dry_run": true,
			"matched": matched,
			"woken":   []string{},
			"failed":  []gin.H{},
		})
		return
	}
	woken := []string{}
	failed := []gin.H{}
	for _, d := range targets {
		if err := s.sendWOL(d.MAC, wol.BroadcastForIP(d.IP, s.cfg.BroadcastIP)); err != nil {
			slog.Error("wol batch send failed", slog.String("device", d.ID), slog.Any("err", err))
			s.recordWake(d.MAC, false, err.Error(), 1, "")
			failed = append(failed, gin.H{"id": d.ID, "error": err.Error()})
			continue
		}
		s.recordWake(d.MAC, true, "", 1, "")
		woken = append(woken, d.ID)
	}
	c.JSON(http.StatusOK, gin.H{
		"dry_run": false,
		"matched": matched,
		"woken":   woken,
		"failed":  failed,
	})
}

// selectWakeTargets resolves the union of ids and tag matches.
func (s *Server) selectWakeTargets(in wakeBatchRequest) ([]*models.Device, string) {
	seen := map[string]bool{}
	var targets []*models.Device
	for _, id := range in.IDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		d, err := s.store.Get(id)
		if err != nil {
			return nil, fmt.Sprintf("unknown device id %q", id)
		}
		seen[id] = true
		targets = append(targets, d)
	}
	if tag := strings.ToLower(strings.TrimSpace(in.Tag)); tag != "" {
		for _, d := range s.store.GetAll() {
			if seen[d.ID] {
				continue
			}
			for _, t := range d.Tags {
				if t == tag {
					seen[d.ID] = true
					targets = append(targets, d)
					break
				}
			}
		}
	}
	if len(in.IDs) == 0 && strings.TrimSpace(in.Tag) == "" {
		targets = s.store.GetAll()
	}
	return targets, ""
}

// exportDevices downloads the whole registry as a JSON array.
func (s *Server) exportDevices(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="devices-export.json"`)
	c.JSON(http.StatusOK, s.store.GetAll())
}

type importDevicesRequest struct {
	Devices []*models.Device `json:"devices"`
	Mode    string           `json:"mode"`
}

// importDevices bulk-loads devices. Every entry is validated before anything
// is written, so a bad batch never leaves a partial registry. Modes:
// abort (default) refuses when any id exists, skip keeps existing ones,
// replace overwrites existing ids (deletes then creates).
func (s *Server) importDevices(c *gin.Context) {
	var in importDevicesRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		writeBindingError(c, err)
		return
	}
	if len(in.Devices) == 0 {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"devices must carry at least one entry", nil)
		return
	}
	mode := in.Mode
	if mode == "" {
		mode = "abort"
	}
	if mode != "abort" && mode != "skip" && mode != "replace" {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			`mode must be one of "abort", "skip", "replace"`, nil)
		return
	}
	for i, d := range in.Devices {
		if d == nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("devices[%d]: null entry", i), nil)
			return
		}
		clone := *d
		clone.Normalize()
		if err := clone.Validate(); err != nil {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("devices[%d] (%s): %s", i, d.ID, err.Error()), nil)
			return
		}
	}
	seen := map[string]int{}
	for i, d := range in.Devices {
		if first, dup := seen[d.ID]; dup {
			writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
				fmt.Sprintf("devices[%d]: duplicate id %q (first at index %d)", i, d.ID, first), nil)
			return
		}
		seen[d.ID] = i
	}
	existing := []string{}
	for _, d := range in.Devices {
		if _, err := s.store.Get(d.ID); err == nil {
			existing = append(existing, d.ID)
		}
	}
	switch mode {
	case "abort":
		if len(existing) > 0 {
			writeProblem(c, http.StatusConflict, "conflict",
				fmt.Sprintf("already registered: %s", strings.Join(existing, ", ")), nil)
			return
		}
		s.importCreate(c, in.Devices, nil)
	case "skip":
		skip := map[string]bool{}
		for _, id := range existing {
			skip[id] = true
		}
		fresh := make([]*models.Device, 0, len(in.Devices))
		for _, d := range in.Devices {
			if !skip[d.ID] {
				fresh = append(fresh, d)
			}
		}
		s.importCreate(c, fresh, existing)
	default: // replace
		for _, id := range existing {
			if err := s.store.Delete(id); err != nil {
				writeErr(c, err)
				return
			}
		}
		s.importCreate(c, in.Devices, nil)
	}
}

func (s *Server) importCreate(c *gin.Context, devices []*models.Device, skipped []string) {
	created, err := s.store.CreateMany(devices)
	if err != nil {
		writeErr(c, err)
		return
	}
	if skipped == nil {
		skipped = []string{}
	}
	c.JSON(http.StatusCreated, gin.H{"created": created, "skipped": skipped})
}

// deviceCounts returns registry totals by reachability for cheap polling.
func (s *Server) deviceCounts(c *gin.Context) {
	var up, down, unknown int
	for _, d := range s.store.GetAll() {
		switch d.Status {
		case models.StatusUp:
			up++
		case models.StatusDown:
			down++
		default:
			unknown++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total":   up + down + unknown,
		"up":      up,
		"down":    down,
		"unknown": unknown,
	})
}
