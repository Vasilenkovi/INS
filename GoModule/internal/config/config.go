package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	GitLab          GitLabConfig          `yaml:"gitlab"`
	LLM             LLMConfig             `yaml:"llm"`
	Runner          RunnerConfig          `yaml:"runner"`
	StandardsService StandardsServiceConfig `yaml:"standards_service"`
}

type GitLabConfig struct {
	BaseURL    string `yaml:"base_url"    env:"CI_SERVER_URL"      env-required:"true"  env-description:"GitLab server URL"`
	JobToken   string `yaml:"job_token"   env:"CI_JOB_TOKEN"       env-required:"true"  env-description:"GitLab CI job token"`
	BotToken   string `yaml:"bot_token"   env:"CR_BOT_TOKEN"       env-required:"true"  env-description:"Bot PAT for posting comments (api scope)"`
	TimeoutSec int    `yaml:"timeout_sec" env:"GITLAB_TIMEOUT_SEC" env-default:"30"     env-description:"GitLab API request timeout in seconds"`
}

type LLMConfig struct {
	ServiceURL string `yaml:"service_url" env:"LLM_SERVICE_URL" env-required:"true"              env-description:"Python LLM module URL"`
	TimeoutSec int    `yaml:"timeout_sec" env:"LLM_TIMEOUT_SEC" env-default:"120"                env-description:"LLM request timeout in seconds"`
}

type RunnerConfig struct {
	BlockOnCritical bool `yaml:"block_on_critical" env:"BLOCK_ON_CRITICAL" env-default:"true" env-description:"Exit with code 1 if critical issues found"`
}

// StandardsServiceConfig — настройки подключения к standards-service.
// cr-assistant получает стандарт кода через HTTP, не напрямую из БД.
type StandardsServiceConfig struct {
	URL        string `yaml:"url"         env:"STANDARDS_SERVICE_URL" env-required:"true" env-description:"standards-service base URL"`
	TimeoutSec int    `yaml:"timeout_sec" env:"STANDARDS_TIMEOUT_SEC" env-default:"15"    env-description:"standards-service request timeout in seconds"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("config: read environment: %w", err)
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
