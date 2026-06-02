package domain

import (
	"errors"
	"time"
)

// =============================================================================
// Team
// =============================================================================

type Team struct {
	ID          string
	Name        string
	Slug        string // уникальный идентификатор команды, URL-safe
	Description string

	// Привязка к GitLab-группе — источник прав доступа.
	GitLabGroupID   int
	GitLabGroupPath string // например "company/backend"

	CreatedBy string // GitLab username
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Team) Validate() error {
	if t.Name == "" {
		return errors.New("name is required")
	}
	if t.Slug == "" {
		return errors.New("slug is required")
	}
	if t.GitLabGroupID == 0 {
		return errors.New("gitlab_group_id is required")
	}
	return nil
}

// =============================================================================
// Repository
// =============================================================================

type Repository struct {
	ID       string
	TeamID   string
	GitLabID int // GitLab project ID
	Name     string
	FullPath string // например "company/backend/auth-service"

	// Список glob-паттернов файлов/папок исключённых из анализа.
	// Дублирует .codereview.yml, но хранится централизованно.
	Excludes []string

	AddedBy   string // GitLab username
	CreatedAt time.Time
}

func (r *Repository) Validate() error {
	if r.TeamID == "" {
		return errors.New("team_id is required")
	}
	if r.GitLabID == 0 {
		return errors.New("gitlab_project_id is required")
	}
	return nil
}

// =============================================================================
// CodeStandard
// =============================================================================

// CodeStandard — стандарт кода команды.
// Один стандарт на команду; версии хранятся в StandardVersion.
type CodeStandard struct {
	ID     string
	TeamID string

	// ID активной версии стандарта.
	// NULL допустим — если стандарт только создан и версий ещё нет.
	ActiveVersionID *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// =============================================================================
// StandardVersion
// =============================================================================

// StandardVersion — одна версия стандарта кода.
// Версии иммутабельны: новая загрузка создаёт новую версию,
// а не изменяет существующую.
type StandardVersion struct {
	ID             string
	CodeStandardID string

	Version int // монотонно возрастающий номер, 1, 2, 3, ...

	// Пресет из встроенных: "PEP8", "Google Python Style Guide",
	// "Airbnb JavaScript Style Guide", "StandardJS" и т.д.
	// Пустая строка означает что пресет не используется.
	Preset string

	// Произвольный стандарт в Markdown или plain text.
	// Может дополнять или переопределять пресет.
	CustomRules string

	// Язык комментариев бота для этой команды.
	Language string // "ru", "en", ...

	Comment string // необязательный комментарий к версии (что изменилось)

	CreatedBy string // GitLab username
	CreatedAt time.Time
}

func (v *StandardVersion) Validate() error {
	if v.CodeStandardID == "" {
		return errors.New("code_standard_id is required")
	}
	if v.Preset == "" && v.CustomRules == "" {
		return errors.New("either preset or custom_rules must be provided")
	}
	return nil
}
