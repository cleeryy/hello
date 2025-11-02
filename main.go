package main

import (
	"net/http"
	"log"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {

	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Unable to load the .env config: ", err)
		return
	}

	r := gin.Default()
	
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": http.StatusOK,
			"message": "welcome to the hello api!",
			"defaultMac": config.DefaultMAC,
		})
	})

	r.GET("/wake", func(c *gin.Context) {
		err := SendWOLPacket(config.DefaultMAC)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send wol packet: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("magic packet sent to %v !", config.DefaultMAC),
			"status": http.StatusOK,
		})
	})

	r.GET("/wake/:macAddress", func(c *gin.Context) {
		macAddr := c.Param("macAddress")
		err := SendWOLPacket(macAddr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to send wol packet to %s device: %v", macAddr, err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("magic packet sent to %v !", macAddr),
			"status": http.StatusOK,
		})
	})
	
	r.Run()
}
