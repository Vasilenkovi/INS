package domain

import "context"

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
	// Upsert создаёт стандарт для команды если его нет, иначе возвращает существующий.
	Upsert(ctx context.Context, standard *CodeStandard) (*CodeStandard, error)
	GetByTeamID(ctx context.Context, teamID string) (*CodeStandard, error)
	SetActiveVersion(ctx context.Context, standardID string, versionID string) error
}

type StandardVersionRepository interface {
	Create(ctx context.Context, version *StandardVersion) error
	GetByID(ctx context.Context, id string) (*StandardVersion, error)
	GetActive(ctx context.Context, standardID string) (*StandardVersion, error)
	ListByStandard(ctx context.Context, standardID string) ([]*StandardVersion, error)
	// GetNextVersion возвращает следующий номер версии для стандарта.
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

// GitLabGroup — группа GitLab с уровнем доступа текущего пользователя.
type GitLabGroup struct {
	ID          int
	Name        string
	FullPath    string
	AccessLevel AccessLevel
}

// AuthPort — проверка прав через GitLab API.
type AuthPort interface {
	// VerifyUser проверяет токен и возвращает пользователя.
	VerifyUser(ctx context.Context, token string) (*GitLabUser, error)

	// GetGroupAccessLevel возвращает уровень доступа пользователя в группе.
	GetGroupAccessLevel(ctx context.Context, token string, groupID, userID int) (AccessLevel, error)

	// GetProjectAccessLevel возвращает уровень доступа пользователя в проекте.
	GetProjectAccessLevel(ctx context.Context, token string, projectID, userID int) (AccessLevel, error)

	// GetGroupByID возвращает информацию о группе GitLab.
	GetGroupByID(ctx context.Context, token string, groupID int) (*GitLabGroup, error)
}
