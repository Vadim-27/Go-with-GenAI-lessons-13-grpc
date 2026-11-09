package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libratrack/internal/model"
)

// Health answers GET /health. It does not belong to any resource.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// NotFound answers every unregistered route with a JSON error.
func NotFound(c *gin.Context) {
	respondError(c, http.StatusNotFound, model.CodeNotFound, "Route not found")
}
