// Package standardsclient — HTTP-клиент для внутреннего API standards-service.
//
// cr-assistant использует его вместо прямого подключения к PostgreSQL.
// Аутентификация — через CI_JOB_TOKEN, который GitLab автоматически
// проставляет в каждый CI job как переменную окружения CI_JOB_TOKEN.
package standardsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ActiveStandard — ответ standards-service для CI job.
type ActiveStandard struct {
	Preset      string `json:"preset"`
	CustomRules string `json:"custom_rules"`
	Language    string `json:"language"`
	Version     int    `json:"version"`
}

// BuildPromptContext формирует строку стандарта для подстановки в промпт LLM.
func (s *ActiveStandard) BuildPromptContext() string {
	if s.Preset != "" && s.CustomRules != "" {
		return fmt.Sprintf("Base preset: %s\n\nAdditional rules:\n%s", s.Preset, s.CustomRules)
	}
	if s.Preset != "" {
		return fmt.Sprintf("Base preset: %s", s.Preset)
	}
	return s.CustomRules
}

// ErrNotFound возвращается когда проект не зарегистрирован или стандарт не настроен.
// cr-assistant обрабатывает его как сигнал работать с пустым стандартом.
type ErrNotFound struct {
	ProjectID int
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("no active standard for project %d", e.ProjectID)
}

// Client — HTTP-клиент для /internal/v1/ API standards-service.
type Client struct {
	baseURL    string
	jobToken   string // CI_JOB_TOKEN — проставляется GitLab автоматически
	httpClient *http.Client
}

// New создаёт клиент.
//   - baseURL  — STANDARDS_SERVICE_URL, например http://standards-service:9090
//   - jobToken — CI_JOB_TOKEN (GitLab проставляет автоматически в каждом job)
//   - timeout  — таймаут одного HTTP-запроса
func New(baseURL, jobToken string, timeout time.Duration) *Client {
	return &Client{
		baseURL:  baseURL,
		jobToken: jobToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetStandard запрашивает активный стандарт для GitLab-проекта.
//
// Возвращает *ErrNotFound если проект не зарегистрирован или стандарт не настроен.
// В этом случае cr-assistant продолжает работу без стандарта.
func (c *Client) GetStandard(ctx context.Context, gitlabProjectID int) (*ActiveStandard, error) {
	url := fmt.Sprintf("%s/internal/v1/projects/%d/standard", c.baseURL, gitlabProjectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("standards client: build request: %w", err)
	}
	req.Header.Set("Job-Token", c.jobToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("standards client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrNotFound{ProjectID: gitlabProjectID}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("standards client: invalid CI_JOB_TOKEN")
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("standards client: unexpected status %d: %s", resp.StatusCode, string(raw))
	}

	var standard ActiveStandard
	if err := json.NewDecoder(resp.Body).Decode(&standard); err != nil {
		return nil, fmt.Errorf("standards client: decode response: %w", err)
	}

	return &standard, nil
}
