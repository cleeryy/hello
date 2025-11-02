package main

import (
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	DefaultMAC string
}

func LoadConfig() (*Config, error) {
	
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	config := &Config{
		DefaultMAC : os.Getenv("DEFAULT_MAC"),
	} 

	return config, nil
}
