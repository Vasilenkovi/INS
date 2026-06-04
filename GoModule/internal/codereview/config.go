// Package codereview реализует чтение и применение .codereview.yml.
//
// Файл хранится в корне репозитория и читается через GitLab API
// с аутентификацией по CI_JOB_TOKEN.
//
// Структура файла:
//
//	review:
//	  block_on_critical: true
//
//	files:
//	  exclude:
//	    - vendor/**
//	    - migrations/**
//	    - "*.pb.go"
package codereview

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config — содержимое .codereview.yml.
// Структура расширяема: новые секции добавляются без изменений БД.
type Config struct {
	Review ReviewConfig `yaml:"review"`
	Files  FilesConfig  `yaml:"files"`
}

type ReviewConfig struct {
	BlockOnCritical bool `yaml:"block_on_critical"`
}

type FilesConfig struct {
	Exclude []string `yaml:"exclude"`
}

// Defaults возвращает конфигурацию по умолчанию (если файл отсутствует).
func Defaults() *Config {
	return &Config{
		Review: ReviewConfig{
			BlockOnCritical: true,
		},
		Files: FilesConfig{
			Exclude: nil,
		},
	}
}

// Loader читает .codereview.yml из GitLab репозитория через API.
type Loader struct {
	gitlabURL  string
	jobToken   string
	httpClient *http.Client
}

func NewLoader(gitlabURL, jobToken string, timeout time.Duration) *Loader {
	return &Loader{
		gitlabURL: gitlabURL,
		jobToken:  jobToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Load загружает .codereview.yml для указанного проекта.
// Если файл отсутствует (404) — возвращает значения по умолчанию без ошибки.
func (l *Loader) Load(ctx context.Context, projectID int) (*Config, error) {
	// GET /projects/:id/repository/files/.codereview.yml?ref=HEAD
	url := fmt.Sprintf(
		"%s/api/v4/projects/%d/repository/files/.codereview.yml?ref=HEAD",
		l.gitlabURL, projectID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("codereview: build request: %w", err)
	}
	req.Header.Set("JOB-TOKEN", l.jobToken)

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codereview: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Файл отсутствует — используем дефолты
		return Defaults(), nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codereview: unexpected status %d", resp.StatusCode)
	}

	// GitLab возвращает base64-encoded содержимое файла
	var fileResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return nil, fmt.Errorf("codereview: decode response: %w", err)
	}

	var raw []byte
	if fileResp.Encoding == "base64" {
		raw, err = base64.StdEncoding.DecodeString(
			strings.ReplaceAll(fileResp.Content, "\n", ""),
		)
		if err != nil {
			return nil, fmt.Errorf("codereview: decode base64: %w", err)
		}
	} else {
		raw = []byte(fileResp.Content)
	}

	cfg := Defaults()
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("codereview: parse yaml: %w", err)
	}

	return cfg, nil
}

// =============================================================================
// Фильтрация файлов
// =============================================================================

// FilterFiles исключает из списка файлы, совпадающие с паттернами excludes.
// Паттерны — glob (path.Match семантика), поддерживает ** как любую вложенность.
func FilterFiles(files []string, excludePatterns []string) []string {
	if len(excludePatterns) == 0 {
		return files
	}

	result := make([]string, 0, len(files))
	for _, f := range files {
		if !matchesAny(f, excludePatterns) {
			result = append(result, f)
		}
	}
	return result
}

// matchesAny возвращает true если filePath совпадает хотя бы с одним паттерном.
func matchesAny(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, filePath) {
			return true
		}
	}
	return false
}

// matchGlob — расширенный glob с поддержкой ** (любая вложенность директорий).
// Для паттернов без ** используется стандартный path.Match.
func matchGlob(pattern, name string) bool {
	// Паттерн с ** — заменяем на эквивалентный обход
	if strings.Contains(pattern, "**") {
		return matchDoubleGlob(pattern, name)
	}
	ok, _ := path.Match(pattern, name)
	return ok
}

// matchDoubleGlob обрабатывает паттерны вида "vendor/**" или "**/generated/**".
func matchDoubleGlob(pattern, name string) bool {
	// Разбиваем паттерн по ** и проверяем что имя совпадает с каждой частью
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := parts[1]

	// Если есть prefix — имя должно начинаться с него
	if prefix != "" {
		if !strings.HasPrefix(name, prefix) {
			return false
		}
		name = name[len(prefix):]
	}

	// Если suffix пустой — любой хвост подходит
	if suffix == "" || suffix == "/" {
		return true
	}

	// Убираем ведущий /
	suffix = strings.TrimPrefix(suffix, "/")

	// Суффикс может совпасть с именем или любой его частью после /
	if ok, _ := path.Match(suffix, name); ok {
		return true
	}
	// Или суффикс является концом пути
	if strings.HasSuffix(name, "/"+suffix) {
		return true
	}
	// Или суффикс — паттерн для basename
	basename := path.Base(name)
	if ok, _ := path.Match(suffix, basename); ok {
		return true
	}
	return false
}
