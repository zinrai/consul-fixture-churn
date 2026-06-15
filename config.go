package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// ChurnConfig is the whole of churn.json: a reference to the inventory file
// (resolved relative to churn.json) and churn's own run parameters. churn.json
// is the only source of these parameters; there are no command-line overrides.
type ChurnConfig struct {
	Inventory string   `json:"inventory"`
	Seed      *int64   `json:"seed"`     // required; pointer distinguishes absent
	Interval  *float64 `json:"interval"` // seconds between operations (default 300)
	Count     *int     `json:"count"`    // stop after N operations (default 0 = until interrupted)
}

func loadConfig(path string) (*ChurnConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ChurnConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Inventory == "" {
		return nil, fmt.Errorf(`%s: "inventory" must be a path to an inventory file`, path)
	}
	return &cfg, nil
}
