package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
)

// RequireRole пропускає лише користувачів, чия роль входить до allowed.
//
// Має стояти ПІСЛЯ AuthMiddleware: роль береться з контексту, який заповнює саме він.
//   - ролі в контексті немає (запит не автентифіковано) -> 401;
//   - роль є, але не дозволена                          -> 403.
//
// В обох випадках викликається AbortWithStatusJSON, щоб хендлер не виконався.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(CtxRole)
		if !exists {
			abortUnauthenticated(c, "authentication required")
			return
		}

		role, _ := value.(string)
		if !slices.Contains(allowed, role) {
			c.AbortWithStatusJSON(http.StatusForbidden,
				model.NewErrorResponse(model.CodeForbidden, "insufficient permissions"))
			return
		}
		c.Next()
	}
}
