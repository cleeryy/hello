package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cleeryy/hello/internal/models"
	"github.com/cleeryy/hello/internal/storage"
)

// seedDevices bulk-loads a JSON array of devices into the registry file named
// by DEVICES_FILE (default ./devices.json). Existing ids abort the import so
// a seed never silently overwrites a hand-curated registry.
func seedDevices(devicesFile, srcPath string) (int, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return 0, fmt.Errorf("seed: read %s: %w", srcPath, err)
	}
	var devices []*models.Device
	if err := json.Unmarshal(raw, &devices); err != nil {
		return 0, fmt.Errorf("seed: decode %s: %w", srcPath, err)
	}
	if len(devices) == 0 {
		return 0, fmt.Errorf("seed: %s carries no devices", srcPath)
	}
	store, err := storage.New(devicesFile)
	if err != nil {
		return 0, err
	}
	created, err := store.CreateMany(devices)
	if err != nil {
		return 0, fmt.Errorf("seed: %w", err)
	}
	return len(created), nil
}

func seedFile() string {
	if v := os.Getenv("DEVICES_FILE"); v != "" {
		return v
	}
	return "devices.json"
}
