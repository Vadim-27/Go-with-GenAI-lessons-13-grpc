package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS adds Access-Control-Allow-Origin to every response and answers
// preflight (OPTIONS) requests itself with status 204.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")

		if c.Request.Method != http.MethodOptions {
			c.Next()
			return
		}

		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// Allow the headers the browser asks for in the preflight request.
		if reqHeaders := c.GetHeader("Access-Control-Request-Headers"); reqHeaders != "" {
			h.Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			h.Set("Access-Control-Allow-Headers", "Content-Type")
		}
		h.Set("Access-Control-Max-Age", "86400")

		// The preflight ends here: it never reaches handlers or routes.
		c.AbortWithStatus(http.StatusNoContent)
	}
}
