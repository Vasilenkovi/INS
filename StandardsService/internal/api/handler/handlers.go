package handler

import (
	"net/http"

	"standards-service/internal/api/middleware"
	"standards-service/internal/domain"
	"standards-service/internal/service"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// TeamHandler
// =============================================================================

type TeamHandler struct {
	svc *service.TeamService
}

func NewTeamHandler(svc *service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// POST /api/v1/teams
func (h *TeamHandler) Create(c *gin.Context) {
	var req struct {
		Name            string `json:"name"              binding:"required"`
		Slug            string `json:"slug"              binding:"required"`
		Description     string `json:"description"`
		GitLabProjectID int    `json:"gitlab_project_id" binding:"required"`
		RepoName        string `json:"repo_name"         binding:"required"`
		RepoFullPath    string `json:"repo_full_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.svc.Create(c.Request.Context(), &domain.Team{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	}, req.GitLabProjectID, req.RepoName, req.RepoFullPath, middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusCreated, team)
}

// GET /api/v1/teams
func (h *TeamHandler) List(c *gin.Context) {
	teams, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, teams)
}

// GET /api/v1/teams/:slug
func (h *TeamHandler) Get(c *gin.Context) {
	team, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, team)
}

// PATCH /api/v1/teams/:slug
func (h *TeamHandler) Update(c *gin.Context) {
	var req struct {
		Name        string `json:"name"        binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.svc.Update(c.Request.Context(), c.Param("slug"), req.Name, req.Description, middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, team)
}

// DELETE /api/v1/teams/:slug
func (h *TeamHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("slug"), middleware.GetToken(c)); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// =============================================================================
// RepositoryHandler
// =============================================================================

type RepositoryHandler struct {
	svc *service.RepositoryService
}

func NewRepositoryHandler(svc *service.RepositoryService) *RepositoryHandler {
	return &RepositoryHandler{svc: svc}
}

// POST /api/v1/teams/:slug/repos
func (h *RepositoryHandler) Add(c *gin.Context) {
	var req struct {
		GitLabID int    `json:"gitlab_project_id" binding:"required"`
		Name     string `json:"name"              binding:"required"`
		FullPath string `json:"full_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := h.svc.Add(c.Request.Context(), c.Param("slug"), &domain.Repository{
		GitLabID: req.GitLabID,
		Name:     req.Name,
		FullPath: req.FullPath,
	}, middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, repo)
}

// GET /api/v1/teams/:slug/repos
func (h *RepositoryHandler) List(c *gin.Context) {
	repos, err := h.svc.ListByTeam(c.Request.Context(), c.Param("slug"), middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, repos)
}

// DELETE /api/v1/teams/:slug/repos/:repo_id
func (h *RepositoryHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("slug"), c.Param("repo_id"), middleware.GetToken(c)); err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// =============================================================================
// StandardHandler
// =============================================================================

type StandardHandler struct {
	svc *service.StandardService
}

func NewStandardHandler(svc *service.StandardService) *StandardHandler {
	return &StandardHandler{svc: svc}
}

// POST /api/v1/teams/:slug/standards
func (h *StandardHandler) Upload(c *gin.Context) {
	var req struct {
		CustomRules string `json:"custom_rules" binding:"required"`
		Comment     string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	version, err := h.svc.Upload(c.Request.Context(), c.Param("slug"), &domain.StandardVersion{
		CustomRules: req.CustomRules,
		Comment:     req.Comment,
	}, middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, version)
}

// GET /api/v1/teams/:slug/standards/active
func (h *StandardHandler) GetActive(c *gin.Context) {
	version, err := h.svc.GetActive(c.Request.Context(), c.Param("slug"), middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, version)
}

// GET /api/v1/teams/:slug/standards/versions
func (h *StandardHandler) ListVersions(c *gin.Context) {
	versions, err := h.svc.ListVersions(c.Request.Context(), c.Param("slug"), middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, versions)
}

// PUT /api/v1/teams/:slug/standards/versions/:version_id/activate
func (h *StandardHandler) Activate(c *gin.Context) {
	err := h.svc.SetActiveVersion(c.Request.Context(), c.Param("slug"), c.Param("version_id"), middleware.GetToken(c))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// =============================================================================
// helpers
// =============================================================================

func respondErr(c *gin.Context, err error) {
	msg := err.Error()
	switch {
	case contains(msg, "unauthorized", "invalid token"):
		c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
	case contains(msg, "forbidden"):
		c.JSON(http.StatusForbidden, gin.H{"error": msg})
	case contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
	case contains(msg, "validation"):
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
