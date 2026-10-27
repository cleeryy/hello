package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/history"
	"github.com/cleeryy/hello/internal/models"
)

// metrics exposes Prometheus text exposition without a client dependency:
// wake counters by trigger and result, device and schedule gauges, and the
// process uptime. The route is guarded like the rest of the API.
func (s *Server) metrics(c *gin.Context) {
	var st history.Stats
	if s.hist != nil {
		st = s.hist.Stats()
	}

	var up, down, unknown int
	for _, d := range s.store.GetAll() {
		if d == nil {
			continue
		}
		switch d.Status {
		case models.StatusUp:
			up++
		case models.StatusDown:
			down++
		default:
			unknown++
		}
	}

	enabled, disabled := 0, 0
	if s.schedStore != nil {
		for _, sch := range s.schedStore.All() {
			if sch.Enabled {
				enabled++
			} else {
				disabled++
			}
		}
	}

	uptime := int64(0)
	if !s.started.IsZero() {
		if secs := int64(time.Since(s.started).Seconds()); secs > 0 {
			uptime = secs
		}
	}

	var b strings.Builder
	b.WriteString("# HELP hello_wake_total Magic packets sent, by trigger and result.\n")
	b.WriteString("# TYPE hello_wake_total counter\n")
	fmt.Fprintf(&b, "hello_wake_total{trigger=\"manual\",result=\"ok\"} %d\n", st.ManualOK)
	fmt.Fprintf(&b, "hello_wake_total{trigger=\"manual\",result=\"error\"} %d\n", st.ManualErr)
	fmt.Fprintf(&b, "hello_wake_total{trigger=\"schedule\",result=\"ok\"} %d\n", st.ScheduleOK)
	fmt.Fprintf(&b, "hello_wake_total{trigger=\"schedule\",result=\"error\"} %d\n", st.ScheduleErr)
	b.WriteString("# HELP hello_devices Registered devices by reachability status.\n")
	b.WriteString("# TYPE hello_devices gauge\n")
	fmt.Fprintf(&b, "hello_devices{status=\"up\"} %d\n", up)
	fmt.Fprintf(&b, "hello_devices{status=\"down\"} %d\n", down)
	fmt.Fprintf(&b, "hello_devices{status=\"unknown\"} %d\n", unknown)
	b.WriteString("# HELP hello_schedules Wake schedules by enabled flag.\n")
	b.WriteString("# TYPE hello_schedules gauge\n")
	fmt.Fprintf(&b, "hello_schedules{enabled=\"true\"} %d\n", enabled)
	fmt.Fprintf(&b, "hello_schedules{enabled=\"false\"} %d\n", disabled)
	b.WriteString("# HELP hello_uptime_seconds Seconds since process start.\n")
	b.WriteString("# TYPE hello_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "hello_uptime_seconds %d\n", uptime)
	b.WriteString("# HELP hello_history_size Wake events currently retained.\n")
	b.WriteString("# TYPE hello_history_size gauge\n")
	fmt.Fprintf(&b, "hello_history_size %d\n", st.Size)

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, b.String())
}
