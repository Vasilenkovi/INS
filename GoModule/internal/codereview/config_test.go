package codereview_test

import (
	"testing"

	"cr-assistant/internal/codereview"
)

func TestFilterFiles_NoPatterns(t *testing.T) {
	files := []string{"main.go", "vendor/lib/lib.go", "migrations/001.sql"}
	result := codereview.FilterFiles(files, nil)
	if len(result) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(result))
	}
}

func TestFilterFiles_VendorGlob(t *testing.T) {
	files := []string{
		"main.go",
		"internal/service/team.go",
		"vendor/github.com/gin-gonic/gin/gin.go",
		"vendor/golang.org/x/net/http2/h2.go",
	}
	result := codereview.FilterFiles(files, []string{"vendor/**"})
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(result), result)
	}
	for _, f := range result {
		if f == "vendor/github.com/gin-gonic/gin/gin.go" || f == "vendor/golang.org/x/net/http2/h2.go" {
			t.Errorf("vendor file should be excluded: %s", f)
		}
	}
}

func TestFilterFiles_MigrationsGlob(t *testing.T) {
	files := []string{
		"main.go",
		"migrations/001_init.up.sql",
		"migrations/002_add_column.down.sql",
		"internal/db/migrate.go",
	}
	result := codereview.FilterFiles(files, []string{"migrations/**"})
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(result), result)
	}
}

func TestFilterFiles_ExtensionGlob(t *testing.T) {
	files := []string{
		"api/v1/service.pb.go",
		"api/v1/service.go",
		"proto/service.proto",
		"internal/handler.pb.go",
	}
	result := codereview.FilterFiles(files, []string{"*.pb.go"})
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(result), result)
	}
}

func TestFilterFiles_MultiplePatterns(t *testing.T) {
	files := []string{
		"main.go",
		"vendor/lib/lib.go",
		"migrations/001.sql",
		"api/v1/svc.pb.go",
		"internal/service.go",
	}
	result := codereview.FilterFiles(files, []string{"vendor/**", "migrations/**", "*.pb.go"})
	if len(result) != 2 {
		t.Errorf("expected 2 files (main.go, internal/service.go), got %d: %v", len(result), result)
	}
}

func TestDefaults(t *testing.T) {
	cfg := codereview.Defaults()
	if cfg == nil {
		t.Fatal("Defaults() should not return nil")
	}
	if !cfg.Review.BlockOnCritical {
		t.Error("default BlockOnCritical should be true")
	}
	if len(cfg.Files.Exclude) != 0 {
		t.Errorf("default excludes should be empty, got %v", cfg.Files.Exclude)
	}
}
