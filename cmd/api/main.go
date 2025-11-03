package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cleeryy/hello/internal/config"
	"github.com/cleeryy/hello/internal/handlers"
	"github.com/cleeryy/hello/internal/monitor"
	"github.com/cleeryy/hello/internal/storage"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load the .env config: ", err)
	}

	store := storage.New("devices.json")

	mon := monitor.New(store, 10*time.Second)
	mon.Start()

	r := gin.Default()

	handlers.RegisterRoutes(r, cfg, store)

	go func() {
		if err := r.Run(); err != nil {
			log.Fatal(err)
		}
	}()

	select {}
}

