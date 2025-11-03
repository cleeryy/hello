package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/handlers"
	"github.com/cleeryy/hello/internal/storage"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load the .env config: ", err)
		return
	}

	store := storage.New("devices.json")

	r := gin.Default()
	
	handlers.RegisterRoutes(r, cfg, store)
	
	r.Run()
}
