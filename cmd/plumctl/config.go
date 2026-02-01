package main

import (
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

var (
	config *Config
)

type Config struct {
	Server       string       `yaml:"server"`
	Certificates Certificates `yaml:"certificates"`
}

type Certificates struct {
	CaCertPath string `yaml:"ca"`
	CertPath   string `yaml:"cert"`
	KeyPath    string `yaml:"key"`
}

func configPath() (string, error) {
	basePath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(basePath, "plumctl", "config.yml"), nil
}

func LoadConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewDecoder(f).Decode(&config)
}

func SaveConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0700)); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := yaml.NewEncoder(f).Encode(config); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}
