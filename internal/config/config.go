package config

import (
	"fmt"
	"os"

	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/proxy"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Services []ServiceEntry `yaml:"services"`
	DRM      drm.Config     `yaml:"drm"`
}

type ServerConfig struct {
	Port      int    `yaml:"port"`
	BaseURL   string `yaml:"base_url"`
	APIPrefix string `yaml:"api_prefix"`
}

type StubSearchEntry struct {
	ResourceType string `yaml:"resourceType"`
	ResourceID   string `yaml:"resourceId"`
}

type ServiceEntry struct {
	Tag         string   `yaml:"tag"`
	Type        string   `yaml:"type"`
	Name        string   `yaml:"name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	URL         string   `yaml:"url,omitempty"`
	Country     string   `yaml:"country,omitempty"`
	Languages   []string `yaml:"languages,omitempty"`

	Movies    []oas.Movie       `yaml:"movies,omitempty"`
	Series    []oas.Series      `yaml:"series,omitempty"`
	Seasons   []oas.Season      `yaml:"seasons,omitempty"`
	Episodes  []oas.Episode     `yaml:"episodes,omitempty"`
	Streams   []oas.Stream      `yaml:"streams,omitempty"`
	Subtitles []oas.Subtitle    `yaml:"subtitles,omitempty"`
	Search    []StubSearchEntry `yaml:"search,omitempty"`

	Proxy *proxy.Config `yaml:"proxy,omitempty"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	cfg.Server.Port = 80
	cfg.Server.APIPrefix = "/api/v1alpha"

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decoding config: %w", err)
	}

	if v := os.Getenv("BASE_URL"); v != "" {
		cfg.Server.BaseURL = v
	}
	if v := os.Getenv("API_PREFIX"); v != "" {
		cfg.Server.APIPrefix = v
	}

	return &cfg, nil
}
