package domain

import (
	"errors"
	"time"
)

// =============================================================================
// Team
// =============================================================================

type Team struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`

	GitLabGroupID   int    `json:"gitlab_group_id"`
	GitLabGroupPath string `json:"gitlab_group_path"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	ID             string `json:"id"`
	CodeStandardID string `json:"code_standard_id"`

	Version int `json:"version"` // монотонно возрастающий номер, 1, 2, 3, ...

	// Пресет из встроенных: "PEP8", "Google Python Style Guide",
	// "Airbnb JavaScript Style Guide", "StandardJS" и т.д.
	// Пустая строка означает что пресет не используется.
	Preset string `json:"preset"`

	// Произвольный стандарт в Markdown или plain text.
	// Может дополнять или переопределять пресет.
	CustomRules string `json:"custom_rules"`

	// Язык комментариев бота для этой команды.
	Language string `json:"language"`

	Comment string `json:"comment"` // необязательный комментарий к версии (что изменилось)

	CreatedBy string    `json:"created_by"` // GitLab username
	CreatedAt time.Time `json:"created_at"`
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
