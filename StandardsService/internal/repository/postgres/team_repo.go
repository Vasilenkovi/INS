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

type TeamRepo struct {
	db *sql.DB
}

func NewTeamRepo(db *sql.DB) *TeamRepo {
	return &TeamRepo{db: db}
}

func (r *TeamRepo) Create(ctx context.Context, team *domain.Team) error {
	team.ID = uuid.New().String()
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO teams (id, name, slug, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		team.ID, team.Name, team.Slug, team.Description,
		team.CreatedBy, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("team create: %w", err)
	}
	return nil
}

func (r *TeamRepo) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	team := &domain.Team{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, description, created_by, created_at, updated_at
		FROM teams WHERE slug = $1`, slug,
	).Scan(
		&team.ID, &team.Name, &team.Slug, &team.Description,
		&team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("team %q not found", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("team get by slug: %w", err)
	}
	return team, nil
}

func (r *TeamRepo) GetByID(ctx context.Context, id string) (*domain.Team, error) {
	team := &domain.Team{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, description, created_by, created_at, updated_at
		FROM teams WHERE id = $1`, id,
	).Scan(
		&team.ID, &team.Name, &team.Slug, &team.Description,
		&team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("team %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("team get by id: %w", err)
	}
	return team, nil
}

func (r *TeamRepo) List(ctx context.Context) ([]*domain.Team, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, slug, description, created_by, created_at, updated_at
		FROM teams ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("team list: %w", err)
	}
	defer rows.Close()

	var teams []*domain.Team
	for rows.Next() {
		t := &domain.Team{}
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Slug, &t.Description,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("team list scan: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (r *TeamRepo) Update(ctx context.Context, team *domain.Team) error {
	team.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE teams SET name=$1, description=$2, updated_at=$3 WHERE id=$4`,
		team.Name, team.Description, team.UpdatedAt, team.ID,
	)
	if err != nil {
		return fmt.Errorf("team update: %w", err)
	}
	return nil
}

func (r *TeamRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM teams WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("team delete: %w", err)
	}
	return nil
}
