package review_test

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"cr-assistant/internal/codereview"
	"cr-assistant/internal/domain"
	"cr-assistant/internal/gitlab"
	"cr-assistant/internal/llm"
	"cr-assistant/internal/report"
	"cr-assistant/internal/review"
	"cr-assistant/internal/testutil/mockserver"
)

const (
	projectID = 10
	mrIID     = 3
	jobToken  = "ci-job-token"
	botToken  = "bot-pat-token"
)

// newOrchestrator собирает оркестратор, направленный на mock-сервер.
func newOrchestrator(t *testing.T, srv *mockserver.Server) *review.Orchestrator {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	glClient := gitlab.NewClient(srv.URL, jobToken, botToken, 10*time.Second, logger)
	llmGateway := llm.NewGateway(srv.URL, 10*time.Second)
	renderer := report.NewRenderer()
	crLoader := codereview.NewLoader(srv.URL, jobToken, 10*time.Second)
	return review.NewOrchestrator(glClient, llmGateway, renderer, crLoader, logger)
}

// baseMR возвращает MR с корректными diff_refs и одним изменённым файлом.
func baseMR() mockserver.MR {
	return mockserver.MR{
		IID: mrIID,
		DiffRefs: mockserver.DiffRefs{
			BaseSHA:  "aaaaaaaa",
			StartSHA: "aaaaaaaa",
			HeadSHA:  "aaaaaaaa",
		},
		Diffs: []mockserver.FileDiff{
			{
				OldPath: "main.py",
				NewPath: "main.py",
				Diff: `@@ -1,3 +1,5 @@
+import os
+PASSWORD='secret'
def main():
    pass
`,
			},
		},
	}
}

// =============================================================================
// Tests
// =============================================================================

// TestOrchestrator_HappyPath — LLM возвращает Critical, оркестратор публикует
// inline-комментарий, summary и HTML-отчёт.
func TestOrchestrator_HappyPath(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("mock server not responding: %v", err)
	}
	resp.Body.Close()

	srv.GitLab.SetMR(projectID, baseMR())
	srv.LLM.SetComments([]mockserver.LLMComment{
		{
			FilePath:   "main.py",
			Line:       2,
			Severity:   "critical",
			Message:    "Hardcoded password detected",
			Suggestion: "Use environment variables instead",
		},
	})

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.HasCritical {
		t.Error("expected HasCritical=true")
	}
	if result.Verdict != "Требует исправления" {
		t.Errorf("verdict = %q, want %q", result.Verdict, "Требует исправления")
	}
	if result.TotalByLevel[domain.SeverityCritical] != 1 {
		t.Errorf("critical count = %d, want 1", result.TotalByLevel[domain.SeverityCritical])
	}

	comments := srv.GitLab.Comments(projectID, mrIID)
	if len(comments) == 0 {
		t.Error("expected comments to be posted to GitLab, got none")
	}
}

// TestOrchestrator_NoIssues — LLM не находит замечаний, вердикт "Пройдено".
func TestOrchestrator_NoIssues(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	srv.GitLab.SetMR(projectID, baseMR())
	srv.LLM.SetComments(nil)

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.HasCritical {
		t.Error("expected HasCritical=false")
	}
	if result.Verdict != "Пройдено" {
		t.Errorf("verdict = %q, want %q", result.Verdict, "Пройдено")
	}
}

// TestOrchestrator_MultipleFiles — несколько файлов в MR, замечания на каждый.
func TestOrchestrator_MultipleFiles(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	mr := baseMR()
	mr.Diffs = append(mr.Diffs, mockserver.FileDiff{
		OldPath: "utils.py", NewPath: "utils.py",
		Diff: "@@ -0,0 +1,5 @@\n+def unused():\n+    x = 1\n",
	})
	srv.GitLab.SetMR(projectID, mr)
	srv.LLM.SetComments([]mockserver.LLMComment{
		{FilePath: "main.py", Line: 2, Severity: "critical", Message: "Secret exposed"},
		{FilePath: "utils.py", Line: 2, Severity: "minor", Message: "Unused variable"},
	})

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := len(result.Comments); got != 2 {
		t.Errorf("comments count = %d, want 2", got)
	}
	if result.TotalByLevel[domain.SeverityMinor] != 1 {
		t.Errorf("minor count = %d, want 1", result.TotalByLevel[domain.SeverityMinor])
	}
}

// TestOrchestrator_ExcludeFiles — файлы из .codereview.yml excludes не анализируются.
func TestOrchestrator_ExcludeFiles(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	mr := baseMR()
	// Добавляем vendor файл, который должен быть исключён
	mr.Diffs = append(mr.Diffs, mockserver.FileDiff{
		OldPath: "vendor/lib/util.go",
		NewPath: "vendor/lib/util.go",
		Diff:    "@@ -0,0 +1 @@\n+package lib\n",
	})
	// Добавляем .pb.go файл
	mr.Diffs = append(mr.Diffs, mockserver.FileDiff{
		OldPath: "api/service.pb.go",
		NewPath: "api/service.pb.go",
		Diff:    "@@ -0,0 +1 @@\n+// generated\n",
	})
	srv.GitLab.SetMR(projectID, mr)

	// Конфигурируем mock чтобы .codereview.yml возвращал excludes
	srv.GitLab.SetCodeReviewConfig(`
review:
  block_on_critical: true
files:
  exclude:
    - vendor/**
    - "*.pb.go"
`)

	srv.LLM.SetComments([]mockserver.LLMComment{
		{FilePath: "main.py", Line: 1, Severity: "minor", Message: "Style issue"},
	})

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// Только main.py должен быть проанализирован
	if got := len(result.Comments); got != 1 {
		t.Errorf("comments count = %d, want 1 (vendor and pb.go should be excluded)", got)
	}
}

// TestOrchestrator_LLMUnavailable — LLM возвращает 500, оркестратор продолжает.
func TestOrchestrator_LLMUnavailable(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	srv.GitLab.SetMR(projectID, baseMR())
	srv.LLM.SetFailNext()

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() should not fail when LLM is down, got: %v", err)
	}
	if result.HasCritical {
		t.Error("expected HasCritical=false when LLM is down")
	}
}

// TestOrchestrator_EmptyDiffRefs — GitLab ещё не обработал MR → fallback notes.
func TestOrchestrator_EmptyDiffRefs(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	mr := baseMR()
	mr.DiffRefs = mockserver.DiffRefs{}
	srv.GitLab.SetMR(projectID, mr)
	srv.LLM.SetComments([]mockserver.LLMComment{
		{FilePath: "main.py", Line: 1, Severity: "major", Message: "Issue"},
	})

	orch := newOrchestrator(t, srv)
	result, err := orch.Run(context.Background(), projectID, mrIID, "")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	comments := srv.GitLab.Comments(projectID, mrIID)
	if len(comments) == 0 {
		t.Error("expected fallback notes to be posted")
	}
	_ = result
}

// TestOrchestrator_MRNotFound — MR не существует → Run возвращает ошибку.
func TestOrchestrator_MRNotFound(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	orch := newOrchestrator(t, srv)
	_, err := orch.Run(context.Background(), projectID, 999, "")

	if err == nil {
		t.Error("expected error when MR not found, got nil")
	}
}

// TestOrchestrator_SummaryAlwaysPosted — summary публикуется даже без замечаний.
func TestOrchestrator_SummaryAlwaysPosted(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()

	srv.GitLab.SetMR(projectID, baseMR())
	srv.LLM.SetComments(nil)

	orch := newOrchestrator(t, srv)
	_, err := orch.Run(context.Background(), projectID, mrIID, "")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	comments := srv.GitLab.Comments(projectID, mrIID)
	if len(comments) == 0 {
		t.Error("summary comment should always be posted")
	}

	found := false
	for _, c := range comments {
		if contains(c, "Пройдено") || contains(c, "Требует исправления") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("summary comment not found in: %v", comments)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
