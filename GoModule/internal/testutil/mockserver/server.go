// Package mockserver предоставляет общий httptest-сервер,
// эмулирующий GitLab API v4 и LLM API для тестирования cr-assistant.
package mockserver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// =============================================================================
// Top-level server
// =============================================================================

type Server struct {
	*httptest.Server
	GitLab *GitLabMock
	LLM    *LLMMock

	mu       sync.Mutex
	requests []RecordedRequest
}

type RecordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

func New() *Server {
	gl := newGitLabMock()
	llm := newLLMMock()

	s := &Server{GitLab: gl, LLM: llm}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v4/", func(w http.ResponseWriter, r *http.Request) {
		s.record(r)
		gl.ServeHTTP(w, r)
	})

	mux.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		s.record(r)
		llm.ServeHTTP(w, r)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	s.Server = httptest.NewServer(mux)
	return s
}

func (s *Server) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *Server) RequestsTo(path string) []RecordedRequest {
	var out []RecordedRequest
	for _, r := range s.Requests() {
		if strings.HasPrefix(r.Path, path) {
			out = append(out, r)
		}
	}
	return out
}

func (s *Server) record(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
	})
}

// =============================================================================
// GitLab Mock
// =============================================================================

type MR struct {
	IID      int
	DiffRefs DiffRefs
	Diffs    []FileDiff
}

type DiffRefs struct {
	BaseSHA  string
	StartSHA string
	HeadSHA  string
}

type FileDiff struct {
	OldPath string
	NewPath string
	Diff    string
	NewFile bool
}

type GitLabUser struct {
	ID       int
	Username string
	Name     string
}

type GitLabGroup struct {
	ID       int
	Name     string
	FullPath string
}

type GitLabMock struct {
	mu sync.RWMutex

	// projectID → mrIID → MR
	mrs map[int]map[int]*MR

	// token → user
	users map[string]*GitLabUser

	// projectID → userID → accessLevel
	projectAccess map[int]map[int]int

	// Записанные комментарии: projectID → mrIID → []string
	comments   map[int]map[int][]string
	commentsMu sync.Mutex

	// Содержимое .codereview.yml (по умолчанию — 404)
	codeReviewConfig string
	hasCodeReview    bool
}

func newGitLabMock() *GitLabMock {
	return &GitLabMock{
		mrs:           make(map[int]map[int]*MR),
		users:         make(map[string]*GitLabUser),
		projectAccess: make(map[int]map[int]int),
		comments:      make(map[int]map[int][]string),
	}
}

func (m *GitLabMock) SetMR(projectID int, mr MR) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mrs[projectID] == nil {
		m.mrs[projectID] = make(map[int]*MR)
	}
	m.mrs[projectID][mr.IID] = &mr
}

func (m *GitLabMock) SetUser(token string, user GitLabUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[token] = &user
}

func (m *GitLabMock) SetProjectAccess(projectID, userID, level int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.projectAccess[projectID] == nil {
		m.projectAccess[projectID] = make(map[int]int)
	}
	m.projectAccess[projectID][userID] = level
}

// SetCodeReviewConfig устанавливает содержимое .codereview.yml для всех проектов.
// Если не вызывался — mock вернёт 404 (файл отсутствует).
func (m *GitLabMock) SetCodeReviewConfig(yaml string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codeReviewConfig = yaml
	m.hasCodeReview = true
}

func (m *GitLabMock) Comments(projectID, mrIID int) []string {
	m.commentsMu.Lock()
	defer m.commentsMu.Unlock()
	return m.comments[projectID][mrIID]
}

func (m *GitLabMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v4")

	switch {
	// GET /api/v4/user
	case r.Method == http.MethodGet && path == "/user":
		token := r.Header.Get("PRIVATE-TOKEN")
		m.mu.RLock()
		user, ok := m.users[token]
		m.mu.RUnlock()
		if !ok {
			writeJSON(w, 401, map[string]string{"message": "401 Unauthorized"})
			return
		}
		writeJSON(w, 200, map[string]any{
			"id": user.ID, "username": user.Username, "name": user.Name,
		})

	// GET /api/v4/job — проверка CI_JOB_TOKEN
	case r.Method == http.MethodGet && path == "/job":
		writeJSON(w, 200, map[string]any{"id": 1})

	// GET /api/v4/projects/:id/members/:user_id и /members/all/:user_id
	case r.Method == http.MethodGet && strings.Contains(path, "/projects/") && strings.Contains(path, "/members/"):
		projectID, userID := parseTwoIDs(path, "/projects/", "/members/")
		m.mu.RLock()
		level := m.projectAccess[projectID][userID]
		m.mu.RUnlock()
		if level == 0 {
			writeJSON(w, 404, map[string]string{"message": "404 Not Found"})
			return
		}
		writeJSON(w, 200, map[string]any{"access_level": level})

	// GET /api/v4/projects/:id/repository/files/.codereview.yml
	case r.Method == http.MethodGet && strings.Contains(path, "/repository/files/"):
		m.mu.RLock()
		hasConfig := m.hasCodeReview
		content := m.codeReviewConfig
		m.mu.RUnlock()
		if !hasConfig {
			writeJSON(w, 404, map[string]string{"message": "404 File Not Found"})
			return
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		writeJSON(w, 200, map[string]any{
			"file_name": ".codereview.yml",
			"content":   encoded,
			"encoding":  "base64",
		})

	// GET /api/v4/projects/:id/merge_requests/:iid — MR info
	case r.Method == http.MethodGet && strings.Contains(path, "/merge_requests/") &&
		!strings.Contains(path, "/changes") && !strings.Contains(path, "/notes") && !strings.Contains(path, "/discussions"):
		projectID, mrIID := parseTwoIDs(path, "/projects/", "/merge_requests/")
		m.mu.RLock()
		mr := m.mrs[projectID][mrIID]
		m.mu.RUnlock()
		if mr == nil {
			writeJSON(w, 404, map[string]string{"message": "404 Not Found"})
			return
		}
		writeJSON(w, 200, map[string]any{
			"iid": mr.IID,
			"diff_refs": map[string]string{
				"base_sha":  mr.DiffRefs.BaseSHA,
				"start_sha": mr.DiffRefs.StartSHA,
				"head_sha":  mr.DiffRefs.HeadSHA,
			},
		})

	// GET /api/v4/projects/:id/merge_requests/:iid/changes — diffs
	case r.Method == http.MethodGet && strings.Contains(path, "/merge_requests/") && strings.HasSuffix(path, "/changes"):
		projectID, mrIID := parseTwoIDs(path, "/projects/", "/merge_requests/")
		m.mu.RLock()
		mr := m.mrs[projectID][mrIID]
		m.mu.RUnlock()
		if mr == nil {
			writeJSON(w, 404, map[string]string{"message": "404 Not Found"})
			return
		}
		changes := make([]map[string]any, 0, len(mr.Diffs))
		for _, d := range mr.Diffs {
			changes = append(changes, map[string]any{
				"old_path": d.OldPath, "new_path": d.NewPath,
				"diff": d.Diff, "new_file": d.NewFile, "deleted_file": false,
			})
		}
		writeJSON(w, 200, map[string]any{"changes": changes})

	// POST /api/v4/projects/:id/merge_requests/:iid/notes — summary comment
	case r.Method == http.MethodPost && strings.Contains(path, "/merge_requests/") && strings.HasSuffix(path, "/notes"):
		projectID, mrIID := parseTwoIDs(path, "/projects/", "/merge_requests/")
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request body"})
			return
		}
		m.commentsMu.Lock()
		if m.comments[projectID] == nil {
			m.comments[projectID] = make(map[int][]string)
		}
		m.comments[projectID][mrIID] = append(m.comments[projectID][mrIID], body["body"])
		m.commentsMu.Unlock()
		writeJSON(w, 201, map[string]any{"id": 1})

	// POST /api/v4/projects/:id/merge_requests/:iid/discussions — inline comment
	case r.Method == http.MethodPost && strings.Contains(path, "/merge_requests/") && strings.HasSuffix(path, "/discussions"):
		projectID, mrIID := parseTwoIDs(path, "/projects/", "/merge_requests/")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request body"})
			return
		}
		bodyText, _ := body["body"].(string)
		m.commentsMu.Lock()
		if m.comments[projectID] == nil {
			m.comments[projectID] = make(map[int][]string)
		}
		m.comments[projectID][mrIID] = append(m.comments[projectID][mrIID], bodyText)
		m.commentsMu.Unlock()
		writeJSON(w, 201, map[string]any{"id": 1})

	default:
		writeJSON(w, 404, map[string]string{"message": fmt.Sprintf("mock: no handler for %s %s", r.Method, path)})
	}
}

// =============================================================================
// LLM Mock
// =============================================================================

type LLMComment struct {
	FilePath   string
	Line       int
	Severity   string
	Message    string
	Suggestion string
}

type LLMMock struct {
	mu       sync.RWMutex
	comments []LLMComment
	failNext bool
}

func newLLMMock() *LLMMock { return &LLMMock{} }

func (m *LLMMock) SetComments(comments []LLMComment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.comments = comments
}

func (m *LLMMock) SetFailNext() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = true
}

func (m *LLMMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	m.mu.Lock()
	fail := m.failNext
	m.failNext = false
	comments := make([]LLMComment, len(m.comments))
	copy(comments, m.comments)
	m.mu.Unlock()

	var req struct {
		FilePath string `json:"file_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	if fail {
		writeJSON(w, 500, map[string]string{"error": "llm unavailable"})
		return
	}

	out := make([]map[string]any, 0)
	for _, c := range comments {
		if c.FilePath != "" && c.FilePath != req.FilePath {
			continue
		}
		out = append(out, map[string]any{
			"line": c.Line, "severity": c.Severity,
			"category": "security", "message": c.Message, "suggestion": c.Suggestion,
		})
	}

	writeJSON(w, 200, map[string]any{"score": 10, "issues": out})
}

// =============================================================================
// Helpers
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

func matchPath(path, prefix string, segments int) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	return len(parts) == segments-1
}

func parseID(path, prefix string) int {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.SplitN(rest, "/", 2)[0]
	var id int
	fmt.Sscanf(rest, "%d", &id)
	return id
}

func parseTwoIDs(path, prefix1, prefix2 string) (int, int) {
	after1 := strings.TrimPrefix(path, prefix1)
	parts := strings.SplitN(after1, prefix2, 2)
	if len(parts) != 2 {
		return 0, 0
	}
	var id1, id2 int
	fmt.Sscanf(strings.SplitN(parts[0], "/", 2)[0], "%d", &id1)
	fmt.Sscanf(strings.SplitN(parts[1], "/", 2)[0], "%d", &id2)
	return id1, id2
}

// Подавляем предупреждение о неиспользуемой функции
var _ = matchPath
var _ = parseID
