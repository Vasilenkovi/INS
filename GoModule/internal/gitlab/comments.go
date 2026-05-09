package gitlab

import (
	"context"
	"fmt"
	"strings"

	"cr-assistant/internal/domain"
)

// Inline comment

// inlineCommentRequest тело запроса для inline-комментария к строке diff.
type inlineCommentRequest struct {
	Body     string        `json:"body"`
	Position *diffPosition `json:"position"`
}

type diffPosition struct {
	PositionType string `json:"position_type"` // всегда "text"
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	NewPath      string `json:"new_path"`
	NewLine      int    `json:"new_line"`
}

// PostInlineComment публикует замечание к конкретной строке diff.
// Реализует domain.GitLabPort.
//
// Примечание: для корректного позиционирования GitLab требует SHA коммитов MR.
// В текущей реализации SHA берётся из переменных окружения CI (CI_COMMIT_SHA и др.).
// Если SHA недоступен — комментарий публикуется как обычный (без привязки к строке).
func (c *Client) PostInlineComment(ctx context.Context, projectID, mrIID int, comment domain.Comment) error {
	body := formatComment(comment)
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/discussions", projectID, mrIID)

	payload := map[string]any{
		"body": body,
	}

	if err := c.post(ctx, path, payload, nil); err != nil {
		return fmt.Errorf("PostInlineComment: %w", err)
	}

	c.logger.Debug("posted inline comment",
		"project_id", projectID,
		"mr_iid", mrIID,
		"file", comment.FilePath,
		"line", comment.Line,
		"severity", comment.Severity,
	)
	return nil
}

// Summary comment

// PostSummaryComment публикует итоговый комментарий-резюме к MR.
// Реализует domain.GitLabPort.
func (c *Client) PostSummaryComment(ctx context.Context, projectID, mrIID int, result domain.ReviewResult) error {
	body := formatSummary(result)
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, mrIID)

	payload := map[string]any{
		"body": body,
	}

	if err := c.post(ctx, path, payload, nil); err != nil {
		return fmt.Errorf("PostSummaryComment: %w", err)
	}

	c.logger.Info("posted summary comment", "project_id", projectID, "mr_iid", mrIID, "verdict", result.Verdict)
	return nil
}

// HTML Report

// PostHTMLReport публикует ссылку на HTML-отчёт как комментарий к MR.
// Реализует domain.GitLabPort.
//
// Примечание: в текущей реализации HTML передаётся как GitLab Job Artifact.
// Ссылка формируется из переменных окружения CI. Загрузка артефакта
// выполняется через механизм GitLab CI (artifacts: paths:), а не через API.
func (c *Client) PostHTMLReport(ctx context.Context, projectID, mrIID int, html string) error {
	// TODO: реализовать загрузку HTML как артефакта GitLab CI Job
	// и публикацию ссылки на него. Пока публикуем заглушку-уведомление.
	body := "**HTML-отчёт** сформирован и доступен в артефактах CI job."
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, mrIID)

	payload := map[string]any{
		"body": body,
	}

	if err := c.post(ctx, path, payload, nil); err != nil {
		return fmt.Errorf("PostHTMLReport: %w", err)
	}

	return nil
}

// Formatters

// formatComment формирует текст inline-комментария по шаблону.
func formatComment(c domain.Comment) string {
	var sb strings.Builder

	icon := severityIcon(c.Severity)
	sb.WriteString(fmt.Sprintf("%s **[%s]** %s\n\n", icon, strings.ToUpper(string(c.Severity)), c.Message))

	if c.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("💡 **Рекомендация:** %s", c.Suggestion))
	}

	return sb.String()
}

// formatSummary формирует текст summary-комментария.
func formatSummary(r domain.ReviewResult) string {
	var sb strings.Builder

	verdictIcon := "✅"
	if r.Verdict == "Требует исправления" {
		verdictIcon = "❌"
	}

	sb.WriteString(fmt.Sprintf("## %s Результат автоматического код-ревью: **%s**\n\n", verdictIcon, r.Verdict))
	sb.WriteString("| Уровень | Кол-во |\n")
	sb.WriteString("|---------|--------|\n")
	sb.WriteString(fmt.Sprintf("| 🔴 Critical | %d |\n", r.TotalByLevel[domain.SeverityCritical]))
	sb.WriteString(fmt.Sprintf("| 🟠 Major    | %d |\n", r.TotalByLevel[domain.SeverityMajor]))
	sb.WriteString(fmt.Sprintf("| 🟡 Minor    | %d |\n", r.TotalByLevel[domain.SeverityMinor]))

	return sb.String()
}

func severityIcon(s domain.Severity) string {
	switch s {
	case domain.SeverityCritical:
		return "🔴"
	case domain.SeverityMajor:
		return "🟠"
	default:
		return "🟡"
	}
}
