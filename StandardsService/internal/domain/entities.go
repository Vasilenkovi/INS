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
// а не изменяет существующую. После загрузки версия автоматически
// становится активной.
type StandardVersion struct {
	ID             string `json:"id"`
	CodeStandardID string `json:"code_standard_id"`

	Version int `json:"version"` // монотонно возрастающий номер, 1, 2, 3, ...

	// Произвольный стандарт в Markdown или plain text.
	CustomRules string `json:"custom_rules"`

	Comment string `json:"comment"` // необязательный комментарий к версии (что изменилось)

	CreatedBy string    `json:"created_by"` // GitLab username
	CreatedAt time.Time `json:"created_at"`
}

func (v *StandardVersion) Validate() error {
	if v.CodeStandardID == "" {
		return errors.New("code_standard_id is required")
	}
	if v.CustomRules == "" {
		return errors.New("custom_rules must be provided")
	}
	return nil
}
