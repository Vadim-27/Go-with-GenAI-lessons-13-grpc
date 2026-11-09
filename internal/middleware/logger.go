// Package middleware holds logging, panic recovery and CORS.
package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger writes one log line per request: method, path, status and duration.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		log.Printf("%s %q -> %d (%s)",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}
