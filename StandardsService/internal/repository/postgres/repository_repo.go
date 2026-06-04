package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"standards-service/internal/domain"

	"github.com/google/uuid"
)

type RepositoryRepo struct {
	db *sql.DB
}

func NewRepositoryRepo(db *sql.DB) *RepositoryRepo {
	return &RepositoryRepo{db: db}
}

func (r *RepositoryRepo) Create(ctx context.Context, repo *domain.Repository) error {
	repo.ID = uuid.New().String()
	repo.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO repositories (id, team_id, gitlab_id, name, full_path, added_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		repo.ID, repo.TeamID, repo.GitLabID, repo.Name, repo.FullPath,
		repo.AddedBy, repo.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("repository create: %w", err)
	}
	return nil
}

func (r *RepositoryRepo) GetByID(ctx context.Context, id string) (*domain.Repository, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id, team_id, gitlab_id, name, full_path, added_by, created_at
		FROM repositories WHERE id = $1`, id))
}

func (r *RepositoryRepo) GetByGitLabID(ctx context.Context, gitlabID int) (*domain.Repository, error) {
	return r.scan(r.db.QueryRowContext(ctx, `
		SELECT id, team_id, gitlab_id, name, full_path, added_by, created_at
		FROM repositories WHERE gitlab_id = $1`, gitlabID))
}

func (r *RepositoryRepo) ListByTeam(ctx context.Context, teamID string) ([]*domain.Repository, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, team_id, gitlab_id, name, full_path, added_by, created_at
		FROM repositories WHERE team_id = $1 ORDER BY created_at ASC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("repository list: %w", err)
	}
	defer rows.Close()

	var repos []*domain.Repository
	for rows.Next() {
		repo := &domain.Repository{}
		if err := rows.Scan(
			&repo.ID, &repo.TeamID, &repo.GitLabID, &repo.Name,
			&repo.FullPath, &repo.AddedBy, &repo.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("repository list scan: %w", err)
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (r *RepositoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	return err
}

func (r *RepositoryRepo) scan(row *sql.Row) (*domain.Repository, error) {
	repo := &domain.Repository{}
	err := row.Scan(
		&repo.ID, &repo.TeamID, &repo.GitLabID, &repo.Name,
		&repo.FullPath, &repo.AddedBy, &repo.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("repository not found")
	}
	if err != nil {
		return nil, fmt.Errorf("repository scan: %w", err)
	}
	return repo, nil
}
