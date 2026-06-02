package service_test

import (
	"context"
	"testing"

	"standards-service/internal/domain"
	"standards-service/internal/service"
	"standards-service/internal/testutil"
)

func setupStandardSvc() (*service.StandardService, *testutil.TeamRepoMock, *testutil.CodeStandardRepoMock, *testutil.StandardVersionRepoMock, *testutil.AuthMock) {
	auth := testutil.NewAuthMock()
	teams := testutil.NewTeamRepoMock()
	standards := testutil.NewCodeStandardRepoMock()
	versions := testutil.NewStandardVersionRepoMock()
	svc := service.NewStandardService(standards, versions, teams, auth)
	return svc, teams, standards, versions, auth
}

// seedTeam создаёт команду напрямую в репо (без проверки прав).
func seedTeam(teams *testutil.TeamRepoMock) *domain.Team {
	t := &domain.Team{
		ID:            "team-uuid-1",
		Name:          "Core Backend",
		Slug:          "core-backend",
		GitLabGroupID: groupID,
	}
	teams.Create(context.Background(), t)
	return t
}

func TestStandardService_Upload_CreatesVersionOne(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	version, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Preset:   "PEP8",
		Language: "ru",
		Comment:  "Initial standard",
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
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	_, _ = svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Preset: "PEP8", Language: "ru",
	}, maintainerToken)

	v2, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "## New rules", Language: "ru", Comment: "Added custom rules",
	}, maintainerToken)

	if err != nil {
		t.Fatalf("second Upload() error = %v", err)
	}
	if v2.Version != 2 {
		t.Errorf("version = %d, want 2", v2.Version)
	}
}

func TestStandardService_Upload_DeveloperForbidden(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	_, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Preset: "PEP8", Language: "ru",
	}, developerToken)

	if err == nil {
		t.Error("expected forbidden error for developer")
	}
}

func TestStandardService_Upload_RequiresPresetOrCustomRules(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	_, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Language: "ru",
		// Ни Preset, ни CustomRules не заданы
	}, maintainerToken)

	if err == nil {
		t.Error("expected validation error when both preset and custom_rules are empty")
	}
}

func TestStandardService_GetActive_AfterUpload(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	uploaded, err := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Preset: "PEP8", Language: "ru",
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

func TestStandardService_SetActiveVersion_Maintainer(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	v1, _ := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		Preset: "PEP8", Language: "ru",
	}, maintainerToken)
	v2, _ := svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{
		CustomRules: "new rules", Language: "ru",
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

func TestStandardService_ListVersions_Developer(t *testing.T) {
	svc, teams, _, _, auth := setupStandardSvc()
	seedAuth(auth)
	seedTeam(teams)

	svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{Preset: "PEP8", Language: "ru"}, maintainerToken)
	svc.Upload(context.Background(), "core-backend", &domain.StandardVersion{CustomRules: "rules", Language: "ru"}, maintainerToken)

	versions, err := svc.ListVersions(context.Background(), "core-backend", developerToken)
	if err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("len(versions) = %d, want 2", len(versions))
	}
}
