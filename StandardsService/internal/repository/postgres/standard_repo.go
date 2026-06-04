package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"standards-service/internal/domain"
)

// =============================================================================
// CodeStandardRepo
// =============================================================================

type CodeStandardRepo struct {
	db *sql.DB
}

func NewCodeStandardRepo(db *sql.DB) *CodeStandardRepo {
	return &CodeStandardRepo{db: db}
}

func (r *CodeStandardRepo) Upsert(ctx context.Context, standard *domain.CodeStandard) (*domain.CodeStandard, error) {
	existing, err := r.GetByTeamID(ctx, standard.TeamID)
	if err == nil {
		return existing, nil
	}

	standard.ID = uuid.New().String()
	standard.CreatedAt = time.Now()
	standard.UpdatedAt = time.Now()

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO code_standards (id, team_id, active_version_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)`,
		standard.ID, standard.TeamID, standard.ActiveVersionID,
		standard.CreatedAt, standard.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("code standard upsert: %w", err)
	}
	return standard, nil
}

func (r *CodeStandardRepo) GetByTeamID(ctx context.Context, teamID string) (*domain.CodeStandard, error) {
	s := &domain.CodeStandard{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, team_id, active_version_id, created_at, updated_at
		FROM code_standards WHERE team_id = $1`, teamID,
	).Scan(&s.ID, &s.TeamID, &s.ActiveVersionID, &s.CreatedAt, &s.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("code standard for team %q not found", teamID)
	}
	if err != nil {
		return nil, fmt.Errorf("code standard get: %w", err)
	}
	return s, nil
}

func (r *CodeStandardRepo) SetActiveVersion(ctx context.Context, standardID, versionID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE code_standards SET active_version_id = $1, updated_at = $2 WHERE id = $3`,
		versionID, time.Now(), standardID,
	)
	if err != nil {
		return fmt.Errorf("set active version: %w", err)
	}
	return nil
}

// =============================================================================
// StandardVersionRepo
// =============================================================================

type StandardVersionRepo struct {
	db *sql.DB
}

func NewStandardVersionRepo(db *sql.DB) *StandardVersionRepo {
	return &StandardVersionRepo{db: db}
}

func (r *StandardVersionRepo) Create(ctx context.Context, v *domain.StandardVersion) error {
	v.ID = uuid.New().String()
	v.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO standard_versions (id, code_standard_id, version, custom_rules, comment, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		v.ID, v.CodeStandardID, v.Version,
		v.CustomRules, v.Comment, v.CreatedBy, v.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("standard version create: %w", err)
	}
	return nil
}

func (r *StandardVersionRepo) GetByID(ctx context.Context, id string) (*domain.StandardVersion, error) {
	v := &domain.StandardVersion{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, code_standard_id, version, custom_rules, comment, created_by, created_at
		FROM standard_versions WHERE id = $1`, id,
	).Scan(
		&v.ID, &v.CodeStandardID, &v.Version,
		&v.CustomRules, &v.Comment, &v.CreatedBy, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("standard version %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("standard version get: %w", err)
	}
	return v, nil
}

func (r *StandardVersionRepo) GetActive(ctx context.Context, standardID string) (*domain.StandardVersion, error) {
	v := &domain.StandardVersion{}
	err := r.db.QueryRowContext(ctx, `
		SELECT sv.id, sv.code_standard_id, sv.version, sv.custom_rules,
		       sv.comment, sv.created_by, sv.created_at
		FROM standard_versions sv
		JOIN code_standards cs ON cs.active_version_id = sv.id
		WHERE cs.id = $1`, standardID,
	).Scan(
		&v.ID, &v.CodeStandardID, &v.Version,
		&v.CustomRules, &v.Comment, &v.CreatedBy, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no active version for standard %q", standardID)
	}
	if err != nil {
		return nil, fmt.Errorf("get active version: %w", err)
	}
	return v, nil
}

func (r *StandardVersionRepo) ListByStandard(ctx context.Context, standardID string) ([]*domain.StandardVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, code_standard_id, version, custom_rules, comment, created_by, created_at
		FROM standard_versions WHERE code_standard_id = $1 ORDER BY version DESC`, standardID)
	if err != nil {
		return nil, fmt.Errorf("standard version list: %w", err)
	}
	defer rows.Close()

	var versions []*domain.StandardVersion
	for rows.Next() {
		v := &domain.StandardVersion{}
		if err := rows.Scan(
			&v.ID, &v.CodeStandardID, &v.Version,
			&v.CustomRules, &v.Comment, &v.CreatedBy, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("standard version scan: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (r *StandardVersionRepo) GetNextVersion(ctx context.Context, standardID string) (int, error) {
	var max sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(version) FROM standard_versions WHERE code_standard_id = $1`, standardID,
	).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("get next version: %w", err)
	}
	return int(max.Int64) + 1, nil
}
