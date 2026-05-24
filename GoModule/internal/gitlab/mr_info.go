package gitlab

import (
	"context"
	"fmt"

	"cr-assistant/internal/domain"
)

// mrInfoResponse — часть ответа GitLab API на GET /merge_requests/:iid.
type mrInfoResponse struct {
	IID      int          `json:"iid"`
	DiffRefs diffRefsJSON `json:"diff_refs"`
}

// diffRefsJSON — JSON-представление diff_refs из GitLab API.
type diffRefsJSON struct {
	BaseSHA  string `json:"base_sha"`
	StartSHA string `json:"start_sha"`
	HeadSHA  string `json:"head_sha"`
}

// GetMRDiffRefs запрашивает diff_refs для указанного MR.
// Реализует domain.GitLabPort.
//
// Вызывается один раз в начале анализа; результат передаётся
// во все последующие вызовы PostInlineComment.
func (c *Client) GetMRDiffRefs(ctx context.Context, projectID, mrIID int) (domain.DiffRefs, error) {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d", projectID, mrIID)

	var resp mrInfoResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return domain.DiffRefs{}, fmt.Errorf("GetMRDiffRefs: %w", err)
	}

	refs := domain.DiffRefs{
		BaseSHA:  resp.DiffRefs.BaseSHA,
		StartSHA: resp.DiffRefs.StartSHA,
		HeadSHA:  resp.DiffRefs.HeadSHA,
	}

	if refs.Empty() {
		// MR только создан и ещё не обработан GitLab — diff_refs временно пустые.
		// Возвращаем ошибку; оркестратор перейдёт в fallback-режим (notes без позиции).
		return domain.DiffRefs{}, fmt.Errorf("GetMRDiffRefs: diff_refs empty (MR may still be processing)")
	}

	c.logger.Debug("fetched diff_refs",
		"project_id", projectID,
		"mr_iid", mrIID,
		"base_sha", refs.BaseSHA[:8],
		"head_sha", refs.HeadSHA[:8],
		"start_sha", refs.StartSHA[:8],
	)

	return refs, nil
}
