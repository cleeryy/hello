
package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/storage"
	"github.com/cleeryy/hello/internal/wol"
)

func RegisterRoutes(r *gin.Engine, cfg *config.Config, store *storage.Storage) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      http.StatusOK,
			"message":     "welcome to the hello api!",
			"defaultMac":  cfg.DefaultMAC,
		})
	})

	r.GET("/wake", func(c *gin.Context) {
		err := wol.SendWOLPacket(cfg.DefaultMAC)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send wol packet: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("magic packet sent to %v !", cfg.DefaultMAC),
			"status":  http.StatusOK,
		})
	})

	r.GET("/wake/:macAddress", func(c *gin.Context) {
		macAddr := c.Param("macAddress")
		err := wol.SendWOLPacket(macAddr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send wol packet to %s device: %v", macAddr, err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("magic packet sent to %v !", macAddr),
			"status":  http.StatusOK,
		})
	})

	r.GET("/devices", func(c *gin.Context) {
		devices := store.GetAll()
		c.JSON(http.StatusOK, gin.H{
			"devices": devices,
		})
	})

	r.POST("/devices", func(c *gin.Context) {
		var device models.Device
		if err := c.BindJSON(&device); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid request: %v", err),
			})
			return
		}

		if err := store.Create(&device); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, device)
	})

	r.GET("/devices/:id", func(c *gin.Context) {
		id := c.Param("id")
		device, err := store.Get(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, device)
	})

	r.PUT("/devices/:id", func(c *gin.Context) {
		id := c.Param("id")
		var device models.Device
		if err := c.BindJSON(&device); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid request: %v", err),
			})
			return
		}

		if err := store.Update(id, &device); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, device)
	})

	r.DELETE("/devices/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := store.Delete(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("device %s deleted", id),
		})
	})
}

