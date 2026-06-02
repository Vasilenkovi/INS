package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"standards-service/internal/api/handler"
	"standards-service/internal/api/middleware"
	"standards-service/internal/auth"
	"standards-service/internal/config"
	"standards-service/internal/db"
	"standards-service/internal/repository/postgres"
	"standards-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// 1. Config
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 2. Database — подключение и миграции
	// MIGRATIONS_PATH можно переопределить через env (полезно в Docker/тестах).
	// По умолчанию "file:///migrations" — абсолютный путь, совместимый со scratch-образом.
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file:///migrations"
	}
	dbCfg := db.DefaultConfig(cfg.Database.DSN, migrationsPath)
	database, err := db.Open(dbCfg, logger)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database, dbCfg, logger); err != nil {
		return err
	}

	// 3. Repositories
	teamRepo := postgres.NewTeamRepo(database)
	repoRepo := postgres.NewRepositoryRepo(database)
	standardRepo := postgres.NewCodeStandardRepo(database)
	versionRepo := postgres.NewStandardVersionRepo(database)

	// 4. Auth
	gitlabAuth := auth.NewGitLabAuthService(
		cfg.GitLab.BaseURL,
		time.Duration(cfg.GitLab.TimeoutSec)*time.Second,
	)

	// 5. Services
	teamSvc := service.NewTeamService(teamRepo, gitlabAuth)
	repoSvc := service.NewRepositoryService(repoRepo, teamRepo, gitlabAuth)
	standardSvc := service.NewStandardService(standardRepo, versionRepo, teamRepo, gitlabAuth)
	ciSvc := service.NewCIService(repoRepo, standardRepo, versionRepo, gitlabAuth)

	// 6. Handlers
	teamH := handler.NewTeamHandler(teamSvc)
	repoH := handler.NewRepositoryHandler(repoSvc)
	standardH := handler.NewStandardHandler(standardSvc)
	ciH := handler.NewCIHandler(ciSvc)

	// 7. Router
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// /internal/v1/ — machine-to-machine, аутентификация по CI_JOB_TOKEN.
	// Токен проверяется внутри handler'а (не через Bearer middleware).
	internal := r.Group("/internal/v1")
	{
		internal.GET("/projects/:gitlab_project_id/standard", ciH.GetStandard)
	}

	api := r.Group("/api/v1", middleware.Auth(gitlabAuth))
	{
		api.POST("/teams", teamH.Create)
		api.GET("/teams", teamH.List)
		api.GET("/teams/:slug", teamH.Get)
		api.PATCH("/teams/:slug", teamH.Update)
		api.DELETE("/teams/:slug", teamH.Delete)

		api.POST("/teams/:slug/repos", repoH.Add)
		api.GET("/teams/:slug/repos", repoH.List)
		api.DELETE("/teams/:slug/repos/:repo_id", repoH.Remove)

		api.POST("/teams/:slug/standards", standardH.Upload)
		api.GET("/teams/:slug/standards/active", standardH.GetActive)
		api.GET("/teams/:slug/standards/versions", standardH.ListVersions)
		api.PUT("/teams/:slug/standards/versions/:version_id/activate", standardH.Activate)
	}

	logger.Info("starting standards-service", "port", cfg.Server.Port)
	return r.Run(":" + cfg.Server.Port)
}

func loadConfig() (*config.Config, error) {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return config.LoadFromFile(path)
	}
	return config.Load()
}
