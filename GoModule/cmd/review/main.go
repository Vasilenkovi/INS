package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"cr-assistant/internal/codereview"
	"cr-assistant/internal/config"
	"cr-assistant/internal/domain"
	"cr-assistant/internal/gitlab"
	"cr-assistant/internal/llm"
	"cr-assistant/internal/report"
	"cr-assistant/internal/review"
	"cr-assistant/internal/standardsclient"
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
	// 1. Config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Параметры MR — проставляются GitLab CI автоматически.
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
		"gitlab_url", cfg.GitLab.BaseURL,
		"standards_service_url", cfg.StandardsService.URL,
	)

	// 3. Получаем стандарт кода из standards-service по HTTP.
	//    CI_JOB_TOKEN используется для аутентификации — GitLab проставляет его
	//    автоматически, владельцы репозиториев не управляют этим токеном.
	//    404 / недоступность сервиса — работаем без стандарта (LLM использует дефолты).
	var codeStandard string
	stdClient := standardsclient.New(
		cfg.StandardsService.URL,
		cfg.GitLab.JobToken,
		time.Duration(cfg.StandardsService.TimeoutSec)*time.Second,
	)

	std, err := stdClient.GetStandard(context.Background(), projectID)
	if err != nil {
		var notFound *standardsclient.ErrNotFound
		if errors.As(err, &notFound) {
			logger.Info("no code standard registered for this project, using LLM defaults",
				"project_id", projectID,
			)
		} else {
			// Сервис недоступен — логируем предупреждение, продолжаем без стандарта.
			logger.Warn("standards-service unavailable, proceeding without code standard",
				"error", err,
			)
		}
	} else {
		codeStandard = std.BuildPromptContext()
		logger.Info("code standard loaded",
			"version", std.Version,
			"has_custom_rules", std.CustomRules != "",
		)
	}

	// 4. Зависимости
	gitlabClient := gitlab.NewClient(
		cfg.GitLab.BaseURL,
		cfg.GitLab.JobToken,
		cfg.GitLab.BotToken,
		time.Duration(cfg.GitLab.TimeoutSec)*time.Second,
		logger,
	)
	llmGateway := llm.NewGateway(
		cfg.LLM.ServiceURL,
		time.Duration(cfg.LLM.TimeoutSec)*time.Second,
	)
	reportRenderer := report.NewRenderer()

	// 5. Загрузчик .codereview.yml — читает настройки из репозитория через GitLab API.
	//    CI_JOB_TOKEN имеет доступ к файлам репозитория по умолчанию.
	crLoader := codereview.NewLoader(
		cfg.GitLab.BaseURL,
		cfg.GitLab.JobToken,
		time.Duration(cfg.GitLab.TimeoutSec)*time.Second,
	)

	orchestrator := review.NewOrchestrator(
		gitlabClient,
		llmGateway,
		reportRenderer,
		crLoader,
		logger,
	)

	// 6. Анализ MR
	ctx := context.Background()
	result, err := orchestrator.Run(ctx, projectID, mrIID, codeStandard)
	if err != nil {
		return fmt.Errorf("orchestrator: %w", err)
	}

	// 7. Exit code определяется .codereview.yml block_on_critical
	//    (оркестратор уже прочитал конфиг; здесь используем HasCritical + cfg.Runner.BlockOnCritical
	//    как fallback для обратной совместимости с переменной окружения BLOCK_ON_CRITICAL).
	if cfg.Runner.BlockOnCritical && result.HasCritical {
		logger.Warn("critical issues found, blocking merge",
			"critical_count", result.TotalByLevel[domain.SeverityCritical],
		)
		fmt.Fprintln(os.Stderr, "❌ Обнаружены критические замечания. Merge заблокирован.")
		os.Exit(1)
	}

	fmt.Println("✅ Код-ревью завершено. Критических замечаний не найдено.")
	return nil
}

func loadConfig() (*config.Config, error) {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return config.LoadFromFile(path)
	}
	return config.Load()
}

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
