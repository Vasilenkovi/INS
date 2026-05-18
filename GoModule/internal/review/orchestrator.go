package review

import (
	"context"
	"fmt"
	"log/slog"

	"cr-assistant/internal/domain"
)

// Orchestrator координирует полный цикл анализа одного MR:
// получение diff - LLM-анализ - публикация результатов.
type Orchestrator struct {
	gitlab domain.GitLabPort
	llm    domain.LLMPort
	report domain.ReportPort
	logger *slog.Logger
}

func NewOrchestrator(
	gitlab domain.GitLabPort,
	llm domain.LLMPort,
	report domain.ReportPort,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		gitlab: gitlab,
		llm:    llm,
		report: report,
		logger: logger,
	}
}

// Run запускает полный цикл анализа для указанного MR.
// Возвращает ReviewResult, по которому CLI решает exit code.
func (o *Orchestrator) Run(ctx context.Context, projectID, mrIID int) (*domain.ReviewResult, error) {
	o.logger.Info("starting review", "project_id", projectID, "mr_iid", mrIID)

	// Получаем diff_refs для inline-позиционирования комментариев.
	// Если GitLab ещё не обработал MR - продолжаем в fallback-режиме
	refs, err := o.gitlab.GetMRDiffRefs(ctx, projectID, mrIID)
	if err != nil {
		o.logger.Warn("could not fetch diff_refs, inline comments will be posted as notes",
			"error", err,
		)
		refs = domain.DiffRefs{} // Empty() == true - fallback в PostInlineComment
	}

	// Получаем diff файлов MR.
	diffs, err := o.gitlab.GetMRDiffs(ctx, projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: get diffs: %w", err)
	}
	o.logger.Info("diffs fetched", "files", len(diffs))

	// Анализируем каждый файл через LLM
	// TODO: заменить на параллельный worker pool (после реализации LLM-модуля)
	var allComments []domain.Comment
	for _, diff := range diffs {
		comments, err := o.llm.Analyze(ctx, diff, "")
		if err != nil {
			o.logger.Warn("llm analyze failed, skipping file",
				"file", diff.NewPath, "error", err)
			continue
		}
		allComments = append(allComments, comments...)
	}

	// Формируем итоговый результат
	result := buildResult(allComments)
	o.logger.Info("review completed",
		"verdict", result.Verdict,
		"critical", result.TotalByLevel[domain.SeverityCritical],
		"major", result.TotalByLevel[domain.SeverityMajor],
		"minor", result.TotalByLevel[domain.SeverityMinor],
	)

	// Публикуем inline-комментарии
	// refs передаётся в каждый вызов; если refs.Empty() - PostInlineComment
	// переключится на fallback note.
	for _, c := range allComments {
		if err := o.gitlab.PostInlineComment(ctx, projectID, mrIID, c, refs); err != nil {
			o.logger.Warn("failed to post inline comment",
				"file", c.FilePath,
				"line", c.Line,
				"error", err,
			)
		}
	}

	// Публикуем summary
	if err := o.gitlab.PostSummaryComment(ctx, projectID, mrIID, result); err != nil {
		o.logger.Warn("failed to post summary", "error", err)
	}

	// Генерируем и публикуем HTML-отчёт
	html, err := o.report.Render(result)
	if err != nil {
		o.logger.Warn("failed to render report", "error", err)
	} else {
		if err := o.gitlab.PostHTMLReport(ctx, projectID, mrIID, html); err != nil {
			o.logger.Warn("failed to post html report", "error", err)
		}
	}

	return &result, nil
}

// buildResult агрегирует комментарии в итоговый результат.
func buildResult(comments []domain.Comment) domain.ReviewResult {
	totals := map[domain.Severity]int{
		domain.SeverityCritical: 0,
		domain.SeverityMajor:    0,
		domain.SeverityMinor:    0,
	}
	for _, c := range comments {
		totals[c.Severity]++
	}

	hasCritical := totals[domain.SeverityCritical] > 0
	verdict := "Пройдено"
	if hasCritical {
		verdict = "Требует исправления"
	}

	return domain.ReviewResult{
		Comments:     comments,
		HasCritical:  hasCritical,
		TotalByLevel: totals,
		Verdict:      verdict,
	}
}
