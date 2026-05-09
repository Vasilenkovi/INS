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
// Все публичные методы бота используют интерфейс domain.GitLabPort,
// реализованный в adapter.go. Client отвечает только за транспорт.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient создаёт клиент.
// baseURL значение CI_SERVER_URL (например https://gitlab.example.com).
// token   CI_JOB_TOKEN или Project Access Token.
// timeout таймаут одного HTTP-запроса.
func NewClient(baseURL, token string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// get выполняет GET-запрос к GitLab API и декодирует JSON-ответ в dest.
func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.do(req, dest)
}

// post выполняет POST-запрос с JSON-телом.
func (c *Client) post(ctx context.Context, path string, body any, dest any) error {
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.do(req, dest)
}

// newRequest собирает http.Request без тела.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s/api/v4%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build request %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// newJSONRequest собирает http.Request с JSON-телом.
func (c *Client) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(json.NewEncoder(pw).Encode(body))
	}()

	req, err := c.newRequest(ctx, method, path, pr)
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
