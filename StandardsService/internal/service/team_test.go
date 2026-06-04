package service_test

import (
	"context"
	"testing"

	"standards-service/internal/domain"
	"standards-service/internal/service"
	"standards-service/internal/testutil"
)

const (
	maintainerToken = "maintainer-token"
	developerToken  = "developer-token"
	invalidToken    = "invalid-token"
	projectID       = 101
	userMaintID     = 1
	userDevID       = 2
)

func newChecker(auth *testutil.AuthMock) *domain.AccessChecker {
	return domain.NewAccessChecker(auth, 0, domain.AccessLevelMaintainer)
}

func setupTeamSvc() (*service.TeamService, *testutil.TeamRepoMock, *testutil.RepositoryRepoMock, *testutil.AuthMock) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	checker := newChecker(auth)
	svc := service.NewTeamService(teams, repos, checker, auth)
	return svc, teams, repos, auth
}

func seedAuth(auth *testutil.AuthMock) {
	auth.SetUser(maintainerToken, domain.GitLabUser{ID: userMaintID, Username: "alice"})
	auth.SetUser(developerToken, domain.GitLabUser{ID: userDevID, Username: "bob"})
	auth.SetProjectAccess(projectID, userMaintID, 40) // Maintainer
	auth.SetProjectAccess(projectID, userDevID, 30)   // Developer
}

func TestTeamService_Create_Maintainer(t *testing.T) {
	svc, _, repos, auth := setupTeamSvc()
	seedAuth(auth)

	team, err := svc.Create(context.Background(), &domain.Team{
		Name: "Core Backend",
		Slug: "core-backend",
	}, projectID, "auth-service", "company/backend/auth-service", maintainerToken)

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if team.ID == "" {
		t.Error("team.ID should be set after Create")
	}
	if team.CreatedBy != "alice" {
		t.Errorf("CreatedBy = %q, want %q", team.CreatedBy, "alice")
	}

	// Проверяем что первый репозиторий добавлен автоматически
	repoList, err := repos.ListByTeam(context.Background(), team.ID)
	if err != nil || len(repoList) != 1 {
		t.Errorf("expected 1 repo added automatically, got %d", len(repoList))
	}
	if repoList[0].GitLabID != projectID {
		t.Errorf("first repo GitLabID = %d, want %d", repoList[0].GitLabID, projectID)
	}
}

func TestTeamService_Create_DeveloperForbidden(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, err := svc.Create(context.Background(), &domain.Team{
		Name: "Core Backend",
		Slug: "core-backend",
	}, projectID, "auth-service", "", developerToken)

	if err == nil {
		t.Error("expected forbidden error for developer, got nil")
	}
}

func TestTeamService_Create_InvalidToken(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, err := svc.Create(context.Background(), &domain.Team{
		Name: "Core Backend",
		Slug: "core-backend",
	}, projectID, "auth-service", "", invalidToken)

	if err == nil {
		t.Error("expected unauthorized error, got nil")
	}
}

func TestTeamService_Create_MissingSlug(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, err := svc.Create(context.Background(), &domain.Team{
		Name: "Core Backend",
		// Slug не задан
	}, projectID, "auth-service", "", maintainerToken)

	if err == nil {
		t.Error("expected validation error for missing slug, got nil")
	}
}

func TestTeamService_GetBySlug(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, err := svc.Create(context.Background(), &domain.Team{
		Name: "Core Backend",
		Slug: "core-backend",
	}, projectID, "auth-service", "", maintainerToken)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	team, err := svc.GetBySlug(context.Background(), "core-backend")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}
	if team.Slug != "core-backend" {
		t.Errorf("slug = %q, want %q", team.Slug, "core-backend")
	}
}

func TestTeamService_Delete_Maintainer(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, err := svc.Create(context.Background(), &domain.Team{
		Name: "To Delete", Slug: "to-delete",
	}, projectID, "auth-service", "", maintainerToken)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(context.Background(), "to-delete", maintainerToken); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	_, err = svc.GetBySlug(context.Background(), "to-delete")
	if err == nil {
		t.Error("expected team to be deleted")
	}
}

func TestTeamService_Delete_DeveloperForbidden(t *testing.T) {
	svc, _, _, auth := setupTeamSvc()
	seedAuth(auth)

	_, _ = svc.Create(context.Background(), &domain.Team{
		Name: "Protected", Slug: "protected",
	}, projectID, "auth-service", "", maintainerToken)

	err := svc.Delete(context.Background(), "protected", developerToken)
	if err == nil {
		t.Error("expected forbidden error for developer trying to delete")
	}
}
