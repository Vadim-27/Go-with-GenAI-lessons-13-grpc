package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"libratrack/internal/auth"
	"libratrack/internal/model"
)

// Ключі, під якими claims токена лежать у контексті Gin.
const (
	CtxUserID = "user_id"
	CtxRole   = "role"
)

// bearerPrefix — схема заголовка Authorization.
const bearerPrefix = "Bearer "

// AuthMiddleware автентифікує запит за заголовком "Authorization: Bearer <token>".
//
// Перевіряє підпис та exp токена; у разі успіху кладе user_id і role в контекст
// (c.Set) для наступних middleware та хендлерів. У разі помилки відповідає 401
// і ПЕРЕРИВАЄ ланцюжок (AbortWithStatusJSON), тож хендлер не виконується.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, bearerPrefix) {
			abortUnauthenticated(c, "missing or malformed token")
			return
		}

		claims, err := auth.ParseToken(strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix)))
		if err != nil {
			// Причину (підробка, прострочення, збій конфігурації) клієнту не розкриваємо.
			abortUnauthenticated(c, "invalid or expired token")
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// abortUnauthenticated відповідає 401 і зупиняє ланцюжок.
func abortUnauthenticated(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized,
		model.NewErrorResponse(model.CodeUnauthenticated, message))
}
