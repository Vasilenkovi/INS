package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"cr-assistant/internal/config"
	"cr-assistant/internal/domain"
	"cr-assistant/internal/gitlab"
	"cr-assistant/internal/llm"
	"cr-assistant/internal/report"
	"cr-assistant/internal/review"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("review failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// Загружаем конфигурацию
	//    CONFIG_PATH задан — читаем из файла (локальный запуск).
	//    Иначе — только из переменных окружения (режим GitLab CI).
	var (
		cfg *config.Config
		err error
	)
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		cfg, err = config.LoadFromFile(cfgPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Читаем параметры MR из переменных окружения GitLab CI. GitLab проставляет их автоматически при запуске pipeline на MR.
	projectID, err := requireEnvInt("CI_PROJECT_ID")
	if err != nil {
		return err
	}
	mrIID, err := requireEnvInt("CI_MERGE_REQUEST_IID")
	if err != nil {
		return err
	}

	logger.Info("starting cr-assistant",
		"project_id", projectID,
		"mr_iid", mrIID,
		"llm_model", cfg.LLM.Model,
		"gitlab_url", cfg.GitLab.BaseURL,
	)

	// Собираем зависимости (dependency injection вручную)
	gitlabClient := gitlab.NewClient(
		cfg.GitLab.BaseURL,
		cfg.GitLab.JobToken,
		cfg.GitLab.BotToken,
		time.Duration(cfg.GitLab.TimeoutSec)*time.Second,
		logger,
	)
	llmGateway := llm.NewGateway(cfg.LLM.ServiceURL, time.Duration(cfg.LLM.TimeoutSec)*time.Second)
	reportRenderer := report.NewRenderer()

	orchestrator := review.NewOrchestrator(
		gitlabClient,
		llmGateway,
		reportRenderer,
		logger,
	)

	// Запускаем анализ
	ctx := context.Background()
	result, err := orchestrator.Run(ctx, projectID, mrIID)
	if err != nil {
		return fmt.Errorf("orchestrator: %w", err)
	}

	// Определяем exit code
	if cfg.Runner.BlockOnCritical && result.HasCritical {
		logger.Warn("critical issues found, blocking merge",
			"critical_count", result.TotalByLevel[domain.SeverityCritical],
		)
		fmt.Fprintln(os.Stderr, "Обнаружены критические замечания. Merge заблокирован.")
		os.Exit(1)
	}

	fmt.Println("Код-ревью завершено. Критических замечаний не найдено.")
	return nil
}

// requireEnvInt читает обязательную переменную окружения как int.
func requireEnvInt(key string) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, fmt.Errorf("required environment variable %q is not set", key)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("environment variable %q must be an integer, got %q", key, v)
	}
	return n, nil
}
