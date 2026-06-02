package standardrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

// ActiveStandard — всё что боту нужно от стандарта кода.
// Бот не работает с полной моделью standards-service —
// только читает активную версию для своего проекта.
type ActiveStandard struct {
	Preset      string
	CustomRules string
	Language    string
}

// StandardReader читает активный стандарт из общей PostgreSQL БД.
// Схема таблиц определена в standards-service/migrations/001_init.up.sql.
type StandardReader struct {
	db *sql.DB
}

func NewStandardReader(db *sql.DB) *StandardReader {
	return &StandardReader{db: db}
}

// GetByGitLabProjectID возвращает активный стандарт для GitLab-проекта.
//
// Цепочка JOIN-ов:
//
//	repositories (gitlab_id) → code_standards (team_id) → standard_versions (active_version_id)
func (r *StandardReader) GetByGitLabProjectID(ctx context.Context, gitlabProjectID int) (*ActiveStandard, error) {
	var s ActiveStandard

	err := r.db.QueryRowContext(ctx, `
		SELECT sv.preset, sv.custom_rules, sv.language
		FROM repositories rep
		JOIN code_standards cs ON cs.team_id = rep.team_id
		JOIN standard_versions sv ON sv.id = cs.active_version_id
		WHERE rep.gitlab_id = $1
	`, gitlabProjectID).Scan(&s.Preset, &s.CustomRules, &s.Language)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no active standard for gitlab project %d", gitlabProjectID)
	}
	if err != nil {
		return nil, fmt.Errorf("get standard: %w", err)
	}

	return &s, nil
}

// GetByTeamSlug возвращает активный стандарт по slug команды.
// Используется как fallback если привязка по project_id не найдена.
func (r *StandardReader) GetByTeamSlug(ctx context.Context, teamSlug string) (*ActiveStandard, error) {
	var s ActiveStandard

	err := r.db.QueryRowContext(ctx, `
		SELECT sv.preset, sv.custom_rules, sv.language
		FROM teams t
		JOIN code_standards cs ON cs.team_id = t.id
		JOIN standard_versions sv ON sv.id = cs.active_version_id
		WHERE t.slug = $1
	`, teamSlug).Scan(&s.Preset, &s.CustomRules, &s.Language)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no active standard for team %q", teamSlug)
	}
	if err != nil {
		return nil, fmt.Errorf("get standard by team: %w", err)
	}

	return &s, nil
}

// BuildPromptContext формирует строку стандарта для подстановки в системный промпт LLM.
// Если задан пресет и кастомные правила — объединяет их.
func (s *ActiveStandard) BuildPromptContext() string {
	if s.Preset != "" && s.CustomRules != "" {
		return fmt.Sprintf("Base preset: %s\n\nAdditional rules:\n%s", s.Preset, s.CustomRules)
	}
	if s.Preset != "" {
		return fmt.Sprintf("Base preset: %s", s.Preset)
	}
	return s.CustomRules
}
