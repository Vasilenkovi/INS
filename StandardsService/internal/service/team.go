package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"

	"github.com/google/uuid"
)

type TeamService struct {
	teams   domain.TeamRepository
	repos   domain.RepositoryRepository
	checker *domain.AccessChecker
	auth    domain.AuthPort
}

func NewTeamService(
	teams domain.TeamRepository,
	repos domain.RepositoryRepository,
	checker *domain.AccessChecker,
	auth domain.AuthPort,
) *TeamService {
	return &TeamService{teams: teams, repos: repos, checker: checker, auth: auth}
}

// Create создаёт команду вокруг указанного GitLab-проекта.
// Пользователь должен иметь write-доступ в указанном репозитории.
// Репозиторий автоматически добавляется как первый в команде.
func (s *TeamService) Create(ctx context.Context, team *domain.Team, gitlabProjectID int, repoName, repoFullPath string, token string) (*domain.Team, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	if err := s.checker.CheckWrite(ctx, token, gitlabProjectID, user.ID); err != nil {
		return nil, err
	}

	team.ID = uuid.NewString()
	team.CreatedBy = user.Username

	if err := team.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	if err := s.teams.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	// Первый репозиторий команды
	repo := &domain.Repository{
		TeamID:   team.ID,
		GitLabID: gitlabProjectID,
		Name:     repoName,
		FullPath: repoFullPath,
		AddedBy:  user.Username,
	}
	if err := s.repos.Create(ctx, repo); err != nil {
		return nil, fmt.Errorf("add initial repository: %w", err)
	}

	return team, nil
}

func (s *TeamService) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	return s.teams.GetBySlug(ctx, slug)
}

func (s *TeamService) List(ctx context.Context) ([]*domain.Team, error) {
	return s.teams.List(ctx)
}

// Update обновляет имя и описание команды.
// Для проверки прав используется первый репозиторий команды.
func (s *TeamService) Update(ctx context.Context, slug string, name, description string, token string) (*domain.Team, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	projectID, err := s.firstProjectID(ctx, team.ID)
	if err != nil {
		return nil, err
	}

	if err := s.checker.CheckWrite(ctx, token, projectID, user.ID); err != nil {
		return nil, err
	}

	team.Name = name
	team.Description = description

	if err := s.teams.Update(ctx, team); err != nil {
		return nil, err
	}
	return team, nil
}

// Delete удаляет команду. Требует write-доступа в первом репозитории.
func (s *TeamService) Delete(ctx context.Context, slug string, token string) error {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}

	projectID, err := s.firstProjectID(ctx, team.ID)
	if err != nil {
		return err
	}

	if err := s.checker.CheckWrite(ctx, token, projectID, user.ID); err != nil {
		return err
	}

	return s.teams.Delete(ctx, team.ID)
}

// firstProjectID возвращает GitLabID первого (любого) репозитория команды.
func (s *TeamService) firstProjectID(ctx context.Context, teamID string) (int, error) {
	repos, err := s.repos.ListByTeam(ctx, teamID)
	if err != nil || len(repos) == 0 {
		return 0, fmt.Errorf("team has no repositories")
	}
	return repos[0].GitLabID, nil
}
