package service

import (
	"context"
	"fmt"

	"standards-service/internal/domain"
)

type StandardService struct {
	standards domain.CodeStandardRepository
	versions  domain.StandardVersionRepository
	teams     domain.TeamRepository
	repos     domain.RepositoryRepository
	checker   *domain.AccessChecker
	auth      domain.AuthPort
}

func NewStandardService(
	standards domain.CodeStandardRepository,
	versions domain.StandardVersionRepository,
	teams domain.TeamRepository,
	repos domain.RepositoryRepository,
	checker *domain.AccessChecker,
	auth domain.AuthPort,
) *StandardService {
	return &StandardService{
		standards: standards,
		versions:  versions,
		teams:     teams,
		repos:     repos,
		checker:   checker,
		auth:      auth,
	}
}

// Upload загружает новую версию стандарта для команды.
// Автоматически создаёт CodeStandard если его ещё нет.
// Новая версия автоматически становится активной.
// Требует write-доступа в первом репозитории команды.
func (s *StandardService) Upload(ctx context.Context, teamSlug string, input *domain.StandardVersion, token string) (*domain.StandardVersion, error) {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	projectID, err := s.firstProjectID(ctx, team.ID)
	if err != nil {
		return nil, err
	}

	if err := s.checker.CheckWrite(ctx, token, projectID, user.ID); err != nil {
		return nil, err
	}

	standard, err := s.standards.Upsert(ctx, &domain.CodeStandard{TeamID: team.ID})
	if err != nil {
		return nil, fmt.Errorf("upsert standard: %w", err)
	}

	nextVersion, err := s.versions.GetNextVersion(ctx, standard.ID)
	if err != nil {
		return nil, fmt.Errorf("get next version: %w", err)
	}

	input.CodeStandardID = standard.ID
	input.Version = nextVersion
	input.CreatedBy = user.Username

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	if err := s.versions.Create(ctx, input); err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	// Новая версия автоматически становится активной
	if err := s.standards.SetActiveVersion(ctx, standard.ID, input.ID); err != nil {
		return nil, fmt.Errorf("set active version: %w", err)
	}

	return input, nil
}

// GetActive возвращает активную версию стандарта команды.
// Доступно всем авторизованным пользователям (MIN_READ_ACCESS_LEVEL=0).
func (s *StandardService) GetActive(ctx context.Context, teamSlug string, token string) (*domain.StandardVersion, error) {
	if _, err := s.auth.VerifyUser(ctx, token); err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	standard, err := s.standards.GetByTeamID(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("standard not found for team %q", teamSlug)
	}

	if standard.ActiveVersionID == nil {
		return nil, fmt.Errorf("no active version")
	}

	return s.versions.GetByID(ctx, *standard.ActiveVersionID)
}

// ListVersions возвращает все версии стандарта команды.
// Доступно всем авторизованным пользователям (MIN_READ_ACCESS_LEVEL=0).
func (s *StandardService) ListVersions(ctx context.Context, teamSlug string, token string) ([]*domain.StandardVersion, error) {
	if _, err := s.auth.VerifyUser(ctx, token); err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	standard, err := s.standards.GetByTeamID(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("standard not found for team %q", teamSlug)
	}

	return s.versions.ListByStandard(ctx, standard.ID)
}

// SetActiveVersion переключает активную версию вручную. Требует write-доступа.
func (s *StandardService) SetActiveVersion(ctx context.Context, teamSlug, versionID string, token string) error {
	user, err := s.auth.VerifyUser(ctx, token)
	if err != nil {
		return fmt.Errorf("unauthorized: %w", err)
	}

	team, err := s.teams.GetBySlug(ctx, teamSlug)
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

	standard, err := s.standards.GetByTeamID(ctx, team.ID)
	if err != nil {
		return err
	}

	version, err := s.versions.GetByID(ctx, versionID)
	if err != nil || version.CodeStandardID != standard.ID {
		return fmt.Errorf("version %q not found in team standard", versionID)
	}

	return s.standards.SetActiveVersion(ctx, standard.ID, versionID)
}

func (s *StandardService) firstProjectID(ctx context.Context, teamID string) (int, error) {
	repos, err := s.repos.ListByTeam(ctx, teamID)
	if err != nil || len(repos) == 0 {
		return 0, fmt.Errorf("team has no repositories")
	}
	return repos[0].GitLabID, nil
}
