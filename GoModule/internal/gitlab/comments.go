package gitlab

import (
	"context"
	"fmt"
	"strings"

	"cr-assistant/internal/domain"
)

// Inline comment

// PostInlineComment публикует замечание к конкретной строке diff в MR.
// Реализует domain.GitLabPort.
//
// Требует DiffRefs, полученных через GetMRDiffRefs. Если refs пустые
// (например MR только что создан) — публикует комментарий без привязки
// к строке (fallback на обычный note), чтобы не потерять замечание.
func (c *Client) PostInlineComment(ctx context.Context, projectID, mrIID int, comment domain.Comment, refs domain.DiffRefs) error {
	if refs.Empty() {
		return c.postFallbackNote(ctx, projectID, mrIID, comment)
	}

	path := fmt.Sprintf("/projects/%d/merge_requests/%d/discussions", projectID, mrIID)

	payload := inlineDiscussionPayload(comment, refs)

	if err := c.post(ctx, path, payload, nil); err != nil {
		// GitLab может вернуть 400 если строка не существует в diff
		// (например LLM вернула несуществующий номер строки).
		// Делаем fallback — публикуем как обычный note.
		c.logger.Warn("inline comment failed, falling back to note",
			"file", comment.FilePath,
			"line", comment.Line,
			"error", err,
		)
		return c.postFallbackNote(ctx, projectID, mrIID, comment)
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

// inlineDiscussionPayload собирает тело запроса для discussion с позицией.
// https://docs.gitlab.com/ee/api/discussions.html#create-new-merge-request-thread
func inlineDiscussionPayload(comment domain.Comment, refs domain.DiffRefs) map[string]any {
	return map[string]any{
		"body": formatComment(comment),
		"position": map[string]any{
			"position_type": "text",
			"base_sha":      refs.BaseSHA,
			"start_sha":     refs.StartSHA,
			"head_sha":      refs.HeadSHA,
			"new_path":      comment.FilePath,
			"new_line":      comment.Line,
		},
	}
}

// postFallbackNote публикует комментарий как обычный note без привязки к строке.
// Используется когда inline-позиционирование недоступно.
func (c *Client) postFallbackNote(ctx context.Context, projectID, mrIID int, comment domain.Comment) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, mrIID)

	// Добавляем файл и строку в текст — чтобы было понятно откуда замечание
	body := fmt.Sprintf(
		"📍 `%s` (строка %d)\n\n%s",
		comment.FilePath, comment.Line, formatComment(comment),
	)

	payload := map[string]any{"body": body}

	if err := c.post(ctx, path, payload, nil); err != nil {
		return fmt.Errorf("postFallbackNote: %w", err)
	}

	c.logger.Debug("posted fallback note",
		"project_id", projectID,
		"mr_iid", mrIID,
		"file", comment.FilePath,
		"line", comment.Line,
	)
	return nil
}

// Summary comment

// PostSummaryComment публикует итоговый комментарий-резюме к MR.
// Реализует domain.GitLabPort.
func (c *Client) PostSummaryComment(ctx context.Context, projectID, mrIID int, result domain.ReviewResult) error {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, mrIID)

	payload := map[string]any{
		"body": formatSummary(result),
	}

	if err := c.post(ctx, path, payload, nil); err != nil {
		return fmt.Errorf("PostSummaryComment: %w", err)
	}

	c.logger.Info("posted summary comment",
		"project_id", projectID,
		"mr_iid", mrIID,
		"verdict", result.Verdict,
	)
	return nil
}

// HTML Report

// PostHTMLReport публикует уведомление о HTML-отчёте как note в MR.
// Реализует domain.GitLabPort.
//
// Сам HTML сохраняется как GitLab CI Job Artifact (через artifacts: paths:
// в .gitlab-ci.yml) API для этого не нужен. Здесь публикуем только ссылку.
func (c *Client) PostHTMLReport(ctx context.Context, projectID, mrIID int, html string) error {
	// TODO: сформировать реальную ссылку на артефакт через CI_JOB_ID и CI_PROJECT_ID
	body := "📋 **HTML-отчёт** сформирован и доступен в артефактах CI job."

	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectID, mrIID)
	payload := map[string]any{"body": body}

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
	sb.WriteString(fmt.Sprintf(
		"%s **[%s]** %s\n\n",
		icon, strings.ToUpper(string(c.Severity)), c.Message,
	))

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
