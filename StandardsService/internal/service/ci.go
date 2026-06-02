package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"
)

// CIStandardResponse — минимальный ответ для CI job.
// Не содержит внутренних ID и метаданных, только то что нужно боту.
type CIStandardResponse struct {
	Preset      string `json:"preset"`
	CustomRules string `json:"custom_rules"`
	Language    string `json:"language"`
	Version     int    `json:"version"`
}

// CIService обслуживает запросы от cr-assistant CI job.
// Аутентификация — через CI_JOB_TOKEN (GitLab job token), который
// проверяется через GitLab API /api/v4/job (не требует PAT).
type CIService struct {
	repos     domain.RepositoryRepository
	standards domain.CodeStandardRepository
	versions  domain.StandardVersionRepository
	auth      domain.AuthPort
}

func NewCIService(
	repos domain.RepositoryRepository,
	standards domain.CodeStandardRepository,
	versions domain.StandardVersionRepository,
	auth domain.AuthPort,
) *CIService {
	return &CIService{
		repos:     repos,
		standards: standards,
		versions:  versions,
		auth:      auth,
	}
}

// GetStandardForProject возвращает активный стандарт для GitLab-проекта.
// token — CI_JOB_TOKEN из пайплайна, проверяется через GitLab API.
// Возвращает ErrNotFound-совместимую ошибку если стандарт не настроен —
// cr-assistant в этом случае продолжает без стандарта.
func (s *CIService) GetStandardForProject(ctx context.Context, gitlabProjectID int, token string) (*CIStandardResponse, error) {
	// Проверяем что токен валидный CI_JOB_TOKEN.
	// VerifyJobToken не проверяет права в группе — достаточно что токен живой.
	if err := s.auth.VerifyJobToken(ctx, token); err != nil {
		return nil, fmt.Errorf("unauthorized: invalid job token: %w", err)
	}

	// Ищем репозиторий по GitLab project ID.
	repo, err := s.repos.GetByGitLabID(ctx, gitlabProjectID)
	if err != nil {
		return nil, fmt.Errorf("not found: project %d is not registered: %w", gitlabProjectID, err)
	}

	// Ищем стандарт команды.
	standard, err := s.standards.GetByTeamID(ctx, repo.TeamID)
	if err != nil {
		return nil, fmt.Errorf("not found: no standard for project %d: %w", gitlabProjectID, err)
	}

	// Получаем активную версию.
	version, err := s.versions.GetActive(ctx, standard.ID)
	if err != nil {
		return nil, fmt.Errorf("not found: no active standard version for project %d: %w", gitlabProjectID, err)
	}

	return &CIStandardResponse{
		Preset:      version.Preset,
		CustomRules: version.CustomRules,
		Language:    version.Language,
		Version:     version.Version,
	}, nil
}
