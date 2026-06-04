package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"
)

// CIStandardResponse — минимальный ответ для CI job.
type CIStandardResponse struct {
	CustomRules string `json:"custom_rules"`
	Version     int    `json:"version"`
}

// CIService обслуживает запросы от cr-assistant CI job.
// Аутентификация — через CI_JOB_TOKEN.
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
// token — CI_JOB_TOKEN из пайплайна.
func (s *CIService) GetStandardForProject(ctx context.Context, gitlabProjectID int, token string) (*CIStandardResponse, error) {
	if err := s.auth.VerifyJobToken(ctx, token); err != nil {
		return nil, fmt.Errorf("unauthorized: invalid job token: %w", err)
	}

	repo, err := s.repos.GetByGitLabID(ctx, gitlabProjectID)
	if err != nil {
		return nil, fmt.Errorf("not found: project %d is not registered: %w", gitlabProjectID, err)
	}

	standard, err := s.standards.GetByTeamID(ctx, repo.TeamID)
	if err != nil {
		return nil, fmt.Errorf("not found: no standard for project %d: %w", gitlabProjectID, err)
	}

	version, err := s.versions.GetActive(ctx, standard.ID)
	if err != nil {
		return nil, fmt.Errorf("not found: no active standard version for project %d: %w", gitlabProjectID, err)
	}

	return &CIStandardResponse{
		CustomRules: version.CustomRules,
		Version:     version.Version,
	}, nil
}
