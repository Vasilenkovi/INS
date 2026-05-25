package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	handler "standards-service/internal/api/handlers"
	"standards-service/internal/api/middleware"
	"standards-service/internal/auth"
	"standards-service/internal/config"
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
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Database
	db, err := sql.Open("postgres", cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	logger.Info("database connected")

	// 3. Repositories
	teamRepo := postgres.NewTeamRepo(db)
	repoRepo := postgres.NewRepositoryRepo(db)
	standardRepo := postgres.NewCodeStandardRepo(db)
	versionRepo := postgres.NewStandardVersionRepo(db)

	// 4. Auth
	gitlabAuth := auth.NewGitLabAuthService(
		cfg.GitLab.BaseURL,
		time.Duration(cfg.GitLab.TimeoutSec)*time.Second,
	)

	// 5. Services
	teamSvc := service.NewTeamService(teamRepo, gitlabAuth)
	repoSvc := service.NewRepositoryService(repoRepo, teamRepo, gitlabAuth)
	standardSvc := service.NewStandardService(standardRepo, versionRepo, teamRepo, gitlabAuth)

	// 6. Handlers
	teamH := handler.NewTeamHandler(teamSvc)
	repoH := handler.NewRepositoryHandler(repoSvc)
	standardH := handler.NewStandardHandler(standardSvc)

	// 7. Router
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1", middleware.Auth(gitlabAuth))
	{
		// Teams
		api.POST("/teams", teamH.Create)
		api.GET("/teams", teamH.List)
		api.GET("/teams/:slug", teamH.Get)
		api.PATCH("/teams/:slug", teamH.Update)
		api.DELETE("/teams/:slug", teamH.Delete)

		// Repositories
		api.POST("/teams/:slug/repos", repoH.Add)
		api.GET("/teams/:slug/repos", repoH.List)
		api.DELETE("/teams/:slug/repos/:repo_id", repoH.Remove)

		// Standards
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
