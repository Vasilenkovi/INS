package review

import (
	"context"
	"fmt"
	"log/slog"

	"cr-assistant/internal/codereview"
	"cr-assistant/internal/domain"
)

// Orchestrator координирует полный цикл анализа одного MR:
// получение diff_refs → получение diff → фильтрация файлов (.codereview.yml) →
// LLM-анализ → публикация результатов.
type Orchestrator struct {
	gitlab         domain.GitLabPort
	llm            domain.LLMPort
	report         domain.ReportPort
	codeReviewLoader *codereview.Loader
	logger         *slog.Logger
}

func NewOrchestrator(
	gitlab domain.GitLabPort,
	llm domain.LLMPort,
	report domain.ReportPort,
	codeReviewLoader *codereview.Loader,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		gitlab:           gitlab,
		llm:              llm,
		report:           report,
		codeReviewLoader: codeReviewLoader,
		logger:           logger,
	}
}

// Run запускает полный цикл анализа для указанного MR.
// codeStandard — контент стандарта из standards-service; пустая строка — LLM использует дефолты.
// Возвращает ReviewResult, по которому CLI решает exit code.
func (o *Orchestrator) Run(ctx context.Context, projectID, mrIID int, codeStandard string) (*domain.ReviewResult, error) {
	o.logger.Info("starting review",
		"project_id", projectID,
		"mr_iid", mrIID,
		"has_standard", codeStandard != "",
	)

	// 1. Читаем .codereview.yml из репозитория.
	//    Если файл отсутствует — используем дефолтные настройки.
	crConfig, err := o.codeReviewLoader.Load(ctx, projectID)
	if err != nil {
		o.logger.Warn("failed to load .codereview.yml, using defaults", "error", err)
		crConfig = codereview.Defaults()
	}
	o.logger.Info("codereview config loaded",
		"excludes", crConfig.Files.Exclude,
		"block_on_critical", crConfig.Review.BlockOnCritical,
	)

	// 2. Получаем diff_refs для inline-позиционирования комментариев.
	refs, err := o.gitlab.GetMRDiffRefs(ctx, projectID, mrIID)
	if err != nil {
		o.logger.Warn("could not fetch diff_refs, inline comments will be posted as notes",
			"error", err,
		)
		refs = domain.DiffRefs{}
	}

	// 3. Получаем diff файлов MR.
	diffs, err := o.gitlab.GetMRDiffs(ctx, projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: get diffs: %w", err)
	}
	o.logger.Info("diffs fetched", "files", len(diffs))

	// 4. Фильтруем файлы по .codereview.yml excludes.
	diffs = filterDiffs(diffs, crConfig.Files.Exclude)
	o.logger.Info("diffs after filtering", "files", len(diffs))

	// 5. Анализируем каждый файл через LLM.
	var allComments []domain.Comment
	for _, diff := range diffs {
		comments, err := o.llm.Analyze(ctx, diff, codeStandard)
		if err != nil {
			o.logger.Warn("llm analyze failed, skipping file",
				"file", diff.NewPath,
				"error", err,
			)
			continue
		}
		allComments = append(allComments, comments...)
	}

	// 6. Формируем итоговый результат.
	result := buildResult(allComments)
	o.logger.Info("review completed",
		"verdict", result.Verdict,
		"critical", result.TotalByLevel[domain.SeverityCritical],
		"major", result.TotalByLevel[domain.SeverityMajor],
		"minor", result.TotalByLevel[domain.SeverityMinor],
	)

	// 7. Публикуем inline-комментарии.
	for _, c := range allComments {
		if err := o.gitlab.PostInlineComment(ctx, projectID, mrIID, c, refs); err != nil {
			o.logger.Warn("failed to post inline comment",
				"file", c.FilePath,
				"line", c.Line,
				"error", err,
			)
		}
	}

	// 8. Публикуем summary.
	if err := o.gitlab.PostSummaryComment(ctx, projectID, mrIID, result); err != nil {
		o.logger.Warn("failed to post summary", "error", err)
	}

	// 9. Генерируем и публикуем HTML-отчёт.
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

// filterDiffs исключает файлы, совпадающие с паттернами excludes из .codereview.yml.
func filterDiffs(diffs []domain.FileDiff, excludePatterns []string) []domain.FileDiff {
	if len(excludePatterns) == 0 {
		return diffs
	}

	paths := make([]string, len(diffs))
	for i, d := range diffs {
		paths[i] = d.NewPath
	}

	allowed := codereview.FilterFiles(paths, excludePatterns)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, p := range allowed {
		allowedSet[p] = struct{}{}
	}

	result := make([]domain.FileDiff, 0, len(allowed))
	for _, d := range diffs {
		if _, ok := allowedSet[d.NewPath]; ok {
			result = append(result, d)
		}
	}
	return result
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
