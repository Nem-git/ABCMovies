package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	Core struct {
		API struct {
			Bind string `yaml:"bind"`
		} `yaml:"api"`
	} `yaml:"core"`
	Stores struct {
		Caches       string `yaml:"caches"`
		Vault        string `yaml:"vault"`
		WatchHistory string `yaml:"watch-history"`
		Jobs         string `yaml:"jobs"`
	} `yaml:"stores"`
}

func Default() *Config {
	c := &Config{}
	c.Core.API.Bind = "127.0.0.1:8443"
	c.Stores.Caches = "in-memory"
	c.Stores.Vault = "in-memory"
	c.Stores.WatchHistory = "in-memory"
	c.Stores.Jobs = "in-memory"
	return c
}

func Load(path string) (*Config, error) {
	c := Default()
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c, nil
}
