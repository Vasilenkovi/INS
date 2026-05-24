package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client низкоуровневый HTTP-клиент для GitLab API v4.
// Использует два токена с разными правами:
//   - jobToken (CI_JOB_TOKEN) — только чтение: diff, информация о MR.
//   - botToken (CR_BOT_TOKEN) — запись: публикация комментариев и отчётов.
//
// Разделение токенов обязательно: CI_JOB_TOKEN не имеет права api
// и не может публиковать комментарии через GitLab API.
type Client struct {
	baseURL    string
	jobToken   string // CI_JOB_TOKEN — read-only операции
	botToken   string // CR_BOT_TOKEN — write операции (комментарии)
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient создаёт клиент с двумя токенами.
//   - baseURL  — CI_SERVER_URL, например http://gitlab.example.com
//   - jobToken — CI_JOB_TOKEN (GitLab проставляет автоматически)
//   - botToken — CR_BOT_TOKEN (Personal Access Token бот-пользователя, scope: api)
//   - timeout  — таймаут одного HTTP-запроса
func NewClient(baseURL, jobToken, botToken string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		jobToken: jobToken,
		botToken: botToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// get выполняет GET-запрос с jobToken (read-only).
func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil, c.jobToken)
	if err != nil {
		return err
	}
	return c.do(req, dest)
}

// post выполняет POST-запрос с botToken (write).
func (c *Client) post(ctx context.Context, path string, body any, dest any) error {
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, body, c.botToken)
	if err != nil {
		return err
	}
	return c.do(req, dest)
}

// newRequest собирает http.Request без тела под указанный токен.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, token string) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v4%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build request %s %s: %w", method, path, err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// newJSONRequest собирает http.Request с JSON-телом под указанный токен.
func (c *Client) newJSONRequest(ctx context.Context, method, path string, body any, token string) (*http.Request, error) {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(json.NewEncoder(pw).Encode(body))
	}()

	req, err := c.newRequest(ctx, method, path, pr, token)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// do выполняет запрос, проверяет статус и декодирует ответ.
func (c *Client) do(req *http.Request, dest any) error {
	c.logger.Debug("gitlab request", "method", req.Method, "url", req.URL.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("gitlab: unexpected status %d: %s", resp.StatusCode, string(raw))
	}

	if dest == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("gitlab: decode response: %w", err)
	}
	return nil
}
