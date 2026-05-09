package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config хранит всю конфигурацию бота.
// Значения берутся из переменных окружения, которые GitLab CI
// передаёт в контейнер при запуске job.
type Config struct {
	// GitLab
	// CI_SERVER_URL и CI_JOB_TOKEN проставляются GitLab автоматически.
	GitLabBaseURL string // CI_SERVER_URL, например https://gitlab.example.com
	GitLabToken   string // CI_JOB_TOKEN

	// LLM Python-модуль
	// Адрес сервиса, который будет вызывать self-hosted LLM.
	// Оставлен опциональным — модуль пока не реализован.
	LLMServiceURL string // LLM_SERVICE_URL
	LLMModel      string // LLM_MODEL, default: deepseek-coder

	// Параллелизм
	MaxWorkers int // MAX_WORKERS, default: 5

	// Таймауты (секунды)
	LLMTimeoutSec    int // LLM_TIMEOUT_SEC, default: 60
	GitLabTimeoutSec int // GITLAB_TIMEOUT_SEC, default: 30

	// Блокировка pipeline
	// Если true — exit code 1 при наличии Critical-замечаний.
	BlockOnCritical bool // BLOCK_ON_CRITICAL, default: true
}

// Load читает конфигурацию из переменных окружения.
// Возвращает ошибку, если обязательное поле отсутствует.
func Load() (*Config, error) {
	cfg := &Config{}

	var err error

	// Обязательные поля
	if cfg.GitLabBaseURL, err = requireEnv("CI_SERVER_URL"); err != nil {
		return nil, err
	}
	if cfg.GitLabToken, err = requireEnv("CI_JOB_TOKEN"); err != nil {
		return nil, err
	}

	// Опциональные поля
	cfg.LLMServiceURL = optionalEnv("LLM_SERVICE_URL", "")
	cfg.LLMModel = optionalEnv("LLM_MODEL", "deepseek-coder")

	cfg.MaxWorkers, err = parseInt("MAX_WORKERS", 5)
	if err != nil {
		return nil, err
	}

	cfg.LLMTimeoutSec, err = parseInt("LLM_TIMEOUT_SEC", 60)
	if err != nil {
		return nil, err
	}

	cfg.GitLabTimeoutSec, err = parseInt("GITLAB_TIMEOUT_SEC", 30)
	if err != nil {
		return nil, err
	}

	cfg.BlockOnCritical, err = parseBool("BLOCK_ON_CRITICAL", true)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// helpers

func requireEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

func optionalEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q must be an integer, got %q", key, v)
	}
	return n, nil
}

func parseBool(key string, defaultVal bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("environment variable %q must be a boolean (true/false/1/0), got %q", key, v)
	}
	return b, nil
}
