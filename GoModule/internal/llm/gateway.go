package llm

import (
	"context"
	"cr-assistant/internal/domain"
)

// Gateway заглушка взаимодействия с Python LLM модулем.
// Будет реализована после того, как Python-модуль будет готов.
type Gateway struct{}

func NewGateway() *Gateway {
	return &Gateway{}
}

// Analyze заглушка. Всегда возвращает пустой список замечаний.
func (g *Gateway) Analyze(_ context.Context, _ domain.FileDiff, _ string) ([]domain.Comment, error) {
	// TODO: реализовать HTTP-вызов Python LLM-модуля
	return []domain.Comment{}, nil
}
