package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"
)

type RepositoryService struct {
	repos   domain.RepositoryRepository
	teams   domain.TeamRepository
	checker *domain.AccessChecker
	auth    domain.AuthPort
}

func NewRepositoryService(
	repos domain.RepositoryRepository,
	teams domain.TeamRepository,
	checker *domain.AccessChecker,
	auth domain.AuthPort,
) *RepositoryService {
	return &RepositoryService{repos: repos, teams: teams, checker: checker, auth: auth}
}

// Add привязывает GitLab-проект к команде.
// Пользователь должен обладать write-доступом в добавляемом проекте.
func (s *RepositoryService) Add(ctx context.Context, teamSlug string, repo *domain.Repository, token string) (*domain.Repository, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	if err := s.checker.CheckWrite(ctx, token, repo.GitLabID, user.ID); err != nil {
		return nil, err
	}

	repo.TeamID = team.ID
	repo.AddedBy = user.Username

	if err := repo.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	if err := s.repos.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("add repository: %w", err)
	}
	return repo, nil
}

// ListByTeam возвращает репозитории команды.
// Чтение доступно всем авторизованным (MIN_READ_ACCESS_LEVEL=0 по умолчанию).
func (s *RepositoryService) ListByTeam(ctx context.Context, teamSlug string, token string) ([]*domain.Repository, error) {
	if _, err := s.auth.VerifyUser(ctx, token); err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	return s.repos.ListByTeam(ctx, team.ID)
}

// Remove отвязывает репозиторий от команды. Требует write-доступа в проекте.
func (s *RepositoryService) Remove(ctx context.Context, teamSlug, repoID string, token string) error {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	repo, err := s.repos.GetByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	if err := s.checker.CheckWrite(ctx, token, repo.GitLabID, user.ID); err != nil {
		return err
	}

	return s.repos.Delete(ctx, repoID)
}
