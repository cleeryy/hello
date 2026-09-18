package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/spec"
)

func (s *Server) openAPI(c *gin.Context) {
	c.Header("Content-Type", "application/yaml")
	c.Data(http.StatusOK, "application/yaml", spec.YAML)
}

func (s *Server) docs(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(spec.SwaggerHTML))
}
