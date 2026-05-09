package gitlab

import (
	"context"
	"fmt"

	"cr-assistant/internal/domain"
)

// mrDiffResponse ответ GitLab API на запрос diff MR.
// https://docs.gitlab.com/ee/api/merge_requests.html#get-single-mr-changes
type mrDiffResponse struct {
	Changes []fileChange `json:"changes"`
}

type fileChange struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	DeletedFile bool   `json:"deleted_file"`
	RenamedFile bool   `json:"renamed_file"`
}

// GetMRDiffs возвращает список diff-ов файлов для указанного MR.
// Реализует domain.GitLabPort.
func (c *Client) GetMRDiffs(ctx context.Context, projectID, mrIID int) ([]domain.FileDiff, error) {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/changes", projectID, mrIID)

	var resp mrDiffResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("GetMRDiffs: %w", err)
	}

	diffs := make([]domain.FileDiff, 0, len(resp.Changes))
	for _, ch := range resp.Changes {
		diffs = append(diffs, domain.FileDiff{
			OldPath:  ch.OldPath,
			NewPath:  ch.NewPath,
			Diff:     ch.Diff,
			IsNew:    ch.NewFile,
			IsDelete: ch.DeletedFile,
		})
	}

	c.logger.Info("fetched MR diffs", "project_id", projectID, "mr_iid", mrIID, "files", len(diffs))
	return diffs, nil
}
