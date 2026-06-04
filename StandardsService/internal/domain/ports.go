package domain

import (
	"context"
	"fmt"
)

// =============================================================================
// Repository ports
// =============================================================================

type TeamRepository interface {
	Create(ctx context.Context, team *Team) error
	GetBySlug(ctx context.Context, slug string) (*Team, error)
	GetByID(ctx context.Context, id string) (*Team, error)
	List(ctx context.Context) ([]*Team, error)
	Update(ctx context.Context, team *Team) error
	Delete(ctx context.Context, id string) error
}

type RepositoryRepository interface {
	Create(ctx context.Context, repo *Repository) error
	GetByID(ctx context.Context, id string) (*Repository, error)
	GetByGitLabID(ctx context.Context, gitlabID int) (*Repository, error)
	ListByTeam(ctx context.Context, teamID string) ([]*Repository, error)
	Delete(ctx context.Context, id string) error
}

type CodeStandardRepository interface {
	Upsert(ctx context.Context, standard *CodeStandard) (*CodeStandard, error)
	GetByTeamID(ctx context.Context, teamID string) (*CodeStandard, error)
	SetActiveVersion(ctx context.Context, standardID string, versionID string) error
}

type StandardVersionRepository interface {
	Create(ctx context.Context, version *StandardVersion) error
	GetByID(ctx context.Context, id string) (*StandardVersion, error)
	GetActive(ctx context.Context, standardID string) (*StandardVersion, error)
	ListByStandard(ctx context.Context, standardID string) ([]*StandardVersion, error)
	GetNextVersion(ctx context.Context, standardID string) (int, error)
}

// =============================================================================
// Auth port
// =============================================================================

// AccessLevel — уровень доступа GitLab (числа совпадают с GitLab API).
type AccessLevel int

const (
	AccessLevelGuest      AccessLevel = 10
	AccessLevelReporter   AccessLevel = 20
	AccessLevelDeveloper  AccessLevel = 30
	AccessLevelMaintainer AccessLevel = 40
	AccessLevelOwner      AccessLevel = 50
)

// GitLabUser — пользователь, аутентифицированный через GitLab PAT.
type GitLabUser struct {
	ID       int
	Username string
	Name     string
}

// AuthPort — проверка прав через GitLab API.
type AuthPort interface {
	VerifyUser(ctx context.Context, token string) (*GitLabUser, error)
	VerifyJobToken(ctx context.Context, token string) error
	GetProjectAccessLevel(ctx context.Context, token string, projectID, userID int) (AccessLevel, error)
}

// =============================================================================
// AccessChecker — единый сервис проверки прав через GitLab Project
// =============================================================================

type AccessChecker struct {
	auth          AuthPort
	minReadLevel  AccessLevel
	minWriteLevel AccessLevel
}

func NewAccessChecker(auth AuthPort, minReadLevel, minWriteLevel AccessLevel) *AccessChecker {
	return &AccessChecker{
		auth:          auth,
		minReadLevel:  minReadLevel,
		minWriteLevel: minWriteLevel,
	}
}

// CheckRead проверяет право на чтение. При minReadLevel == 0 доступно всем авторизованным.
func (a *AccessChecker) CheckRead(ctx context.Context, token string, projectID, userID int) error {
	if a.minReadLevel == 0 {
		return nil
	}
	level, err := a.auth.GetProjectAccessLevel(ctx, token, projectID, userID)
	if err != nil || level < a.minReadLevel {
		return fmt.Errorf("forbidden: read access requires level %d in project %d", a.minReadLevel, projectID)
	}
	return nil
}

// CheckWrite проверяет право на запись в проекте.
func (a *AccessChecker) CheckWrite(ctx context.Context, token string, projectID, userID int) error {
	level, err := a.auth.GetProjectAccessLevel(ctx, token, projectID, userID)
	if err != nil || level < a.minWriteLevel {
		return fmt.Errorf("forbidden: write access requires level %d in project %d", a.minWriteLevel, projectID)
	}
	return nil
}
