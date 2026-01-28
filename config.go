package main

import (
    "fmt"
    "log"
    "os"

    "github.com/joho/godotenv"
)

func LoadConfig() (*Config, error) {
    if err := godotenv.Load(); err != nil {
        log.Printf("warning: could not load .env file: %v", err)
    }

    config := &Config{
        DefaultMAC: os.Getenv("DEFAULT_MAC"),
    }

    if config.DefaultMAC == "" {
        return nil, fmt.Errorf("default MAC address not found (DEFAULT_MAC)")
    }

    return config, nil
}
