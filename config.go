package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DefaultMAC string
}

func LoadConfig() (*Config, error) {
	
	godotenv.Load()

	config := &Config{
		DefaultMAC : os.Getenv("DEFAULT_MAC"),
	} 

	if config.DefaultMAC == "" {
		log.Fatal("not default mac address found.")
	}

	return config, nil
}
