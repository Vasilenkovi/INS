package domain

import "context"

// GitLabPort всё, что боту нужно от GitLab.
// Реализация живёт в internal/gitlab, но зависимость объявлена здесь.
type GitLabPort interface {
	// GetMRDiffs возвращает список diff-ов файлов для указанного MR.
	GetMRDiffs(ctx context.Context, projectID, mrIID int) ([]FileDiff, error)

	// PostInlineComment публикует замечание к конкретной строке diff.
	PostInlineComment(ctx context.Context, projectID, mrIID int, comment Comment) error

	// PostSummaryComment публикует итоговый комментарий к MR.
	PostSummaryComment(ctx context.Context, projectID, mrIID int, result ReviewResult) error

	// PostHTMLReport загружает HTML-отчёт как артефакт и публикует ссылку в MR.
	PostHTMLReport(ctx context.Context, projectID, mrIID int, html string) error
}

// LLMPort взаимодействие с Python-модулем / LLM API.
// Пока заглушка реализация появится вместе с Python-модулем.
type LLMPort interface {
	// Analyze отправляет diff файла и возвращает список замечаний.
	Analyze(ctx context.Context, diff FileDiff, codeStandard string) ([]Comment, error)
}

// ReportPort генерация HTML-отчёта.
type ReportPort interface {
	Render(result ReviewResult) (string, error)
}
