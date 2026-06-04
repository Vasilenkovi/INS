package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"standards-service/internal/service"
)

// CIHandler обслуживает внутренние запросы от cr-assistant CI job.
// Маршруты регистрируются под /internal/v1/ — отдельно от /api/v1/,
// чтобы было очевидно что это machine-to-machine интерфейс.
type CIHandler struct {
	svc *service.CIService
}

func NewCIHandler(svc *service.CIService) *CIHandler {
	return &CIHandler{svc: svc}
}

// GET /internal/v1/projects/:gitlab_project_id/standard
//
// Возвращает активный стандарт кода для GitLab-проекта.
// Аутентификация через заголовок Job-Token (CI_JOB_TOKEN из пайплайна).
//
// Response 200:
//
//	{"custom_rules":"...","version":3}
//
// Response 404: проект не зарегистрирован или стандарт не настроен.
// cr-assistant обрабатывает 404 как "работать без стандарта".
func (h *CIHandler) GetStandard(c *gin.Context) {
	projectIDStr := c.Param("gitlab_project_id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gitlab_project_id must be an integer"})
		return
	}

	token := c.GetHeader("Job-Token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Job-Token header required"})
		return
	}

	result, err := h.svc.GetStandardForProject(c.Request.Context(), projectID, token)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}
