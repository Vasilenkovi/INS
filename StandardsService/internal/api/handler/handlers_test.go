package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"standards-service/internal/api/handler"
	"standards-service/internal/api/middleware"
	"standards-service/internal/domain"
	"standards-service/internal/service"
	"standards-service/internal/testutil"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	handlerProjectID = 101
	handlerMaintID   = 1
	handlerDevID     = 2
)

func newChecker(auth *testutil.AuthMock) *domain.AccessChecker {
	return domain.NewAccessChecker(auth, 0, domain.AccessLevelMaintainer)
}

// newRouter собирает роутер с реальными handlers и mock-зависимостями.
func newRouter(auth *testutil.AuthMock, teams *testutil.TeamRepoMock, repos *testutil.RepositoryRepoMock) *gin.Engine {
	standardRepo := testutil.NewCodeStandardRepoMock()
	versionRepo := testutil.NewStandardVersionRepoMock()
	checker := newChecker(auth)

	teamSvc := service.NewTeamService(teams, repos, checker, auth)
	repoSvc := service.NewRepositoryService(repos, teams, checker, auth)
	standardSvc := service.NewStandardService(standardRepo, versionRepo, teams, repos, checker, auth)

	teamH := handler.NewTeamHandler(teamSvc)
	repoH := handler.NewRepositoryHandler(repoSvc)
	standardH := handler.NewStandardHandler(standardSvc)

	r := gin.New()
	api := r.Group("/api/v1", middleware.Auth(auth))
	{
		api.POST("/teams", teamH.Create)
		api.GET("/teams", teamH.List)
		api.GET("/teams/:slug", teamH.Get)
		api.PATCH("/teams/:slug", teamH.Update)
		api.DELETE("/teams/:slug", teamH.Delete)
		api.POST("/teams/:slug/repos", repoH.Add)
		api.GET("/teams/:slug/repos", repoH.List)
		api.DELETE("/teams/:slug/repos/:repo_id", repoH.Remove)
		api.POST("/teams/:slug/standards", standardH.Upload)
		api.GET("/teams/:slug/standards/active", standardH.GetActive)
		api.GET("/teams/:slug/standards/versions", standardH.ListVersions)
		api.PUT("/teams/:slug/standards/versions/:version_id/activate", standardH.Activate)
	}
	return r
}

// seedFixtures инициализирует моки пользователями и правами.
func seedFixtures(auth *testutil.AuthMock) {
	auth.SetUser("maint-token", domain.GitLabUser{ID: handlerMaintID, Username: "alice"})
	auth.SetUser("dev-token", domain.GitLabUser{ID: handlerDevID, Username: "bob"})
	auth.SetProjectAccess(handlerProjectID, handlerMaintID, 40) // alice → maintainer
	auth.SetProjectAccess(handlerProjectID, handlerDevID, 30)   // bob → developer
}

func doRequest(r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// =============================================================================
// Auth middleware
// =============================================================================

func TestHandler_NoToken_Returns401(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodGet, "/api/v1/teams", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandler_InvalidToken_Returns401(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodGet, "/api/v1/teams", "bad-token", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// =============================================================================
// Team handler
// =============================================================================

func TestHandler_CreateTeam_Maintainer(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "maint-token", map[string]any{
		"name":              "Core Backend",
		"slug":              "core-backend",
		"gitlab_project_id": handlerProjectID,
		"repo_name":         "auth-service",
		"repo_full_path":    "company/backend/auth-service",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateTeam_MissingProjectID_Returns400(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "maint-token", map[string]any{
		"name": "Core Backend",
		"slug": "core-backend",
		// gitlab_project_id отсутствует
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandler_CreateTeam_Developer_Returns403(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "dev-token", map[string]any{
		"name":              "Core Backend",
		"slug":              "core-backend",
		"gitlab_project_id": handlerProjectID,
		"repo_name":         "auth-service",
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHandler_ListTeams_AnyAuth(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodGet, "/api/v1/teams", "dev-token", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// =============================================================================
// Standard handler
// =============================================================================

func seedTeamAndRepoInMocks(auth *testutil.AuthMock, teams *testutil.TeamRepoMock, repos *testutil.RepositoryRepoMock) {
	team := &domain.Team{ID: "team-1", Name: "Core Backend", Slug: "core-backend"}
	teams.Create(context.Background(), team)
	repos.Create(context.Background(), &domain.Repository{
		ID: "repo-1", TeamID: "team-1", GitLabID: handlerProjectID, Name: "auth-service",
	})
}

func TestHandler_UploadStandard_Maintainer(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	seedTeamAndRepoInMocks(auth, teams, repos)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodPost, "/api/v1/teams/core-backend/standards", "maint-token", map[string]any{
		"custom_rules": "## Rules\n- Use gofmt\n- No unused imports",
		"comment":      "Initial standard",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadStandard_NoCustomRules_Returns400(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	seedTeamAndRepoInMocks(auth, teams, repos)
	r := newRouter(auth, teams, repos)

	w := doRequest(r, http.MethodPost, "/api/v1/teams/core-backend/standards", "maint-token", map[string]any{
		"comment": "no rules",
		// custom_rules отсутствует
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandler_GetActiveStandard_AnyAuth(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	seedTeamAndRepoInMocks(auth, teams, repos)
	r := newRouter(auth, teams, repos)

	// Сначала загружаем стандарт
	doRequest(r, http.MethodPost, "/api/v1/teams/core-backend/standards", "maint-token", map[string]any{
		"custom_rules": "## Rules",
	})

	// Developer может читать
	w := doRequest(r, http.MethodGet, "/api/v1/teams/core-backend/standards/active", "dev-token", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadStandard_AutoActivates(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	seedFixtures(auth)
	seedTeamAndRepoInMocks(auth, teams, repos)
	r := newRouter(auth, teams, repos)

	doRequest(r, http.MethodPost, "/api/v1/teams/core-backend/standards", "maint-token", map[string]any{
		"custom_rules": "## Rules v1",
	})
	doRequest(r, http.MethodPost, "/api/v1/teams/core-backend/standards", "maint-token", map[string]any{
		"custom_rules": "## Rules v2",
	})

	// Активной должна быть последняя версия
	w := doRequest(r, http.MethodGet, "/api/v1/teams/core-backend/standards/active", "dev-token", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if v, ok := resp["version"].(float64); !ok || v != 2 {
		t.Errorf("expected active version = 2, got %v", resp["version"])
	}
}
