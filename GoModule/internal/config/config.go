package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config хранит всю конфигурацию бота.
// Теги env        — переменные окружения (GitLab CI проставляет автоматически).
// Теги yaml       — поля в config.yml для локального запуска.
// Теги env-default — значения по умолчанию.
// Теги env-required — обязательные поля, cleanenv вернёт ошибку если не заданы.
type Config struct {
	GitLab GitLabConfig `yaml:"gitlab"`
	LLM    LLMConfig    `yaml:"llm"`
	Runner RunnerConfig `yaml:"runner"`
}

type GitLabConfig struct {
	// CI_SERVER_URL — GitLab проставляет автоматически в CI-окружении.
	BaseURL string `yaml:"base_url" env:"CI_SERVER_URL" env-required:"true" env-description:"GitLab server URL"`

	// CI_JOB_TOKEN — GitLab проставляет автоматически. Используется только
	// для чтения: получение diff, информации о MR.
	JobToken string `yaml:"job_token" env:"CI_JOB_TOKEN" env-required:"true" env-description:"GitLab CI job token (read-only)"`

	// CR_BOT_TOKEN — Personal Access Token технического пользователя бота.
	// Требует права api. Используется для публикации комментариев и отчётов.
	// Создаётся вручную под bot-пользователем, хранится как protected CI variable.
	BotToken string `yaml:"bot_token" env:"CR_BOT_TOKEN" env-required:"true" env-description:"Bot PAT for posting comments (api scope)"`

	// Таймаут HTTP-запросов к GitLab API.
	TimeoutSec int `yaml:"timeout_sec" env:"GITLAB_TIMEOUT_SEC" env-default:"30" env-description:"GitLab API request timeout in seconds"`
}

type LLMConfig struct {
	// Адрес Python LLM-модуля / mock-сервера.
	ServiceURL string `yaml:"service_url" env:"LLM_SERVICE_URL" env-default:"http://mock-llm:8000" env-description:"LLM service URL"`

	// Название модели, передаётся в Python-модулю.
	Model string `yaml:"model" env:"LLM_MODEL" env-default:"deepseek-coder" env-description:"LLM model name"`

	// Таймаут одного LLM-запроса.
	TimeoutSec int `yaml:"timeout_sec" env:"LLM_TIMEOUT_SEC" env-default:"60" env-description:"LLM request timeout in seconds"`
}

type RunnerConfig struct {
	// Максимальное число параллельных LLM-запросов.
	MaxWorkers int `yaml:"max_workers" env:"MAX_WORKERS" env-default:"5" env-description:"Max parallel LLM workers"`

	// Если true — exit code 1 при наличии Critical-замечаний.
	BlockOnCritical bool `yaml:"block_on_critical" env:"BLOCK_ON_CRITICAL" env-default:"true" env-description:"Exit with code 1 if critical issues found"`
}

// Load читает конфигурацию из переменных окружения.
// Используется внутри GitLab CI — все значения приходят через env.
func Load() (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("config: read environment: %w", err)
	}

	return &cfg, nil
}

// LoadFromFile читает конфигурацию из yaml-файла,
// переменные окружения переопределяют значения из файла.
// Используется для локального запуска вне GitLab CI.
func LoadFromFile(path string) (*Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}

	return &cfg, nil
}
