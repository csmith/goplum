package goplum

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Defaults CheckSettings     `json:"defaults"`
	Alerts   []ConfiguredAlert `json:"alerts"`
	Checks   []ConfiguredCheck `json:"checks"`
}

type CheckSettings struct {
	Alerts           []string      `json:"alerts"`
	Interval         time.Duration `json:"interval"`
	GoodThreshold    int           `json:"good_threshold"`
	FailingThreshold int           `json:"failing_threshold"`
}

type ConfiguredAlert struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

type ConfiguredCheck struct {
	CheckSettings
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

var DefaultSettings = CheckSettings{
	Alerts:           []string{"*"},
	Interval:         time.Second * 30,
	GoodThreshold:    2,
	FailingThreshold: 2,
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config := &Config{}
	if err := json.NewDecoder(f).Decode(config); err != nil {
		return nil, err
	}

	// Populate all check fields with default values, either from the "Defaults" section of the config or our
	// hardcoded defaults.
	for i := range config.Checks {
		check := &config.Checks[i].CheckSettings
		check.fillCheckSettings(config.Defaults)
		check.fillCheckSettings(DefaultSettings)
	}

	return config, nil
}

func (c *CheckSettings) fillCheckSettings(from CheckSettings) {
	if len(c.Alerts) == 0 {
		c.Alerts = from.Alerts
	}

	if c.Interval == 0 {
		c.Interval = from.Interval
	}

	if c.FailingThreshold == 0 {
		c.FailingThreshold = from.FailingThreshold
	}

	if c.GoodThreshold == 0 {
		c.GoodThreshold = from.GoodThreshold
	}
}
