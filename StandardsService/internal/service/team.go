package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"

	"github.com/google/uuid"
)

type TeamService struct {
	teams domain.TeamRepository
	auth  domain.AuthPort
}

func NewTeamService(teams domain.TeamRepository, auth domain.AuthPort) *TeamService {
	return &TeamService{teams: teams, auth: auth}
}

// Create создаёт команду. Требует Maintainer в указанной GitLab-группе.
func (s *TeamService) Create(ctx context.Context, team *domain.Team, token string) (*domain.Team, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	level, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || level < domain.AccessLevelMaintainer {
		return nil, fmt.Errorf("forbidden: maintainer access required in group %d", team.GitLabGroupID)
	}

	group, err := s.auth.GetGroupByID(ctx, token, team.GitLabGroupID)
	if err != nil {
		return nil, fmt.Errorf("gitlab group not found: %w", err)
	}

	team.ID = uuid.NewString()
	team.GitLabGroupPath = group.FullPath
	team.CreatedBy = user.Username

	if err := team.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	if err := s.teams.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	return team, nil
}

func (s *TeamService) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	return s.teams.GetBySlug(ctx, slug)
}

func (s *TeamService) List(ctx context.Context) ([]*domain.Team, error) {
	return s.teams.List(ctx)
}

// Update обновляет команду. Требует Maintainer в группе команды.
func (s *TeamService) Update(ctx context.Context, slug string, name, description string, token string) (*domain.Team, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	level, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || level < domain.AccessLevelMaintainer {
		return nil, fmt.Errorf("forbidden: maintainer access required")
	}

	team.Name = name
	team.Description = description

	if err := s.teams.Update(ctx, team); err != nil {
		return nil, err
	}
	return team, nil
}

// Delete удаляет команду. Требует Maintainer.
func (s *TeamService) Delete(ctx context.Context, slug string, token string) error {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}

	level, err := s.auth.GetGroupAccessLevel(ctx, token, team.GitLabGroupID, user.ID)
	if err != nil || level < domain.AccessLevelMaintainer {
		return fmt.Errorf("forbidden: maintainer access required")
	}

	return s.teams.Delete(ctx, team.ID)
}
