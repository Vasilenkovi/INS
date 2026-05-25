package middleware

import (
	"net/http"
	"strings"

	"standards-service/internal/domain"

	"github.com/gin-gonic/gin"
)

const (
	keyUser  = "gitlab_user"
	keyToken = "gitlab_token"
)

// Auth проверяет Bearer токен через GitLab API и кладёт пользователя в контекст.
// Все последующие проверки прав делаются в сервисном слое.
func Auth(authPort domain.AuthPort) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		user, err := authPort.VerifyUser(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(keyUser, user)
		c.Set(keyToken, token)
		c.Next()
	}
}

// GetUser извлекает аутентифицированного пользователя из контекста Gin.
func GetUser(c *gin.Context) *domain.GitLabUser {
	v, _ := c.Get(keyUser)
	user, _ := v.(*domain.GitLabUser)
	return user
}

// GetToken извлекает токен из контекста Gin.
func GetToken(c *gin.Context) string {
	return c.GetString(keyToken)
}
