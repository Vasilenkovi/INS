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

// newRouter собирает роутер с реальными handlers и mock-зависимостями.
func newRouter(auth *testutil.AuthMock, teams *testutil.TeamRepoMock) *gin.Engine {
	standardRepo := testutil.NewCodeStandardRepoMock()
	versionRepo := testutil.NewStandardVersionRepoMock()

	teamSvc := service.NewTeamService(teams, auth)
	standardSvc := service.NewStandardService(standardRepo, versionRepo, teams, auth)

	teamH := handler.NewTeamHandler(teamSvc)
	standardH := handler.NewStandardHandler(standardSvc)

	r := gin.New()
	api := r.Group("/api/v1", middleware.Auth(auth))
	{
		api.POST("/teams", teamH.Create)
		api.GET("/teams", teamH.List)
		api.GET("/teams/:slug", teamH.Get)
		api.PATCH("/teams/:slug", teamH.Update)
		api.DELETE("/teams/:slug", teamH.Delete)
		api.POST("/teams/:slug/standards", standardH.Upload)
		api.GET("/teams/:slug/standards/active", standardH.GetActive)
		api.GET("/teams/:slug/standards/versions", standardH.ListVersions)
		api.PUT("/teams/:slug/standards/versions/:version_id/activate", standardH.Activate)
	}
	return r
}

// seedFixtures инициализирует моки пользователями и правами.
func seedFixtures(auth *testutil.AuthMock) {
	auth.SetUser("maint-token", domain.GitLabUser{ID: 1, Username: "alice"})
	auth.SetUser("dev-token", domain.GitLabUser{ID: 2, Username: "bob"})
	auth.SetGroupAccess(42, 1, 40) // alice → maintainer
	auth.SetGroupAccess(42, 2, 30) // bob → developer
	auth.SetGroup(domain.GitLabGroup{ID: 42, Name: "Backend", FullPath: "company/backend"})
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
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodGet, "/api/v1/teams", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandler_InvalidToken_Returns401(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodGet, "/api/v1/teams", "bad-token", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// =============================================================================
// Teams
// =============================================================================

func TestHandler_CreateTeam_Maintainer(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "maint-token", map[string]any{
		"name":            "Core Backend",
		"slug":            "core-backend",
		"gitlab_group_id": 42,
	})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["slug"] != "core-backend" {
		t.Errorf("slug = %v, want core-backend", resp["slug"])
	}
}

func TestHandler_CreateTeam_Developer_Returns403(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "dev-token", map[string]any{
		"name":            "Core Backend",
		"slug":            "core-backend",
		"gitlab_group_id": 42,
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHandler_CreateTeam_MissingFields_Returns400(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodPost, "/api/v1/teams", "maint-token", map[string]any{
		"name": "No Slug", // slug и gitlab_group_id отсутствуют
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandler_GetTeam_NotFound_Returns404(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodGet, "/api/v1/teams/nonexistent", "maint-token", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_ListTeams(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)

	// Создаём команду напрямую
	teams.Create(context.Background(), &domain.Team{
		ID: "1", Name: "Alpha", Slug: "alpha", GitLabGroupID: 42,
	})

	r := newRouter(auth, teams)
	w := doRequest(r, http.MethodGet, "/api/v1/teams", "dev-token", nil)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("len(list) = %d, want 1", len(list))
	}
}

// =============================================================================
// Standards
// =============================================================================

func TestHandler_UploadStandard_Maintainer(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	teams.Create(context.Background(), &domain.Team{
		ID: "t1", Name: "Backend", Slug: "backend", GitLabGroupID: 42,
	})
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodPost, "/api/v1/teams/backend/standards", "maint-token", map[string]any{
		"preset":   "PEP8",
		"language": "ru",
		"comment":  "Initial",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["version"].(float64) != 1 {
		t.Errorf("version = %v, want 1", resp["version"])
	}
}

func TestHandler_UploadStandard_Developer_Returns403(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	teams.Create(context.Background(), &domain.Team{
		ID: "t1", Name: "Backend", Slug: "backend", GitLabGroupID: 42,
	})
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodPost, "/api/v1/teams/backend/standards", "dev-token", map[string]any{
		"preset": "PEP8", "language": "ru",
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHandler_GetActiveStandard_AfterUpload(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	teams.Create(context.Background(), &domain.Team{
		ID: "t1", Name: "Backend", Slug: "backend", GitLabGroupID: 42,
	})
	r := newRouter(auth, teams)

	// Загружаем стандарт
	doRequest(r, http.MethodPost, "/api/v1/teams/backend/standards", "maint-token", map[string]any{
		"preset": "PEP8", "language": "ru",
	})

	// Читаем активный
	w := doRequest(r, http.MethodGet, "/api/v1/teams/backend/standards/active", "dev-token", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["preset"] != "PEP8" {
		t.Errorf("preset = %v, want PEP8", resp["preset"])
	}
}

func TestHandler_GetActiveStandard_NotUploaded_Returns404(t *testing.T) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	seedFixtures(auth)
	teams.Create(context.Background(), &domain.Team{
		ID: "t1", Name: "Backend", Slug: "backend", GitLabGroupID: 42,
	})
	r := newRouter(auth, teams)

	w := doRequest(r, http.MethodGet, "/api/v1/teams/backend/standards/active", "dev-token", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
