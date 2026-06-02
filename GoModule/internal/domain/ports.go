package domain

import "context"

// DiffRefs содержит SHA-хеши для позиционирования inline-комментария.
// Все три поля возвращает GitLab API в поле diff_refs объекта MR.
type DiffRefs struct {
	BaseSHA  string // общий предок ветки MR и target-ветки
	StartSHA string // HEAD target-ветки на момент создания/обновления MR
	HeadSHA  string // последний коммит в ветке MR
}

// Empty возвращает true если хотя бы один из SHA не заполнен.
func (r DiffRefs) Empty() bool {
	return r.BaseSHA == "" || r.StartSHA == "" || r.HeadSHA == ""
}

// GitLabPort — всё, что боту нужно от GitLab.
// Реализация живёт в internal/gitlab, но зависимость объявлена здесь.
type GitLabPort interface {
	// GetMRDiffRefs возвращает SHA-хеши для inline-позиционирования комментариев.
	// Вызывается один раз в начале анализа MR.
	GetMRDiffRefs(ctx context.Context, projectID, mrIID int) (DiffRefs, error)

	// GetMRDiffs возвращает список diff-ов файлов для указанного MR.
	GetMRDiffs(ctx context.Context, projectID, mrIID int) ([]FileDiff, error)

	// PostInlineComment публикует замечание к конкретной строке diff.
	// refs — SHA-хеши из GetMRDiffRefs; если refs.Empty() — публикует
	// как обычный note без привязки к строке.
	PostInlineComment(ctx context.Context, projectID, mrIID int, comment Comment, refs DiffRefs) error

	// PostSummaryComment публикует итоговый комментарий к MR.
	PostSummaryComment(ctx context.Context, projectID, mrIID int, result ReviewResult) error

	// PostHTMLReport публикует уведомление об HTML-отчёте в MR.
	PostHTMLReport(ctx context.Context, projectID, mrIID int, html string) error
}

// LLMPort — взаимодействие с Python-модулем / LLM API.
// Пока заглушка — реализация появится вместе с Python-модулем.
type LLMPort interface {
	// Analyze отправляет diff файла и возвращает список замечаний.
	Analyze(ctx context.Context, diff FileDiff, codeStandard string) ([]Comment, error)
}

// ReportPort — генерация HTML-отчёта.
type ReportPort interface {
	Render(result ReviewResult) (string, error)
}
