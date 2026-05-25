package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"
)

type RepositoryService struct {
	repos domain.RepositoryRepository
	teams domain.TeamRepository
	auth  domain.AuthPort
}

func NewRepositoryService(
	repos domain.RepositoryRepository,
	teams domain.TeamRepository,
	auth domain.AuthPort,
) *RepositoryService {
	return &RepositoryService{repos: repos, teams: teams, auth: auth}
}

// Add привязывает GitLab-проект к команде.
// Требует: пользователь — Maintainer проекта И Maintainer группы команды.
func (s *RepositoryService) Add(ctx context.Context, teamSlug string, repo *domain.Repository, token string) (*domain.Repository, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	// Проверяем права на GitLab-проект
	projectLevel, err := s.auth.GetProjectAccessLevel(ctx, token, repo.GitLabID, user.ID)
	if err != nil || projectLevel < domain.AccessLevelMaintainer {
		return nil, fmt.Errorf("forbidden: maintainer access required on gitlab project %d", repo.GitLabID)
	}

	// Проверяем права в группе команды
	groupLevel, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || groupLevel < domain.AccessLevelMaintainer {
		return nil, fmt.Errorf("forbidden: maintainer access required in team group")
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

// ListByTeam возвращает репозитории команды. Доступно Developer и выше.
func (s *RepositoryService) ListByTeam(ctx context.Context, teamSlug string, token string) ([]*domain.Repository, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	level, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || level < domain.AccessLevelDeveloper {
		return nil, fmt.Errorf("forbidden: developer access required")
	}

	return s.repos.ListByTeam(ctx, team.ID)
}

// Remove отвязывает репозиторий от команды. Требует Maintainer.
func (s *RepositoryService) Remove(ctx context.Context, teamSlug, repoID string, token string) error {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return err
	}

	level, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || level < domain.AccessLevelMaintainer {
		return fmt.Errorf("forbidden: maintainer access required")
	}

	return s.repos.Delete(ctx, repoID)
}
