package service_test

import (
	"context"
	"testing"

	"standards-service/internal/domain"
	"standards-service/internal/service"
	"standards-service/internal/testutil"
)

func setupStandardSvc() (
	*service.StandardService,
	*testutil.TeamRepoMock,
	*testutil.RepositoryRepoMock,
	*testutil.CodeStandardRepoMock,
	*testutil.StandardVersionRepoMock,
	*testutil.AuthMock,
) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	repos := testutil.NewRepositoryRepoMock()
	standards := testutil.NewCodeStandardRepoMock()
	versions := testutil.NewStandardVersionRepoMock()
	checker := newChecker(auth)
	svc := service.NewStandardService(standards, versions, teams, repos, checker, auth)
	return svc, teams, repos, standards, versions, auth
}

// seedTeamWithRepo создаёт команду с репозиторием напрямую в репо (без проверки прав).
func seedTeamWithRepo(teams *testutil.TeamRepoMock, repos *testutil.RepositoryRepoMock) *domain.Team {
	t := &domain.Team{
		ID:   "team-uuid-1",
		Name: "Core Backend",
		Slug: "core-backend",
	}
	teams.Create(context.Background(), t)
	repos.Create(context.Background(), &domain.Repository{
		ID:       "repo-1",
		TeamID:   t.ID,
		GitLabID: projectID,
		Name:     "auth-service",
	})
	return t
}

func TestStandardService_Upload_CreatesVersionOne(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	version, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules\n- Use gofmt",
		Comment:     "Initial standard",
	}, maintainerToken)

	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if version.Version != 1 {
		t.Errorf("version = %d, want 1", version.Version)
	}
	if version.CreatedBy != "alice" {
		t.Errorf("CreatedBy = %q, want %q", version.CreatedBy, "alice")
	}
	if version.ID == "" {
		t.Error("version.ID should be set")
	}
}

func TestStandardService_Upload_IncrementsVersion(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	_, _ = svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules v1",
	}, maintainerToken)

	v2, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules v2",
		Comment:     "Added custom rules",
	}, maintainerToken)

	if err != nil {
		t.Fatalf("second Upload() error = %v", err)
	}
	if v2.Version != 2 {
		t.Errorf("version = %d, want 2", v2.Version)
	}
}

func TestStandardService_Upload_DeveloperForbidden(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	_, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules",
	}, developerToken)

	if err == nil {
		t.Error("expected forbidden error for developer")
	}
}

func TestStandardService_Upload_RequiresCustomRules(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	_, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		// CustomRules не задан
	}, maintainerToken)

	if err == nil {
		t.Error("expected validation error when custom_rules is empty")
	}
}

func TestStandardService_Upload_AutoActivates(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	uploaded, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules",
	}, maintainerToken)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Сразу после загрузки — новая версия должна быть активной
	active, err := svc.GetActive(context.Background(), "core-backend", developerToken)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.ID != uploaded.ID {
		t.Errorf("active version ID = %q, want %q", active.ID, uploaded.ID)
	}
}

func TestStandardService_GetActive_AfterUpload(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	uploaded, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules",
	}, maintainerToken)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	active, err := svc.GetActive(context.Background(), "core-backend", developerToken)
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if active.ID != uploaded.ID {
		t.Errorf("active version ID = %q, want %q", active.ID, uploaded.ID)
	}
}

func TestStandardService_GetActive_AllAuthorizedCanRead(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	_, _ = svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules",
	}, maintainerToken)

	// Developer может читать (MIN_READ_ACCESS_LEVEL=0)
	_, err := svc.GetActive(context.Background(), "core-backend", developerToken)
	if err != nil {
		t.Errorf("developer should be able to read active standard, got: %v", err)
	}
}

func TestStandardService_SetActiveVersion_Maintainer(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	v1, _ := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules v1",
	}, maintainerToken)
	v2, _ := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## Rules v2",
	}, maintainerToken)

	// Откатываемся на v1
	err := svc.SetActiveVersion(context.Background(), "core-backend", v1.ID, maintainerToken)
	if err != nil {
		t.Fatalf("SetActiveVersion() error = %v", err)
	}

	active, _ := svc.GetActive(context.Background(), "core-backend", developerToken)
	if active.ID != v1.ID {
		t.Errorf("active = %q, want v1 = %q (v2 = %q)", active.ID, v1.ID, v2.ID)
	}
}

func TestStandardService_ListVersions_AnyAuthUser(t *testing.T) {
	svc, teams, repos, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeamWithRepo(teams, repos)

	svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{CustomRules: "## v1"}, maintainerToken)
	svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{CustomRules: "## v2"}, maintainerToken)

	// Developer может получить список версий
	versions, err := svc.ListVersions(context.Background(), "core-backend", developerToken)
	if err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("len(versions) = %d, want 2", len(versions))
	}
}
