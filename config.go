package goplum

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Alerts []ConfiguredAlert `json:"alerts"`
	Checks []ConfiguredCheck `json:"checks"`
}

type ConfiguredAlert struct {
	Notifier string          `json:"notifier"`
	Params   json.RawMessage `json:"params"`
}

type ConfiguredCheck struct {
	Name     string          `json:"name"`
	Check    string          `json:"check"`
	Interval time.Duration   `json:"interval"`
	Params   json.RawMessage `json:"params"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config := &Config{}
	return config, json.NewDecoder(f).Decode(config)
}
