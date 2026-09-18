package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/models"
)

func (s *Server) listDevices(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"devices": s.store.GetAll()})
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

func (s *Server) deleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.Delete(id); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
