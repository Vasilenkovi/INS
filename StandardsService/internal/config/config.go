package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	GitLab   GitLabConfig   `yaml:"gitlab"`
}

type ServerConfig struct {
	Port string `yaml:"port" env:"PORT" env-default:"9090" env-description:"HTTP server port"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn" env:"DATABASE_URL" env-required:"true" env-description:"PostgreSQL DSN"`
}

type GitLabConfig struct {
	BaseURL    string `yaml:"base_url"    env:"GITLAB_URL"         env-required:"true" env-description:"GitLab instance URL"`
	TimeoutSec int    `yaml:"timeout_sec" env:"GITLAB_TIMEOUT_SEC" env-default:"10"    env-description:"GitLab API timeout"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}

func LoadFromFile(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}
	return &cfg, nil
}
