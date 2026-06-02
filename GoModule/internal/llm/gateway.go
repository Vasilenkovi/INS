package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cr-assistant/internal/domain"
)

// Gateway — HTTP-клиент для Python LLM-модуля.
//
// Контракт запроса к Python-модулю:
//
//	POST /review
//	{
//	  "file_path":     "internal/controllers/user_controller.go",
//	  "code_content":  "package controllers\n...",
//	  "standard_yaml": "version: 1.0\narchitecture:\n..."
//	}
//
// Контракт ответа (YAML, декодируется в reviewResponse):
//
//	score: 6
//	issues:
//	  - line: 14
//	    severity: critical
//	    category: architecture
//	    message: "..."
//	    suggestion: "..."
type Gateway struct {
	serviceURL string
	timeout    time.Duration
	httpClient *http.Client
}

func NewGateway(serviceURL string, timeout time.Duration) *Gateway {
	return &Gateway{
		serviceURL: serviceURL,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// reviewRequest — тело запроса к Python LLM-модулю.
type reviewRequest struct {
	FilePath     string `json:"file_path"`
	CodeContent  string `json:"code_content"`
	StandardYAML string `json:"standard_yaml"`
}

// reviewResponse — ответ Python LLM-модуля (десериализуется из JSON).
// Python-модуль возвращает YAML, но мы принимаем JSON —
// оба формата поддерживаются на стороне Python (yaml.dump совместим с JSON-парсером).
type reviewResponse struct {
	Score  int           `json:"score"`
	Issues []reviewIssue `json:"issues"`
}

type reviewIssue struct {
	Line       int    `json:"line"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// Analyze отправляет diff файла в Python LLM-модуль и возвращает список замечаний.
//
// Если Python-модуль недоступен или вернул ошибку — возвращает ошибку.
// Оркестратор логирует её и пропускает файл (не прерывает весь MR-анализ).
//
// codeStandard передаётся в поле standard_yaml — это plain text или YAML
// из standards-service. Python-модуль сам интерпретирует содержимое.
func (g *Gateway) Analyze(ctx context.Context, diff domain.FileDiff, codeStandard string) ([]domain.Comment, error) {
	// Удалённые файлы не анализируем — в них нет нового кода.
	if diff.IsDelete {
		return nil, nil
	}

	// Извлекаем добавленные строки из unified diff как "code_content".
	// Python-модуль получает только добавленный код, а не весь файл —
	// это соответствует задаче ревью MR, а не всего кодовой базы.
	codeContent := extractAddedLines(diff.Diff)
	fmt.Printf("DEBUG Analyze: file=%s, diff=%q, codeContent=%q\n", diff.NewPath, diff.Diff, codeContent)

	if codeContent == "" {
		fmt.Printf("DEBUG Analyze: EMPTY codeContent for %s\n", diff.NewPath)
		return nil, nil
	}

	reqBody := reviewRequest{
		FilePath:     diff.NewPath,
		CodeContent:  codeContent,
		StandardYAML: codeStandard,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: marshal request: %w", err)
	}

	url := strings.TrimRight(g.serviceURL, "/") + "/analyze"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("llm gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("llm gateway: unexpected status %d: %s", resp.StatusCode, string(raw))
	}

	var result reviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("llm gateway: decode response: %w", err)
	}

	return mapIssues(diff.NewPath, result.Issues), nil
}

// mapIssues конвертирует ответ Python-модуля в domain.Comment.
func mapIssues(filePath string, issues []reviewIssue) []domain.Comment {
	comments := make([]domain.Comment, 0, len(issues))
	for _, issue := range issues {
		comments = append(comments, domain.Comment{
			FilePath:   filePath,
			Line:       issue.Line,
			Severity:   mapSeverity(issue.Severity),
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
		})
	}
	return comments
}

// mapSeverity конвертирует строку severity из Python-ответа в domain.Severity.
// Неизвестные значения маппятся в Minor — не теряем замечание, но не блокируем merge.
func mapSeverity(s string) domain.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return domain.SeverityCritical
	case "major":
		return domain.SeverityMajor
	default:
		return domain.SeverityMinor
	}
}

// extractAddedLines извлекает добавленные строки из unified diff.
// Строки начинающиеся с '+' (кроме '+++') — это новый код в MR.
func extractAddedLines(unifiedDiff string) string {
	var sb strings.Builder
	lines := strings.Split(unifiedDiff, "\n")
	for _, line := range lines {
		// Пропускаем заголовки diff (---, +++) и служебные строки (@@)
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "@@") {
			continue
		}
		// Добавляем только строки с '+' в начале
		if strings.HasPrefix(line, "+") {
			sb.WriteString(strings.TrimPrefix(line, "+"))
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String())
}
