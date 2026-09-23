package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
)

// listHistory returns recorded wakes newest-first, JSON envelope included.
// Filters combine: device_id, trigger (manual|schedule), success, and an
// RFC3339 since/before window.
func (s *Server) listHistory(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeProblem(c, http.StatusBadRequest, "bad request",
				"limit must be a positive integer", nil)
			return
		}
		limit = n
	}
	filter, ok := parseHistoryFilter(c)
	if !ok {
		return
	}
	if s.hist == nil {
		c.JSON(http.StatusOK, gin.H{"history": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": s.hist.ListFiltered(filter, limit)})
}

// parseHistoryFilter reads the trigger/success/since/before query dimensions.
func parseHistoryFilter(c *gin.Context) (history.Filter, bool) {
	fail := func(detail string) (history.Filter, bool) {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity", detail, nil)
		return history.Filter{}, false
	}
	var f history.Filter
	f.DeviceID = c.Query("device_id")
	if raw := c.Query("trigger"); raw != "" {
		t := models.Trigger(raw)
		if !t.IsValid() {
			return fail(fmt.Sprintf("trigger must be %q or %q", models.TriggerManual, models.TriggerSchedule))
		}
		f.Trigger = t
	}
	if raw := c.Query("success"); raw != "" {
		switch raw {
		case "true", "1":
			ok := true
			f.Success = &ok
		case "false", "0":
			ok := false
			f.Success = &ok
		default:
			return fail("success must be true or false")
		}
	}
	for _, dim := range []struct {
		query string
		set   func(int64)
	}{
		{"since", func(v int64) { f.Since = v }},
		{"before", func(v int64) { f.Before = v }},
	} {
		if raw := c.Query(dim.query); raw != "" {
			ts, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return fail(fmt.Sprintf("%s must be an RFC3339 timestamp", dim.query))
			}
			dim.set(ts.Unix())
		}
	}
	return f, true
}

// historyStats aggregates the retained log: totals, result split,
// per-trigger split, and the success rate over retained entries.
func (s *Server) historyStats(c *gin.Context) {
	byTrigger := map[string]gin.H{
		string(models.TriggerManual):   {"ok": 0, "error": 0},
		string(models.TriggerSchedule): {"ok": 0, "error": 0},
	}
	ok, failed := 0, 0
	if s.hist != nil {
		for _, e := range s.hist.ListFiltered(history.Filter{}, math.MaxInt32) {
			if e.Success {
				ok++
				byTrigger[string(e.Trigger)]["ok"] = byTrigger[string(e.Trigger)]["ok"].(int) + 1
			} else {
				failed++
				byTrigger[string(e.Trigger)]["error"] = byTrigger[string(e.Trigger)]["error"].(int) + 1
			}
		}
	}
	total := ok + failed
	rate := 0.0
	if total > 0 {
		rate = math.Round(float64(ok)/float64(total)*10000) / 10000
	}
	c.JSON(http.StatusOK, gin.H{
		"total":        total,
		"ok":           ok,
		"error":        failed,
		"success_rate": rate,
		"by_trigger":   byTrigger,
	})
}

// purgeHistory drops entries recorded strictly before ?before= (RFC3339,
// required) and reports the removed count.
func (s *Server) purgeHistory(c *gin.Context) {
	raw := c.Query("before")
	if raw == "" {
		writeProblem(c, http.StatusBadRequest, "bad request",
			"before is required (RFC3339 timestamp)", nil)
		return
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeProblem(c, http.StatusUnprocessableEntity, "unprocessable entity",
			"before must be an RFC3339 timestamp", nil)
		return
	}
	purged := 0
	if s.hist != nil {
		if purged, err = s.hist.Purge(ts.Unix()); err != nil {
			writeErr(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"purged": purged})
}

// exportHistory downloads the retained log as CSV, newest first.
func (s *Server) exportHistory(c *gin.Context) {
	var rows []models.WakeEvent
	if s.hist != nil {
		rows = s.hist.ListFiltered(history.Filter{}, math.MaxInt32)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "at", "device_id", "mac", "trigger", "success", "attempts", "note", "error"})
	for _, e := range rows {
		_ = w.Write([]string{
			e.ID,
			time.Unix(e.At, 0).UTC().Format(time.RFC3339),
			e.DeviceID,
			e.MAC,
			string(e.Trigger),
			strconv.FormatBool(e.Success),
			strconv.Itoa(e.Attempts),
			e.Note,
			e.Error,
		})
	}
	w.Flush()
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", `attachment; filename="history.csv"`)
	c.String(http.StatusOK, buf.String())
}
