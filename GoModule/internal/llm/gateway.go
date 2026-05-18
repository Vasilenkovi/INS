package llm

import (
	"context"
	"time"

	"cr-assistant/internal/domain"
)

// Gateway заглушка взаимодействия с Python LLM модулем.
// Будет реализована после того, как Python-модуль будет готов.
type Gateway struct {
	serviceURL string
	timeout    time.Duration
}

func NewGateway(serviceURL string, timeout time.Duration) *Gateway {
	return &Gateway{
		serviceURL: serviceURL,
		timeout:    timeout,
	}
}

// Analyze заглушка. Всегда возвращает пустой список замечаний.
func (g *Gateway) Analyze(_ context.Context, _ domain.FileDiff, _ string) ([]domain.Comment, error) {
	// TODO: реализовать HTTP-вызов Python LLM-модуля
	return []domain.Comment{}, nil
}
