package report

import (
	"cr-assistant/internal/domain"
)

// Renderer — заглушка генератора HTML-отчёта.
type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render — заглушка. Возвращает минимальный HTML.
func (r *Renderer) Render(result domain.ReviewResult) (string, error) {
	// TODO: реализовать полноценный HTML-отчёт с группировкой по критичности
	return "<html><body><p>Отчёт будет реализован.</p></body></html>", nil
}
