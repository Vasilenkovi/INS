// Package testutil содержит mock-реализации портов для unit-тестов.
package testutil

import (
	"context"
	"fmt"
	"sync"

	"standards-service/internal/domain"
)

// =============================================================================
// AuthMock
// =============================================================================

type AuthMock struct {
	mu       sync.RWMutex
	users    map[string]*domain.GitLabUser // token → user
	projects map[int]map[int]int           // projectID → userID → level
}

func NewAuthMock() *AuthMock {
	return &AuthMock{
		users:    make(map[string]*domain.GitLabUser),
		projects: make(map[int]map[int]int),
	}
}

func (m *AuthMock) SetUser(token string, user domain.GitLabUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[token] = &user
}

func (m *AuthMock) SetProjectAccess(projectID, userID, level int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.projects[projectID] == nil {
		m.projects[projectID] = make(map[int]int)
	}
	m.projects[projectID][userID] = level
}

func (m *AuthMock) VerifyUser(ctx context.Context, token string) (*domain.GitLabUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return u, nil
}

func (m *AuthMock) GetProjectAccessLevel(ctx context.Context, token string, projectID, userID int) (domain.AccessLevel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	level := m.projects[projectID][userID]
	if level == 0 {
		return 0, fmt.Errorf("not found")
	}
	return domain.AccessLevel(level), nil
}

func (m *AuthMock) VerifyJobToken(ctx context.Context, token string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.users[token]; !ok {
		return fmt.Errorf("invalid job token")
	}
	return nil
}

// =============================================================================
// TeamRepoMock
// =============================================================================

type TeamRepoMock struct {
	mu    sync.RWMutex
	teams map[string]*domain.Team // slug → team
	byID  map[string]*domain.Team
}

func NewTeamRepoMock() *TeamRepoMock {
	return &TeamRepoMock{
		teams: make(map[string]*domain.Team),
		byID:  make(map[string]*domain.Team),
	}
}

func (m *TeamRepoMock) Create(ctx context.Context, t *domain.Team) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.teams[t.Slug]; exists {
		return fmt.Errorf("slug %q already exists", t.Slug)
	}
	cp := *t
	m.teams[t.Slug] = &cp
	m.byID[t.ID] = &cp
	return nil
}

func (m *TeamRepoMock) GetBySlug(ctx context.Context, slug string) (*domain.Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.teams[slug]
	if !ok {
		return nil, fmt.Errorf("team %q not found", slug)
	}
	cp := *t
	return &cp, nil
}

func (m *TeamRepoMock) GetByID(ctx context.Context, id string) (*domain.Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byID[id]
	if !ok {
		return nil, fmt.Errorf("team %q not found", id)
	}
	cp := *t
	return &cp, nil
}

func (m *TeamRepoMock) List(ctx context.Context) ([]*domain.Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Team, 0, len(m.teams))
	for _, t := range m.teams {
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}

func (m *TeamRepoMock) Update(ctx context.Context, t *domain.Team) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[t.ID]; !ok {
		return fmt.Errorf("team %q not found", t.ID)
	}
	cp := *t
	m.teams[t.Slug] = &cp
	m.byID[t.ID] = &cp
	return nil
}

func (m *TeamRepoMock) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byID[id]
	if !ok {
		return fmt.Errorf("team %q not found", id)
	}
	delete(m.teams, t.Slug)
	delete(m.byID, id)
	return nil
}

// =============================================================================
// RepositoryRepoMock
// =============================================================================

type RepositoryRepoMock struct {
	mu       sync.RWMutex
	repos    map[string]*domain.Repository // id → repo
	byGitLab map[int]*domain.Repository    // gitlabID → repo
	byTeam   map[string][]*domain.Repository
}

func NewRepositoryRepoMock() *RepositoryRepoMock {
	return &RepositoryRepoMock{
		repos:    make(map[string]*domain.Repository),
		byGitLab: make(map[int]*domain.Repository),
		byTeam:   make(map[string][]*domain.Repository),
	}
}

func (m *RepositoryRepoMock) Create(ctx context.Context, r *domain.Repository) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.ID == "" {
		r.ID = fmt.Sprintf("repo-%d", r.GitLabID)
	}
	cp := *r
	m.repos[r.ID] = &cp
	m.byGitLab[r.GitLabID] = &cp
	m.byTeam[r.TeamID] = append(m.byTeam[r.TeamID], &cp)
	return nil
}

func (m *RepositoryRepoMock) GetByID(ctx context.Context, id string) (*domain.Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.repos[id]
	if !ok {
		return nil, fmt.Errorf("repository %q not found", id)
	}
	cp := *r
	return &cp, nil
}

func (m *RepositoryRepoMock) GetByGitLabID(ctx context.Context, gitlabID int) (*domain.Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.byGitLab[gitlabID]
	if !ok {
		return nil, fmt.Errorf("repository with gitlab_id %d not found", gitlabID)
	}
	cp := *r
	return &cp, nil
}

func (m *RepositoryRepoMock) ListByTeam(ctx context.Context, teamID string) ([]*domain.Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Repository, 0)
	for _, r := range m.byTeam[teamID] {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}

func (m *RepositoryRepoMock) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.repos[id]
	if !ok {
		return fmt.Errorf("repository %q not found", id)
	}
	delete(m.repos, id)
	delete(m.byGitLab, r.GitLabID)
	list := m.byTeam[r.TeamID]
	for i, repo := range list {
		if repo.ID == id {
			m.byTeam[r.TeamID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

// =============================================================================
// CodeStandardRepoMock + StandardVersionRepoMock
// =============================================================================

type CodeStandardRepoMock struct {
	mu        sync.RWMutex
	standards map[string]*domain.CodeStandard // teamID → standard
}

func NewCodeStandardRepoMock() *CodeStandardRepoMock {
	return &CodeStandardRepoMock{standards: make(map[string]*domain.CodeStandard)}
}

func (m *CodeStandardRepoMock) Upsert(ctx context.Context, s *domain.CodeStandard) (*domain.CodeStandard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.standards[s.TeamID]; ok {
		return existing, nil
	}
	if s.ID == "" {
		s.ID = "std-" + s.TeamID
	}
	cp := *s
	m.standards[s.TeamID] = &cp
	return &cp, nil
}

func (m *CodeStandardRepoMock) GetByTeamID(ctx context.Context, teamID string) (*domain.CodeStandard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.standards[teamID]
	if !ok {
		return nil, fmt.Errorf("standard for team %q not found", teamID)
	}
	cp := *s
	return &cp, nil
}

func (m *CodeStandardRepoMock) SetActiveVersion(ctx context.Context, standardID, versionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.standards {
		if s.ID == standardID {
			s.ActiveVersionID = &versionID
			return nil
		}
	}
	return fmt.Errorf("standard %q not found", standardID)
}

type StandardVersionRepoMock struct {
	mu       sync.RWMutex
	versions map[string]*domain.StandardVersion   // id → version
	byStd    map[string][]*domain.StandardVersion // standardID → versions
	active   map[string]*domain.StandardVersion   // standardID → active version
}

func NewStandardVersionRepoMock() *StandardVersionRepoMock {
	return &StandardVersionRepoMock{
		versions: make(map[string]*domain.StandardVersion),
		byStd:    make(map[string][]*domain.StandardVersion),
		active:   make(map[string]*domain.StandardVersion),
	}
}

func (m *StandardVersionRepoMock) Create(ctx context.Context, v *domain.StandardVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v.ID == "" {
		v.ID = fmt.Sprintf("ver-%d", len(m.versions)+1)
	}

	cp := *v
	m.versions[v.ID] = &cp
	m.byStd[v.CodeStandardID] = append(m.byStd[v.CodeStandardID], &cp)

	return nil
}

func (m *StandardVersionRepoMock) GetByID(ctx context.Context, id string) (*domain.StandardVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id]
	if !ok {
		return nil, fmt.Errorf("version %q not found", id)
	}
	cp := *v
	return &cp, nil
}

func (m *StandardVersionRepoMock) GetActive(ctx context.Context, standardID string) (*domain.StandardVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.active[standardID]
	if !ok {
		return nil, fmt.Errorf("no active version for standard %q", standardID)
	}
	cp := *v
	return &cp, nil
}

func (m *StandardVersionRepoMock) SetActive(standardID string, v *domain.StandardVersion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[standardID] = v
}

func (m *StandardVersionRepoMock) ListByStandard(ctx context.Context, standardID string) ([]*domain.StandardVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byStd[standardID], nil
}

func (m *StandardVersionRepoMock) GetNextVersion(ctx context.Context, standardID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.byStd[standardID]) + 1, nil
}
