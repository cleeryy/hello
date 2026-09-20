package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// listHistory returns recorded wakes newest-first, JSON envelope included.
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
	if s.hist == nil {
		c.JSON(http.StatusOK, gin.H{"history": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": s.hist.List(c.Query("device_id"), limit)})
}
