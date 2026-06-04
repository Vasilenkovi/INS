package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"standards-service/internal/domain"
)

// GitLabAuthService реализует domain.AuthPort.
// Все проверки прав делегируются GitLab API — своей БД пользователей нет.
type GitLabAuthService struct {
	gitlabURL  string
	httpClient *http.Client
}

func NewGitLabAuthService(gitlabURL string, timeout time.Duration) *GitLabAuthService {
	return &GitLabAuthService{
		gitlabURL: gitlabURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// VerifyJobToken проверяет CI_JOB_TOKEN через GET /api/v4/job.
func (s *GitLabAuthService) VerifyJobToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.gitlabURL+"/api/v4/job", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("JOB-TOKEN", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid job token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// VerifyUser проверяет токен через GET /api/v4/user.
func (s *GitLabAuthService) VerifyUser(ctx context.Context, token string) (*domain.GitLabUser, error) {
	var user struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}

	if err := s.get(ctx, token, "/api/v4/user", &user); err != nil {
		return nil, fmt.Errorf("verify user: %w", err)
	}

	return &domain.GitLabUser{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
	}, nil
}

// GetProjectAccessLevel возвращает уровень доступа пользователя в проекте.
// Сначала пробует прямых членов, при 404 — унаследованных.
// GET /api/v4/projects/:id/members/:user_id
// GET /api/v4/projects/:id/members/all/:user_id
func (s *GitLabAuthService) GetProjectAccessLevel(ctx context.Context, token string, projectID, userID int) (domain.AccessLevel, error) {
	var member struct {
		AccessLevel int `json:"access_level"`
	}

	path := fmt.Sprintf("/api/v4/projects/%d/members/%d", projectID, userID)
	err := s.get(ctx, token, path, &member)
	if err == nil {
		return domain.AccessLevel(member.AccessLevel), nil
	}

	// Fallback: унаследованные члены
	allPath := fmt.Sprintf("/api/v4/projects/%d/members/all/%d", projectID, userID)
	if err2 := s.get(ctx, token, allPath, &member); err2 != nil {
		return 0, fmt.Errorf("get project access level: %w", err)
	}

	return domain.AccessLevel(member.AccessLevel), nil
}

// =============================================================================
// HTTP helper
// =============================================================================

func (s *GitLabAuthService) get(ctx context.Context, token, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.gitlabURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}
