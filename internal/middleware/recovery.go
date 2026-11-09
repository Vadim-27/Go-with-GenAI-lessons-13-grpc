package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
)

// Recovery catches a panic in a handler. The details (panic text and stack
// trace) go only to the log; the client gets a 500 internal_server_error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// This is the standard way for net/http to abort a response: do not swallow it.
			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			log.Printf("panic recovered: %v\n%s", rec, debug.Stack())

			if c.Writer.Written() {
				// The response has already started, the status cannot change.
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				model.NewErrorResponse(model.CodeInternalServerError, "Internal server error"))
		}()

		c.Next()
	}
}
